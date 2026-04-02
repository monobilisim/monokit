//go:build linux

package common

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rs/zerolog/log"
)

func SystemdUnitActive(unitName string) bool {
	ctx := context.Background()

	// Check if the unit is active
	systemdConnection, err := dbus.NewSystemConnectionContext(ctx)

	if err != nil {
		log.Error().Err(err).Msg("Error connecting to systemd")
	}

	defer systemdConnection.Close()

	listOfUnits, err := systemdConnection.ListUnitsContext(ctx)

	if err != nil {
		log.Error().Err(err).Msg("Error listing systemd units")
	}

	for _, unit := range listOfUnits {
		if unit.Name == unitName {
			return unit.ActiveState == "active"
		}
	}

	return false
}

// SystemdUnitExists checks if a systemd unit file exists in common locations.
// The unit name may contain glob wildcards (e.g. "postgresql@*.service").
func SystemdUnitExists(unit string) bool {
	// Common paths for systemd unit files
	paths := []string{
		"/etc/systemd/system/",
		"/run/systemd/system/",
		"/usr/lib/systemd/system/",
		"/lib/systemd/system/",
	}

	isGlob := strings.ContainsAny(unit, "*?[")

	for _, p := range paths {
		if isGlob {
			matches, err := filepath.Glob(p + unit)
			if err != nil {
				log.Error().Str("pattern", p+unit).Err(err).Msg("Error matching systemd unit glob pattern")
				continue
			}
			if len(matches) > 0 {
				log.Debug().Str("pattern", p+unit).Str("match", matches[0]).Msg("Found systemd unit file via glob")
				return true
			}
		} else {
			filePath := p + unit
			if _, err := os.Stat(filePath); err == nil {
				log.Debug().Str("filePath", filePath).Msg("Found systemd unit file")
				return true
			} else if !os.IsNotExist(err) {
				log.Error().Str("filePath", filePath).Err(err).Msg("Error checking for systemd unit file")
			}
		}
	}

	log.Debug().Str("unit", unit).Msg("Systemd unit file not found in common locations")
	return false
}

// ServiceTail returns the last N lines from journald for the given systemd unit.
// Uses `journalctl` to avoid cgo dependency. Linux-only file.
func ServiceTail(unit string, maxLines int) string {
	if unit == "" {
		return ""
	}
	if maxLines <= 0 {
		maxLines = 100
	}

	// Ensure journalctl exists
	if _, err := exec.LookPath("journalctl"); err != nil {
		log.Debug().Err(err).Str("unit", unit).Msg("journalctl not found; skipping ServiceTail")
		return ""
	}

	cmd := exec.Command("journalctl", "-u", unit, "-n", strconv.Itoa(maxLines), "--no-pager", "-o", "short-iso")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Debug().Err(err).Str("unit", unit).Msg("Failed to collect journal tail for unit")
		return ""
	}
	return string(output)
}
