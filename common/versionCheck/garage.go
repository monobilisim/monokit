package common

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

// GarageCheck detects the installed Garage (S3-compatible object store) version,
// compares with the stored version, creates a news item on change, and persists
// the current version. Follows the same pattern as CaddyCheck/VersityGWCheck.
func GarageCheck() (string, error) {
	// Check if Garage binary is installed
	_, err := exec.LookPath("garage")
	if err != nil {
		log.Debug().Msg("Garage binary not found, skipping version check")
		addToNotInstalled("Garage")
		return "", nil // Not an error, just not installed
	}

	// Get Garage version
	// Typical output:
	//   garage v2.2.0 [features: bundled-libs, consul-discovery, fjall, journald, ...]
	out, err := exec.Command("garage", "--version").Output()
	if err != nil {
		errMsg := "Error getting Garage version: " + err.Error()
		log.Error().Msg(errMsg)
		addToVersionErrors(fmt.Errorf("%s", errMsg))
		return "", fmt.Errorf("%s", errMsg)
	}

	versionOutput := strings.TrimSpace(string(out))
	fields := strings.Fields(versionOutput)
	if len(fields) == 0 {
		errMsg := "Unexpected output format from garage --version: " + versionOutput
		log.Error().Msg(errMsg)
		addToVersionErrors(fmt.Errorf("%s", errMsg))
		return "", fmt.Errorf("%s", errMsg)
	}

	// Skip the leading "garage" token if present; fall back to first token otherwise.
	first := fields[0]
	if strings.EqualFold(first, "garage") {
		if len(fields) < 2 {
			errMsg := "Unexpected output format from garage --version: " + versionOutput
			log.Error().Msg(errMsg)
			addToVersionErrors(fmt.Errorf("%s", errMsg))
			return "", fmt.Errorf("%s", errMsg)
		}
		first = fields[1]
	}
	version := strings.TrimPrefix(first, "v")

	if version == "" {
		errMsg := "Unable to parse Garage version: " + versionOutput
		log.Error().Msg(errMsg)
		addToVersionErrors(fmt.Errorf("%s", errMsg))
		return "", fmt.Errorf("%s", errMsg)
	}

	log.Debug().Str("version", version).Msg("Detected Garage version")

	oldVersion := GatherVersion("garage")

	if oldVersion != "" && oldVersion == version {
		log.Debug().Msg("Garage version unchanged.")
		addToNotUpdated(AppVersion{Name: "Garage", OldVersion: oldVersion})
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("Garage has been updated")
		addToUpdated(AppVersion{Name: "Garage", OldVersion: oldVersion, NewVersion: version})
		CreateNews("Garage", oldVersion, version, false)
	} else {
		log.Debug().Str("version", version).Msg("Storing initial Garage version")
		addToNotUpdated(AppVersion{Name: "Garage", OldVersion: version})
	}

	StoreVersion("garage", version)
	return version, nil
}
