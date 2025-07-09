//go:build linux

package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

// ZimbraCheck detects the installed Zimbra/Zextras version, compares it with the previously stored version,
// potentially creates a Redmine news item, stores the current version, and returns the current version string.
func ZimbraCheck() (string, error) {
	var zimbraPath string
	var zimbraUser string

	if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
		zimbraPath = "/opt/zimbra"
		zimbraUser = "zimbra"
	}

	if _, err := os.Stat("/opt/zextras"); !os.IsNotExist(err) {
		zimbraPath = "/opt/zextras"
		zimbraUser = "zextras"
	}

	// Check if zimbraPath is empty
	if zimbraPath == "" {
		return "", nil // Zimbra/Zextras not found, ignore it
	}

	// Get the version of Zimbra/Zextras
	cmd := exec.Command("/bin/su", zimbraUser, "-c", zimbraPath+"/bin/zmcontrol -v")
	out, err := cmd.Output()
	if err != nil {
		errMsgLog := "Error getting Zimbra/Zextras version"
		errMsg := errMsgLog + ": " + err.Error()
		log.Error().Err(err).Msg(errMsgLog)
		return "", fmt.Errorf(errMsg)
	}

	// Parse the version
	// Example output: Release 8.8.15_GA_3869.UBUNTU18.64 UBUNTU18_64 FOSS edition.
	// Example output: Release 10.0.7_GA_0005.RHEL8_64 RHEL8_64 NETWORK edition.
	// Eg. output
	// Release 8.8.15_GA_3869.UBUNTU18.64 UBUNTU18_64 FOSS edition.
	versionParts := strings.Fields(string(out))
	if len(versionParts) < 2 {
		errMsgLog := "Unexpected output format from zmcontrol -v"
		errMsg := errMsgLog + ": " + string(out)
		log.Error().Msg(errMsgLog)
		return "", fmt.Errorf(errMsg)
	}
	version := strings.Split(versionParts[1], "_GA_")[0] // Extract version like "8.8.15" or "10.0.7"

	log.Debug().Str("version", version).Msg("Detected Zimbra/Zextras version")

	oldVersion := GatherVersion("zimbra") // Use "zimbra" key for both Zimbra and Zextras

	if oldVersion != "" && oldVersion == version {
		log.Debug().Msg("Zimbra/Zextras version unchanged.")
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Msg("Zimbra/Zextras has been updated.")
		log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("Zimbra/Zextras has been updated")
		CreateNews("Zimbra/Zextras", oldVersion, version, false) // Update news title
	} else {
		log.Debug().Str("version", version).Msg("Storing initial Zimbra/Zextras version")
	}

	StoreVersion("zimbra", version) // Store the detected version

	return version, nil // Return the detected version string
}
