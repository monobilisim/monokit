package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// mimirImageRegex matches the Mimir image but not grafana/mimir-continuous-test
// or grafana/mimirtool.
var mimirImageRegex = regexp.MustCompile(`(?i)(?:^|/)mimir:([\w.-]+)$`)

// MimirCheck detects the installed Grafana Mimir version and records updates.
// `mimir --version` prints "Mimir, version 3.2.1 (branch: HEAD, revision: ...)".
func MimirCheck() {
	if _, err := exec.LookPath("mimir"); err == nil {
		out, err := exec.Command("mimir", "--version").CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Msg("mimir --version failed; will try docker fallback")
		} else {
			text := strings.TrimSpace(string(out))
			if m := promCommonVersionRegex.FindStringSubmatch(text); len(m) == 2 {
				recordVersion("Mimir", "mimir", m[1])
				return
			}
			log.Debug().Str("output", text).Msg("Could not parse Mimir version; will try docker fallback")
		}
	}

	if tag := dockerImageVersion(mimirImageRegex); tag != "" {
		recordVersion("Mimir", "mimir", tag)
		return
	}

	log.Debug().Msg("Mimir version not detected; skipping")
	addToNotInstalled("Mimir")
}
