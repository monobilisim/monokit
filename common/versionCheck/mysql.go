package common

import (
    "os/exec"
    "regexp"
    "strings"

    "github.com/rs/zerolog/log"
)

// MySQLCheck detects the installed MySQL/MariaDB server version, compares
// with stored value, creates news on change, and persists the current version.
func MySQLCheck() {
    // Prefer server binaries over client for authoritative version
    serverBinary := ""
    if _, err := exec.LookPath("mysqld"); err == nil {
        serverBinary = "mysqld"
    } else if _, err := exec.LookPath("mariadbd"); err == nil {
        serverBinary = "mariadbd"
    } else if _, err := exec.LookPath("mysql"); err == nil {
        // Fallback to client binary if server binary isn't available
        serverBinary = "mysql"
    } else if _, err := exec.LookPath("mariadb"); err == nil {
        // Fallback to MariaDB client
        serverBinary = "mariadb"
    }

    if serverBinary == "" {
        log.Debug().Msg("MySQL/MariaDB binaries not found, skipping MySQL version check")
        return
    }

    out, err := exec.Command(serverBinary, "--version").Output()
    if err != nil {
        log.Error().Err(err).Str("binary", serverBinary).Msg("Error getting MySQL/MariaDB version")
        return
    }

    output := strings.TrimSpace(string(out))

    // Extract semantic version like 8.0.37 or 10.11.6 from mixed outputs
    versionRegex := regexp.MustCompile(`\b(\d+\.\d+(?:\.\d+)?)\b`)
    versionMatch := versionRegex.FindString(output)

    // Determine flavor for naming and storage key
    isMaria := strings.Contains(strings.ToLower(output), "mariadb") || serverBinary == "mariadbd" || serverBinary == "mariadb"
    serviceKey := "mysql"
    serviceTitle := "MySQL"
    if isMaria {
        serviceKey = "mariadb"
        serviceTitle = "MariaDB"
    }

    if versionMatch == "" {
        // As a fallback, try token after "Ver" if present
        fields := strings.Fields(output)
        for i := 0; i < len(fields)-1; i++ {
            if strings.EqualFold(fields[i], "Ver") {
                candidate := strings.Trim(fields[i+1], ",")
                // Strip distro suffixes like -0ubuntu...
                if idx := strings.Index(candidate, "-"); idx > 0 {
                    candidate = candidate[:idx]
                }
                versionMatch = candidate
                break
            }
        }
    }

    if versionMatch == "" {
        log.Error().Str("output", output).Msg("Unable to parse MySQL/MariaDB version")
        return
    }

    oldVersion := GatherVersion(serviceKey)
    if oldVersion != "" && oldVersion == versionMatch {
        log.Debug().Str("service", serviceTitle).Msg("Version unchanged")
    } else if oldVersion != "" && oldVersion != versionMatch {
        log.Debug().Str("service", serviceTitle).Str("old_version", oldVersion).Str("new_version", versionMatch).Msg("Service has been updated")
        CreateNews(serviceTitle, oldVersion, versionMatch, false)
    } else {
        log.Debug().Str("service", serviceTitle).Str("version", versionMatch).Msg("Storing initial version")
    }

    StoreVersion(serviceKey, versionMatch)
}
