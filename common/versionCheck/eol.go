package common

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
}

type EOLCycle struct {
	Cycle             string      `json:"cycle"`
	Latest            string      `json:"latest"`
	LatestReleaseDate string      `json:"latestReleaseDate"`
	ReleaseDate       string      `json:"releaseDate"`
	LTS               interface{} `json:"lts"`
}

func compareVersions(v1, v2 string) int {
	v1 = strings.ReplaceAll(v1, "-", ".")
	v2 = strings.ReplaceAll(v2, "-", ".")
	p1 := strings.Split(v1, ".")
	p2 := strings.Split(v2, ".")

	maxLen := len(p1)
	if len(p2) > maxLen {
		maxLen = len(p2)
	}

	for i := 0; i < maxLen; i++ {
		n1, n2 := 0, 0
		if i < len(p1) {
			n1, _ = strconv.Atoi(p1[i])
		}
		if i < len(p2) {
			n2, _ = strconv.Atoi(p2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
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
			if currentVersion == c.Cycle || strings.HasPrefix(currentVersion, c.Cycle+".") || strings.HasPrefix(currentVersion, c.Cycle+"-") {
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
