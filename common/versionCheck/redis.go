package common

import (
    "os/exec"
    "strings"

    "github.com/rs/zerolog/log"
)

// RedisCheck detects the installed Redis server version, compares with stored
// value, creates news on change, and persists the current version.
func RedisCheck() {
    // Check if redis-server exists
    if _, err := exec.LookPath("redis-server"); err != nil {
        log.Debug().Msg("redis-server not found, skipping Redis version check")
        return
    }

    // `redis-server --version` outputs like:
    // Redis server v=7.2.5 sha=00000000:0 malloc=libc bits=64 build=...
    out, err := exec.Command("redis-server", "--version").Output()
    if err != nil {
        log.Error().Err(err).Msg("Error getting Redis version")
        return
    }

    text := strings.TrimSpace(string(out))
    // Find token starting with v=
    var version string
    for _, tok := range strings.Fields(text) {
        if strings.HasPrefix(tok, "v=") {
            version = strings.TrimPrefix(tok, "v=")
            break
        }
    }

    if version == "" {
        log.Error().Str("output", text).Msg("Unable to parse Redis version")
        return
    }

    oldVersion := GatherVersion("redis")
    if oldVersion != "" && oldVersion == version {
        log.Debug().Msg("Redis version unchanged")
    } else if oldVersion != "" && oldVersion != version {
        log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("Redis has been updated")
        CreateNews("Redis", oldVersion, version, false)
    } else {
        log.Debug().Str("version", version).Msg("Storing initial Redis version")
    }

    StoreVersion("redis", version)
}
