package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// consulVersionRegex parses the first line of `consul version`, which reads
// "Consul v2.0.4" and is followed by Revision/Build Date/Protocol lines.
var consulVersionRegex = regexp.MustCompile(`(?im)^Consul\s+v?(\S+)`)

// consulImageRegex matches the Consul server image (hashicorp/consul) but not
// sidecars such as hashicorp/consul-template or hashicorp/consul-k8s-control-plane.
var consulImageRegex = regexp.MustCompile(`(?i)(?:^|/)consul:([\w.-]+)$`)

// ConsulCheck detects the installed Consul version and records updates.
func ConsulCheck() {
	if _, err := exec.LookPath("consul"); err == nil {
		out, err := exec.Command("consul", "version").Output()
		if err != nil {
			log.Debug().Err(err).Msg("consul version failed; will try docker fallback")
		} else {
			text := strings.TrimSpace(string(out))
			if m := consulVersionRegex.FindStringSubmatch(text); len(m) == 2 {
				recordVersion("Consul", "consul", m[1])
				return
			}
			log.Debug().Str("output", text).Msg("Could not parse Consul version; will try docker fallback")
		}
	}

	if tag := dockerImageVersion(consulImageRegex); tag != "" {
		recordVersion("Consul", "consul", tag)
		return
	}

	log.Debug().Msg("Consul version not detected; skipping")
	addToNotInstalled("Consul")
}
