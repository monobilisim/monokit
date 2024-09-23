//go:build linux
package redisHealth

import (
    "github.com/coreos/go-systemd/v22/dbus"
    "github.com/monobilisim/monokit/common"
)

func SystemdUnitActive(unitName string) bool {
    // Check if the unit is active
    systemdConnection, err := dbus.NewSystemdConnection()
    
    if err != nil {
        common.LogError("Error connecting to systemd: " + err.Error())
    }

    listOfUnits, err := systemdConnection.ListUnits()

    if err != nil {
        common.LogError("Error listing systemd units: " + err.Error())
    }

    for _, unit := range listOfUnits {
        if unit.Name == unitName {
            return unit.ActiveState == "active"
        }
    }

    systemdConnection.Close()
    return false
}
