package common

import (
	"fmt"
	"os/exec"
	"strings"
)

// FrankenPHPCheck detects the installed FrankenPHP version and reports updates
func FrankenPHPCheck() {
	// Check if FrankenPHP binary exists
	_, err := exec.LookPath("frankenphp")
	if err != nil {
		addToNotInstalled("FrankenPHP")
		return
	}

	// Get the version output
	out, err := exec.Command("frankenphp", "-v").Output()
	if err != nil {
		addToVersionErrors(fmt.Errorf("Error getting FrankenPHP version: %s", err.Error()))
		return
	}

	// Expected example output:
	// FrankenPHP v1.9.0 PHP 8.4.10 Caddy v2.10.0 h1:...
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		addToVersionErrors(fmt.Errorf("Unexpected FrankenPHP version output"))
		return
	}
	// fields[1] should be like "v1.9.0"
	version := strings.TrimPrefix(fields[1], "v")

	oldVersion := GatherVersion("frankenphp")

	if oldVersion != "" && oldVersion != version {
		addToUpdated(AppVersion{Name: "FrankenPHP", OldVersion: oldVersion, NewVersion: version})
		CreateNews("FrankenPHP", oldVersion, version, false)
	} else {
		addToNotUpdated(AppVersion{Name: "FrankenPHP", OldVersion: version})
	}

	StoreVersion("frankenphp", version)
}
