//go:build linux

package common

import (
	"context"
	"fmt"
	"os"

	"github.com/coreos/go-systemd/v22/dbus"
)

func SystemdUnitActive(unitName string) bool {
	ctx := context.Background()

	// Check if the unit is active
	systemdConnection, err := dbus.NewSystemConnectionContext(ctx)

	if err != nil {
		LogError("Error connecting to systemd: " + err.Error())
	}

	defer systemdConnection.Close()

	listOfUnits, err := systemdConnection.ListUnitsContext(ctx)

	if err != nil {
		LogError("Error listing systemd units: " + err.Error())
	}

	for _, unit := range listOfUnits {
		if unit.Name == unitName {
			return unit.ActiveState == "active"
		}
	}

	return false
}

// SystemdUnitExists checks if a systemd unit file exists in common locations.
func SystemdUnitExists(unit string) bool {
	// Common paths for systemd unit files
	paths := []string{
		"/etc/systemd/system/",
		"/run/systemd/system/",
		"/usr/lib/systemd/system/",
		"/lib/systemd/system/",
	}

	for _, p := range paths {
		filePath := p + unit
		if _, err := os.Stat(filePath); err == nil {
			LogDebug(fmt.Sprintf("Found systemd unit file at: %s", filePath))
			return true // File exists
		} else if !os.IsNotExist(err) {
			// Log error if it's something other than "not found"
			LogError(fmt.Sprintf("Error checking for systemd unit file %s: %v", filePath, err))
		}
	}

	LogDebug(fmt.Sprintf("Systemd unit file %s not found in common locations.", unit))
	return false // File does not exist in any common location
}
