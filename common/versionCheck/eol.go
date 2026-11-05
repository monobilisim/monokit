package common

import (
	"encoding/json"
	"errors"
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

// eolAPIBase is the endoflife.date API root; tests point it at a local server.
var eolAPIBase = "https://endoflife.date/api"

// eolLockTimeout bounds how long a run waits for another monokit process that
// is checking the same product.
const eolLockTimeout = 30 * time.Second

func CheckLatestVersions(apps []AppVersion) {
	client := &http.Client{Timeout: 10 * time.Second}

	for _, app := range apps {
		checkLatestVersion(client, app)
	}
}

// checkLatestVersion alarms once for each newer release in the cycle of the
// installed version. Reading the state, alarming and saving the state run
// under a lock shared by every monokit process: overlapping runs (e.g. cron
// runs piled up behind a stalled host) otherwise all read the same state and
// all send the alarm.
func checkLatestVersion(client *http.Client, app AppVersion) {
	productSlug, ok := eolProductMap[app.Name]
	if !ok {
		return
	}

	currentVersion := app.NewVersion
	if currentVersion == "" {
		currentVersion = app.OldVersion
	}
	if currentVersion == "" {
		return
	}

	key := fmt.Sprintf("%s_eol", productSlug)

	unlock, err := healthdb.Lock("eol_alarm", key, eolLockTimeout)
	if err != nil {
		if errors.Is(err, healthdb.ErrLockTimeout) {
			log.Warn().Str("app", app.Name).Dur("timeout", eolLockTimeout).Msg("Another monokit process is still checking this app's latest version; skipping this run")
			return
		}
		// The lock itself is broken (e.g. lock dir not writable). Carry on
		// unlocked rather than stop checking altogether.
		log.Error().Err(err).Str("app", app.Name).Msg("Failed to take EOL alarm lock; continuing without it")
		unlock = func() {}
	}
	defer unlock()

	jsonStr, _, _, found, err := healthdb.GetJSON("eol_alarm", key)
	if err != nil {
		// An unreadable state is not an empty one: treating it as "never
		// notified" re-sends an alarm that already went out.
		log.Error().Err(err).Str("app", app.Name).Msg("Failed to read EOL alarm state; skipping this run")
		return
	}

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
			return
		}
		if time.Since(state.LastCheckTime) < 24*time.Hour {
			return
		}
	}

	url := fmt.Sprintf("%s/%s.json", eolAPIBase, productSlug)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Debug().Err(err).Str("app", app.Name).Msg("Failed to create request for endoflife.date")
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Monokit/1.0")

	resp, err := client.Do(req)
	if err != nil {
		log.Debug().Err(err).Str("app", app.Name).Msg("Failed to fetch from endoflife.date")
		return
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		log.Debug().Int("status", resp.StatusCode).Str("app", app.Name).Msg("Bad status from endoflife.date")
		return
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to read body from endoflife.date")
		return
	}

	var cycles []EOLCycle
	if err := json.Unmarshal(body, &cycles); err != nil {
		log.Debug().Err(err).Msg("Failed to parse JSON from endoflife.date")
		return
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
	if err := healthdb.PutJSON("eol_alarm", key, string(b), nil, time.Now()); err != nil {
		log.Error().Err(err).Str("app", app.Name).Msg("Failed to save EOL alarm state")
	}
}
