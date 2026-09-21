package common

import (
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// wazuhControlPath is the manager/agent control script installed by every
// Wazuh 4.x package.
const wazuhControlPath = "/var/ossec/bin/wazuh-control"

// wazuhVersionRegex parses `wazuh-control info`, which prints shell-style
// assignments: WAZUH_VERSION="v4.9.0", WAZUH_REVISION, WAZUH_TYPE.
var wazuhVersionRegex = regexp.MustCompile(`WAZUH_VERSION="?v?([^"\s]+)"?`)

// wazuhImageRegex matches the Wazuh manager image. The indexer and dashboard
// ship their own images and their own version numbers, so the manager is the
// one that stands for "the Wazuh version" here.
var wazuhImageRegex = regexp.MustCompile(`(?i)(?:^|/)wazuh-manager:([\w.-]+)$`)

// WazuhCheck detects the installed Wazuh version and records updates.
func WazuhCheck() {
	if _, err := os.Stat(wazuhControlPath); err == nil {
		out, err := exec.Command(wazuhControlPath, "info").CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Msg("wazuh-control info failed; will try docker fallback")
		} else {
			text := strings.TrimSpace(string(out))
			if m := wazuhVersionRegex.FindStringSubmatch(text); len(m) == 2 {
				recordVersion("Wazuh", "wazuh", m[1])
				return
			}
			log.Debug().Str("output", text).Msg("Could not parse Wazuh version; will try docker fallback")
		}
	}

	if tag := dockerImageVersion(wazuhImageRegex); tag != "" {
		recordVersion("Wazuh", "wazuh", tag)
		return
	}

	log.Debug().Msg("Wazuh version not detected; skipping")
	addToNotInstalled("Wazuh")
}
