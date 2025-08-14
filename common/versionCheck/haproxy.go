package common

import (
    "os/exec"
    "regexp"
    "strings"

    "github.com/rs/zerolog/log"
)

// HAProxyCheck detects the installed HAProxy version and reports updates.
func HAProxyCheck() {
    if _, err := exec.LookPath("haproxy"); err != nil {
        log.Debug().Msg("haproxy binary not found, skipping version check")
        return
    }

    // `haproxy -v` prints a line like:
    // "HA-Proxy version 2.8.9-1e87e80 2025/05/30 - https://haproxy.org/"
    out, err := exec.Command("haproxy", "-v").CombinedOutput()
    if err != nil {
        log.Debug().Err(err).Msg("haproxy -v returned non-zero; attempting to parse output")
    }

    text := strings.TrimSpace(string(out))
    // Match both "HA-Proxy" and "HAProxy" and allow '+' in version (e.g., 2.6.12-1+deb12u2)
    re := regexp.MustCompile(`(?i)HA-?Proxy version\s+([\w\.\-\+]+)`) // capture version token
    version := ""
    if m := re.FindStringSubmatch(text); len(m) == 2 {
        version = m[1]
    }

    if version == "" {
        log.Error().Str("output", text).Msg("Unable to parse HAProxy version")
        return
    }

    oldVersion := GatherVersion("haproxy")
    if oldVersion != "" && oldVersion == version {
        log.Debug().Msg("HAProxy version unchanged")
    } else if oldVersion != "" && oldVersion != version {
        log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("HAProxy updated")
        CreateNews("HAProxy", oldVersion, version, false)
    } else {
        log.Debug().Str("version", version).Msg("Storing initial HAProxy version")
    }

    StoreVersion("haproxy", version)
}
