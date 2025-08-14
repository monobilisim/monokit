package common

import (
    "fmt"
    "os/exec"
    "strings"

    "github.com/rs/zerolog/log"
)

// CaddyCheck detects the installed Caddy version, compares with the stored version,
// creates a news item on change, and persists the current version.
func CaddyCheck() (string, error) {
    // Check if Caddy binary is installed
    _, err := exec.LookPath("caddy")
    if err != nil {
        log.Debug().Msg("Caddy binary not found, skipping version check")
        return "", nil // Not an error, just not installed
    }

    // Get Caddy version
    // Typical outputs:
    // - "v2.7.6 h1:..."
    // - "Caddy v2.9.1 h1:..."
    out, err := exec.Command("caddy", "version").Output()
    if err != nil {
        errMsg := "Error getting Caddy version: " + err.Error()
        log.Error().Msg(errMsg)
        return "", fmt.Errorf("%s", errMsg)
    }

    versionOutput := strings.TrimSpace(string(out))
    fields := strings.Fields(versionOutput)
    if len(fields) == 0 {
        errMsg := "Unexpected output format from caddy version: " + versionOutput
        log.Error().Msg(errMsg)
        return "", fmt.Errorf("%s", errMsg)
    }

    first := fields[0]
    if strings.EqualFold(first, "caddy") {
        if len(fields) < 2 {
            errMsg := "Unexpected output format from caddy version: " + versionOutput
            log.Error().Msg(errMsg)
            return "", fmt.Errorf("%s", errMsg)
        }
        first = fields[1]
    }
    version := strings.TrimPrefix(first, "v")

    log.Debug().Str("version", version).Msg("Detected Caddy version")

    oldVersion := GatherVersion("caddy")

    if oldVersion != "" && oldVersion == version {
        log.Debug().Msg("Caddy version unchanged.")
    } else if oldVersion != "" && oldVersion != version {
        log.Debug().Msg("Caddy has been updated.")
        log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("Caddy has been updated")
        CreateNews("Caddy", oldVersion, version, false)
    } else {
        log.Debug().Msg("Storing initial Caddy version: " + version)
    }

    StoreVersion("caddy", version)
    return version, nil
}
