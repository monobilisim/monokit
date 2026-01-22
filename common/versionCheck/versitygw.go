package common

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

// VersityGWCheck detects the installed Versity S3 Gateway version,
// compares with stored value, creates news on change, and persists the current version.
func VersityGWCheck() {
	// Check if versitygw binary exists
	if _, err := exec.LookPath("versitygw"); err != nil {
		log.Debug().Msg("versitygw not found, skipping VersityGW version check")
		addToNotInstalled("VersityGW")
		return
	}

	// versitygw --version output format:
	// Version  : 1.0.20
	// Build    : b15e03d1540584ebb80daee4b1421c253b6051fa
	// BuildTime: 2025-12-18T20:34:09Z
	out, err := exec.Command("versitygw", "--version").Output()
	if err != nil {
		log.Error().Err(err).Msg("Error getting VersityGW version")
		addToVersionErrors(fmt.Errorf("Error getting VersityGW version: %s", err.Error()))
		return
	}

	// Parse the version from "Version  : 1.0.20" line
	text := strings.TrimSpace(string(out))
	var version string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Version") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				version = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	if version == "" {
		log.Error().Str("output", text).Msg("Unable to parse VersityGW version")
		addToVersionErrors(fmt.Errorf("Unable to parse VersityGW version"))
		return
	}

	oldVersion := GatherVersion("versitygw")
	if oldVersion != "" && oldVersion == version {
		log.Debug().Msg("VersityGW version unchanged")
		addToNotUpdated(AppVersion{Name: "VersityGW", OldVersion: oldVersion, NewVersion: version})
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("VersityGW has been updated")
		addToUpdated(AppVersion{Name: "VersityGW", OldVersion: oldVersion, NewVersion: version})
		CreateNews("VersityGW", oldVersion, version, false)
	} else {
		log.Debug().Str("version", version).Msg("Storing initial VersityGW version")
		addToNotUpdated(AppVersion{Name: "VersityGW", OldVersion: version})
	}

	StoreVersion("versitygw", version)
}
