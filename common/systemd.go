//go:build linux

package common

import (
    "context"
    "fmt"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/coreos/go-systemd/v22/dbus"
    "github.com/coreos/go-systemd/v22/sdjournal"
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
			log.Debug().Str("filePath", filePath).Msg("Found systemd unit file")
			return true // File exists
		} else if !os.IsNotExist(err) {
			// Log error if it's something other than "not found"
			log.Error().Str("filePath", filePath).Err(err).Msg("Error checking for systemd unit file")
		}
	}

	log.Debug().Str("unit", unit).Msg("Systemd unit file not found in common locations")
	return false // File does not exist in any common location
}

// ServiceTail returns the last N lines from journald for the given systemd unit.
// If journalctl is unavailable or an error occurs, it returns an empty string.
// Linux-only; file has linux build tag.
func ServiceTail(unit string, maxLines int) string {
    if unit == "" {
        return ""
    }
    if maxLines <= 0 {
        maxLines = 100
    }

    j, err := sdjournal.NewJournal()
    if err != nil {
        log.Debug().Err(err).Str("unit", unit).Msg("Failed to open systemd journal")
        return ""
    }
    defer j.Close()

    // Filter by unit
    match := sdjournal.Match{
        Field: "_SYSTEMD_UNIT",
        Value: unit,
    }
    if err := j.AddMatch(match.String()); err != nil {
        log.Debug().Err(err).Str("unit", unit).Msg("Failed to add journal match")
        return ""
    }

    // Seek to tail and step back up to maxLines entries
    if err := j.SeekTail(); err != nil {
        log.Debug().Err(err).Str("unit", unit).Msg("Failed to seek to journal tail")
        return ""
    }
    // Move iterator to last entry position
    // Next returns 0 at end; call once to position at last entry
    _, _ = j.Next()

    // Step backwards to get up to maxLines entries in buffer
    back := 0
    for back < maxLines-1 {
        n, err := j.Previous()
        if err != nil || n == 0 {
            break
        }
        back++
    }

    // Now iterate forward collecting up to maxLines entries
    var builder strings.Builder
    count := 0
    for count < maxLines {
        // Read current entry
        entry, err := j.GetEntry()
        if err != nil {
            break
        }
        ts := time.Unix(0, int64(entry.RealtimeTimestamp)*int64(time.Microsecond)).Format(time.RFC3339)
        msg := entry.Fields["MESSAGE"]
        priStr := entry.Fields["PRIORITY"]
        pri := 6
        if priStr != "" {
            if v, err := strconv.Atoi(priStr); err == nil {
                pri = v
            }
        }
        builder.WriteString(fmt.Sprintf("%s [%d] %s\n", ts, pri, msg))
        count++
        n, err := j.Next()
        if err != nil || n == 0 {
            break
        }
    }

    return builder.String()
}
