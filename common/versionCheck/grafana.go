package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// grafanaVersionRegex matches both output shapes Grafana has shipped:
//
//	grafana version 13.2.2                                       (`grafana --version`)
//	Version 13.2.2 (commit: 1bea008f7e, branch: release-13.2.2)   (`grafana-server -v`)
var grafanaVersionRegex = regexp.MustCompile(`(?i)version\s+v?(\d+\.\d+\.\d+)`)

// grafanaImageRegex matches the Grafana server images (grafana/grafana,
// grafana/grafana-oss, grafana/grafana-enterprise) but not the rest of the
// grafana/* namespace such as grafana/tempo or grafana/grafana-image-renderer.
var grafanaImageRegex = regexp.MustCompile(`(?i)(?:^|/)grafana(?:-oss|-enterprise)?:([\w.-]+)$`)

// grafanaVersionCommands are tried in order: the `grafana` binary replaced
// `grafana-server` in Grafana 11, but the old one is still around on hosts that
// have been upgraded in place.
var grafanaVersionCommands = [][]string{
	{"grafana", "--version"},
	{"grafana-server", "-v"},
}

// GrafanaCheck detects the installed Grafana version and records updates.
func GrafanaCheck() {
	for _, argv := range grafanaVersionCommands {
		if _, err := exec.LookPath(argv[0]); err != nil {
			continue
		}

		out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Str("command", argv[0]).Msg("Grafana version command failed")
			continue
		}

		text := strings.TrimSpace(string(out))
		if m := grafanaVersionRegex.FindStringSubmatch(text); len(m) == 2 {
			recordVersion("Grafana", "grafana", m[1])
			return
		}
		log.Debug().Str("command", argv[0]).Str("output", text).Msg("Could not parse Grafana version")
	}

	if tag := dockerImageVersion(grafanaImageRegex); tag != "" {
		recordVersion("Grafana", "grafana", tag)
		return
	}

	log.Debug().Msg("Grafana version not detected; skipping")
	addToNotInstalled("Grafana")
}
