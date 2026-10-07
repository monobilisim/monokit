package common

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	monocommon "github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
	"github.com/rs/zerolog/log"
)

var eolProductMap = map[string]string{
	"Proxmox VE":            "proxmox-ve",
	"PostgreSQL":            "postgresql",
	"MySQL":                 "mysql",
	"MariaDB":               "mariadb",
	"MongoDB":               "mongodb",
	"Redis":                 "redis",
	"Caddy":                 "caddy",
	"Nginx":                 "nginx",
	"HAProxy":               "haproxy",
	"Docker Engine":         "docker-engine",
	"Docker Client":         "docker-engine",
	"Proxmox Mail Gateway":  "proxmox-mail-gateway",
	"Proxmox Backup Server": "proxmox-backup-server",
	"RabbitMQ":              "rabbitmq",
	"Zabbix":                "zabbix",
	"Jenkins":               "jenkins",
	"Prometheus":            "prometheus",
	"Vault":                 "hashicorp-vault",
	"OPNsense":              "opnsense",
	"Garage":                "garage",
	"Consul":                "consul",
	"Grafana":               "grafana",
	"OpenSearch":            "opensearch",
	"PHP":                   "php",
}

type EOLCycle struct {
	Cycle             string      `json:"cycle"`
	Latest            string      `json:"latest"`
	LatestReleaseDate string      `json:"latestReleaseDate"`
	ReleaseDate       string      `json:"releaseDate"`
	LTS               interface{} `json:"lts"`
}

var versionTokenRegex = regexp.MustCompile(`[0-9]+|[A-Za-z]+`)

// compareVersions compares two version strings token by token. Separators
// (., -, _, +) are interchangeable, so a FreeBSD style package revision such
// as OPNsense's 26.7.4_1 sorts above the plain 26.7.4 it patches. Alphabetic
// tokens (rc, beta, ...) mark pre-releases and sort below the release they
// lead up to.
func compareVersions(v1, v2 string) int {
	t1 := versionTokenRegex.FindAllString(v1, -1)
	t2 := versionTokenRegex.FindAllString(v2, -1)

	maxLen := len(t1)
	if len(t2) > maxLen {
		maxLen = len(t2)
	}

	for i := 0; i < maxLen; i++ {
		var s1, s2 string
		if i < len(t1) {
			s1 = t1[i]
		}
		if i < len(t2) {
			s2 = t2[i]
		}

		r1, r2 := versionTokenRank(s1), versionTokenRank(s2)
		if r1 != r2 {
			if r1 > r2 {
				return 1
			}
			return -1
		}

		var cmp int
		switch r1 {
		case versionTokenNumeric:
			cmp = compareNumericToken(s1, s2)
		case versionTokenAlpha:
			cmp = strings.Compare(strings.ToLower(s1), strings.ToLower(s2))
		}
		if cmp != 0 {
			return cmp
		}
	}
	return 0
}

const (
	versionTokenAlpha = iota
	versionTokenAbsent
	versionTokenNumeric
)

// versionTokenRank orders the token kinds: a pre-release marker sorts below a
// missing token, which in turn sorts below an extra numeric token.
func versionTokenRank(s string) int {
	switch {
	case s == "":
		return versionTokenAbsent
	case s[0] >= '0' && s[0] <= '9':
		return versionTokenNumeric
	default:
		return versionTokenAlpha
	}
}

// compareNumericToken compares two digit-only tokens without the overflow
// risk of parsing them into ints.
func compareNumericToken(s1, s2 string) int {
	s1 = strings.TrimLeft(s1, "0")
	s2 = strings.TrimLeft(s2, "0")
	if len(s1) != len(s2) {
		if len(s1) > len(s2) {
			return 1
		}
		return -1
	}
	return strings.Compare(s1, s2)
}

func CheckLatestVersions(apps []AppVersion) {
	client := &http.Client{Timeout: 10 * time.Second}

	for _, app := range apps {
		productSlug, ok := eolProductMap[app.Name]
		if !ok {
			continue
		}

		currentVersion := app.NewVersion
		if currentVersion == "" {
			currentVersion = app.OldVersion
		}
		if currentVersion == "" {
			continue
		}

		key := fmt.Sprintf("%s_eol", productSlug)
		jsonStr, _, _, found, _ := healthdb.GetJSON("eol_alarm", key)

		var state struct {
			NotifiedVersion string    `json:"notified_version"`
			AppVersion      string    `json:"app_version"`
			LastCheckTime   time.Time `json:"last_check_time"`
		}
		if found && jsonStr != "" {
			json.Unmarshal([]byte(jsonStr), &state)
		}

		if state.AppVersion == currentVersion {
			if state.NotifiedVersion != "" && compareVersions(state.NotifiedVersion, currentVersion) > 0 {
				continue
			}
			if time.Since(state.LastCheckTime) < 24*time.Hour {
				continue
			}
		}

		url := fmt.Sprintf("https://endoflife.date/api/%s.json", productSlug)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Debug().Err(err).Str("app", app.Name).Msg("Failed to create request for endoflife.date")
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Monokit/1.0")

		resp, err := client.Do(req)
		if err != nil {
			log.Debug().Err(err).Str("app", app.Name).Msg("Failed to fetch from endoflife.date")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Debug().Int("status", resp.StatusCode).Str("app", app.Name).Msg("Bad status from endoflife.date")
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Debug().Err(err).Msg("Failed to read body from endoflife.date")
			continue
		}

		var cycles []EOLCycle
		if err := json.Unmarshal(body, &cycles); err != nil {
			log.Debug().Err(err).Msg("Failed to parse JSON from endoflife.date")
			continue
		}

		var matchedCycle *EOLCycle
		for i, c := range cycles {
			if currentVersion == c.Cycle || strings.HasPrefix(currentVersion, c.Cycle+".") || strings.HasPrefix(currentVersion, c.Cycle+"-") || strings.HasPrefix(currentVersion, c.Cycle+"_") {
				matchedCycle = &cycles[i]
				break
			}
		}

		if matchedCycle == nil {
			log.Debug().Str("app", app.Name).Str("version", currentVersion).Msg("Could not find matching cycle on endoflife.date")
		} else {
			if compareVersions(matchedCycle.Latest, currentVersion) > 0 {
				latest := matchedCycle.Latest

				if state.NotifiedVersion != latest {
					msg := fmt.Sprintf("[versionCheck - %s] [:warning:] Update required: %s version %s is out (you have %s)", monocommon.Config.Identifier, app.Name, latest, currentVersion)
					log.Info().Str("app", app.Name).Str("latest", latest).Str("current", currentVersion).Msg("New stable LTS release found, sending alarm")

					monocommon.Alarm(msg, "", "", false)

					state.NotifiedVersion = latest
				}
			}
		}

		state.AppVersion = currentVersion
		state.LastCheckTime = time.Now()
		b, _ := json.Marshal(state)
		healthdb.PutJSON("eol_alarm", key, string(b), nil, time.Now())
	}
}
