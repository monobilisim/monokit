package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// tempoImageRegex matches the Tempo image but not grafana/tempo-query.
var tempoImageRegex = regexp.MustCompile(`(?i)(?:^|/)tempo:([\w.-]+)$`)

// TempoCheck detects the installed Grafana Tempo version and records updates.
// `tempo --version` prints "tempo, version v3.0.0 (branch: main, revision: ...)".
func TempoCheck() {
	if _, err := exec.LookPath("tempo"); err == nil {
		out, err := exec.Command("tempo", "--version").CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Msg("tempo --version failed; will try docker fallback")
		} else {
			text := strings.TrimSpace(string(out))
			if m := promCommonVersionRegex.FindStringSubmatch(text); len(m) == 2 {
				recordVersion("Tempo", "tempo", m[1])
				return
			}
			log.Debug().Str("output", text).Msg("Could not parse Tempo version; will try docker fallback")
		}
	}

	if tag := dockerImageVersion(tempoImageRegex); tag != "" {
		recordVersion("Tempo", "tempo", tag)
		return
	}

	log.Debug().Msg("Tempo version not detected; skipping")
	addToNotInstalled("Tempo")
}
