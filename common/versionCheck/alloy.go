package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// alloyImageRegex matches the Alloy image but not neighbours such as
// grafana/alloy-operator or grafana/alloy-dev.
var alloyImageRegex = regexp.MustCompile(`(?i)(?:^|/)alloy:([\w.-]+)$`)

// AlloyCheck detects the installed Grafana Alloy version and records updates.
// `alloy --version` prints "alloy, version v1.19.2 (branch: HEAD, revision: ...)",
// the same prometheus/common banner Tempo and Mimir use.
func AlloyCheck() {
	if _, err := exec.LookPath("alloy"); err == nil {
		out, err := exec.Command("alloy", "--version").CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Msg("alloy --version failed; will try docker fallback")
		} else {
			text := strings.TrimSpace(string(out))
			if m := promCommonVersionRegex.FindStringSubmatch(text); len(m) == 2 {
				recordVersion("Alloy", "alloy", m[1])
				return
			}
			log.Debug().Str("output", text).Msg("Could not parse Alloy version; will try docker fallback")
		}
	}

	if tag := dockerImageVersion(alloyImageRegex); tag != "" {
		recordVersion("Alloy", "alloy", tag)
		return
	}

	log.Debug().Msg("Alloy version not detected; skipping")
	addToNotInstalled("Alloy")
}
