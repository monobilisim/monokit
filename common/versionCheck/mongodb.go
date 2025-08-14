package common

import (
    "fmt"
    "os/exec"
    "strings"

    "github.com/rs/zerolog/log"
)

// MongoDBCheck detects the installed MongoDB (mongod) version, compares it with the previously stored version,
// potentially creates a Redmine news item, stores the current version, and returns the current version string.
func MongoDBCheck() (string, error) {
    // Check if mongod binary is installed
    _, err := exec.LookPath("mongod")
    if err != nil {
        fmt.Println("MongoDB is not installed on this system.")
        return "", nil // Not an error, just not installed
    }

    // Get the version of mongod
    // Typical output starts with a line like: "db version v8.0.12"
    out, err := exec.Command("mongod", "-version").Output()
    if err != nil {
        errMsg := "Error getting mongod version: " + err.Error()
        log.Error().Msg(errMsg)
        return "", fmt.Errorf("%s", errMsg)
    }

    versionOutput := string(out)
    var version string

    // Prefer the line starting with "db version"
    for _, line := range strings.Split(versionOutput, "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(strings.ToLower(line), "db version") {
            // Expected tokens: ["db", "version", "v8.0.12"]
            fields := strings.Fields(line)
            if len(fields) >= 3 {
                version = strings.TrimPrefix(fields[2], "v")
            }
            break
        }
    }

    // Fallback: try to find JSON field \"version\": \"x.y.z\" in the Build Info block
    if version == "" {
        // crude but effective: find the first occurrence of '"version"' and extract the quoted value following it
        lower := versionOutput
        idx := strings.Index(lower, "\"version\"")
        if idx != -1 {
            after := lower[idx+len("\"version\""):]
            // find the first quote after the colon
            colon := strings.Index(after, ":")
            if colon != -1 {
                rest := after[colon+1:]
                // trim spaces
                rest = strings.TrimSpace(rest)
                if strings.HasPrefix(rest, "\"") {
                    rest = rest[1:]
                    end := strings.Index(rest, "\"")
                    if end != -1 {
                        version = rest[:end]
                    }
                }
            }
        }
    }

    if version == "" {
        errMsg := "Unable to parse MongoDB version from mongod -version output"
        log.Error().Str("output", versionOutput).Msg(errMsg)
        return "", fmt.Errorf(errMsg)
    }

    log.Debug().Str("version", version).Msg("Detected MongoDB version")

    oldVersion := GatherVersion("mongodb")

    if oldVersion != "" && oldVersion == version {
        log.Debug().Msg("MongoDB version unchanged.")
    } else if oldVersion != "" && oldVersion != version {
        log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("MongoDB has been updated")
        CreateNews("MongoDB", oldVersion, version, false)
    } else {
        log.Debug().Msg("Storing initial MongoDB version: " + version)
    }

    StoreVersion("mongodb", version)
    return version, nil
}
