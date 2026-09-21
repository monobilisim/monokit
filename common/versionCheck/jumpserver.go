package common

import (
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// jumpserverVersionRegex pulls CURRENT_VERSION out of JumpServer's config.txt.
// The installer writes plain unquoted "KEY=value" lines (scripts/gists/conf.sh),
// and values look like "v3.10.12" or, from v4 on, "v4.10.0-ce".
var jumpserverVersionRegex = regexp.MustCompile(`(?m)^[ \t]*CURRENT_VERSION=(.*)$`)

// jumpserverImageRegex matches the JumpServer core image. Every JumpServer
// component (core, koko, lion, chen, magnus...) is tagged with the same
// ${VERSION}, and core is the one deployment that is always present. The
// optional host prefix covers IMAGE_PULL_PREFIX pointing at a private registry.
var jumpserverImageRegex = regexp.MustCompile(`(?i)(?:^|/)jumpserver/core:([\w.-]+)$`)

// jumpserverConfigPath returns the installer's config file. const.sh resolves it
// as ${JS_CONFIG_DIR:-/opt/jumpserver/config}/config.txt, so honour the same
// override rather than hardcoding the default location.
func jumpserverConfigPath() string {
	configDir := os.Getenv("JS_CONFIG_DIR")
	if configDir == "" {
		configDir = "/opt/jumpserver/config"
	}

	return configDir + "/config.txt"
}

// JumpServerCheck detects the installed JumpServer version and records updates.
// Reads the version the installer recorded in config.txt, falling back to the
// running core container's image tag.
func JumpServerCheck() {
	// Case 1: the installer writes CURRENT_VERSION on every install and upgrade.
	// This is what `jmsctl version` reports; reading the file directly avoids
	// executing the installer's shell scripts from a monitoring run.
	path := jumpserverConfigPath()
	if content, err := os.ReadFile(path); err == nil {
		// Last match wins, matching get_config()'s own loop: it keeps assigning
		// as it reads, so a duplicated key resolves to the final occurrence.
		matches := jumpserverVersionRegex.FindAllStringSubmatch(string(content), -1)
		if len(matches) > 0 {
			version := strings.TrimSpace(matches[len(matches)-1][1])
			// Trim the "v" so this agrees with the container tag, which
			// dockerImageVersion already normalises the same way.
			if version = strings.TrimPrefix(version, "v"); version != "" {
				recordVersion("JumpServer", "jumpserver", version)
				return
			}
		}
		log.Debug().Str("path", path).Msg("No CURRENT_VERSION in JumpServer config; will try docker fallback")
	}

	// Case 2: infer the version from the running core container.
	if tag := dockerImageVersion(jumpserverImageRegex); tag != "" {
		recordVersion("JumpServer", "jumpserver", tag)
		return
	}

	log.Debug().Msg("JumpServer version not detected; skipping")
	addToNotInstalled("JumpServer")
}
