//go:build linux

package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/monobilisim/monokit/common"
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

	// Get the version of Zimbra/Zextras
	cmd := exec.Command("/bin/su", zimbraUser, "-c", zimbraPath+"/bin/zmcontrol -v")
	out, err := cmd.Output()
	if err != nil {
		errMsg := "Error getting Zimbra/Zextras version: " + err.Error()
		common.LogError(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// Parse the version
	// Example output: Release 8.8.15_GA_3869.UBUNTU18.64 UBUNTU18_64 FOSS edition.
	// Example output: Release 10.0.7_GA_0005.RHEL8_64 RHEL8_64 NETWORK edition.
	// Eg. output
	// Release 8.8.15_GA_3869.UBUNTU18.64 UBUNTU18_64 FOSS edition.
	versionParts := strings.Fields(string(out))
	if len(versionParts) < 2 {
		errMsg := "Unexpected output format from zmcontrol -v: " + string(out)
		common.LogError(errMsg)
		return "", fmt.Errorf(errMsg)
	}
	version := strings.Split(versionParts[1], "_GA_")[0] // Extract version like "8.8.15" or "10.0.7"

	common.LogDebug("Detected Zimbra/Zextras version: " + version)

	oldVersion := GatherVersion("zimbra") // Use "zimbra" key for both Zimbra and Zextras

	if oldVersion != "" && oldVersion == version {
		common.LogDebug("Zimbra/Zextras version unchanged.")
	} else if oldVersion != "" && oldVersion != version {
		common.LogInfo("Zimbra/Zextras has been updated.")
		common.LogInfo("Old version: " + oldVersion)
		common.LogInfo("New version: " + version)
		CreateNews("Zimbra/Zextras", oldVersion, version, false) // Update news title
	} else {
		common.LogInfo("Storing initial Zimbra/Zextras version: " + version)
	}

	StoreVersion("zimbra", version) // Store the detected version

	return version, nil // Return the detected version string
}
