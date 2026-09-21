package common

import (
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// zulipVersionPyRegex extracts ZULIP_VERSION = "9.4" from a deployment's version.py
var zulipVersionPyRegex = regexp.MustCompile(`(?m)^ZULIP_VERSION\s*=\s*["']([^"']+)["']`)

// zulipImageRegex matches the Zulip server image only (zulip/docker-zulip:9.4-0 or
// zulip/zulip:9.4-0), not the sidecar images such as zulip/zulip-postgresql:14.
var zulipImageRegex = regexp.MustCompile(`(?i)(?:^|/)(?:docker-)?zulip:([\w.-]+)$`)

// zulipDockerTagSuffixRegex strips the docker image rebuild counter from tags like
// "9.4-0" so the reported version matches what version.py would report.
var zulipDockerTagSuffixRegex = regexp.MustCompile(`-\d+$`)

// zulipVersionPaths are the locations of version.py for a standard Zulip install.
var zulipVersionPaths = []string{
	"/home/zulip/deployments/current/version.py",
	"/srv/zulip/version.py",
}

// ZulipCheck detects the installed Zulip version and records updates.
// Reads version.py from the active deployment first; falls back to the running
// container image tag for docker-zulip deployments.
func ZulipCheck() {
	// Case 1: standard install, read the active deployment's version.py
	for _, path := range zulipVersionPaths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		m := zulipVersionPyRegex.FindStringSubmatch(string(content))
		if len(m) != 2 {
			log.Debug().Str("path", path).Msg("Could not parse ZULIP_VERSION from version.py")
			continue
		}

		if version := strings.TrimSpace(m[1]); version != "" {
			recordVersion("Zulip", "zulip", version)
			return
		}
	}

	// Case 2: docker-zulip, infer the version from the running image tag
	if tag := dockerImageVersion(zulipImageRegex); tag != "" {
		recordVersion("Zulip", "zulip", zulipDockerTagSuffixRegex.ReplaceAllString(tag, ""))
		return
	}

	log.Debug().Msg("Zulip version not detected; skipping")
	addToNotInstalled("Zulip")
}
