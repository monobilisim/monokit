//go:build linux
package common

import (
    "context"
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
