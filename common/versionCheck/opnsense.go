package common

import (
	"fmt"
	"os/exec"
	"strings"
)

func OPNsenseCheck() {
	// Check if OPNsense is installed by checking the existence of command "opnsense-version"
	_, err := exec.LookPath("opnsense-version")
	if err != nil {
		addToNotInstalled("OPNsense")
		return
	}

	// Get the version of OPNsense
	out, err := exec.Command("opnsense-version").Output()
	if err != nil {
		addToVersionErrors(fmt.Errorf("Error getting OPNsense version"))
		return
	}

	// Parse the version
	// Eg. output
	// OPNsense 21.1.8_1 (amd64)
	version := strings.Split(string(out), " ")[1]

	oldVersion := GatherVersion("opnsense")

	if oldVersion != "" && oldVersion != version {
		addToUpdated(AppVersion{Name: "OPNsense", OldVersion: oldVersion, NewVersion: version})
		CreateNews("OPNsense", oldVersion, version, false)
	} else {
		addToNotUpdated(AppVersion{Name: "OPNsense", OldVersion: version})
	}

	StoreVersion("opnsense", version)
}
