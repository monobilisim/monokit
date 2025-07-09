package common

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

// VaultCheck detects the installed Vault version, compares it with the previously stored version,
// potentially creates a Redmine news item, stores the current version, and returns the current version string.
func VaultCheck() (string, error) {
	// Check if Vault binary is installed
	_, err := exec.LookPath("vault")
	if err != nil {
		log.Debug().Msg("Vault binary not found, skipping version check")
		return "", nil // Not an error, just not installed
	}

	// Get the version of Vault
	cmd := exec.Command("vault", "version")
	out, err := cmd.Output()
	if err != nil {
		errMsg := "Error getting Vault version: " + err.Error()
		log.Error().Msg(errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}

	// Parse the version
	// Example output: "Vault v1.2.3"
	// Example output: "Vault v1.2.3, built 2022-05-03T08:34:11Z"
	versionOutput := strings.TrimSpace(string(out))
	versionParts := strings.Split(versionOutput, "v")
	if len(versionParts) < 2 {
		errMsg := "Unexpected output format from vault version: " + versionOutput
		log.Error().Msg(errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}

	version := strings.TrimSpace(versionParts[1])
	// Remove any additional build info (e.g., "1.2.3, built 2022-05-03T08:34:11Z")
	if spaceIndex := strings.Index(version, ","); spaceIndex != -1 {
		version = strings.TrimSpace(version[:spaceIndex])
	}

	log.Debug().Str("version", version).Msg("Detected Vault version")

	oldVersion := GatherVersion("vault")

	if oldVersion != "" && oldVersion == version {
		log.Debug().Msg("Vault version unchanged.")
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Msg("Vault has been updated.")
		log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("Vault has been updated")
		CreateNews("Vault", oldVersion, version, false)
	} else {
		log.Debug().Msg("Storing initial Vault version: " + version)
	}

	StoreVersion("vault", version)

	return version, nil
}
