package common

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// MySQLCheck detects the installed MySQL/MariaDB server version, compares
// with stored value, creates news on change, and persists the current version.
func MySQLCheck() {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("/usr/sbin:%s", currentPath)
	os.Setenv("PATH", newPath)

	serverBinary := ""
	if _, err := exec.LookPath("mysqld"); err == nil {
		serverBinary = "mysqld"
	} else if _, err := exec.LookPath("mariadbd"); err == nil {
		serverBinary = "mariadbd"
	}

	if serverBinary == "" {
		log.Debug().Msg("MySQL/MariaDB binaries not found, skipping MySQL version check")
		addToNotInstalled("MySQL/MariaDB")
		return
	}

	out, err := exec.Command(serverBinary, "--version").Output()
	if err != nil {
		log.Error().Err(err).Str("binary", serverBinary).Msg("Error getting MySQL/MariaDB version")
		addToVersionErrors(fmt.Errorf("Error getting MySQL/MariaDB version: %s", err.Error()))
		return
	}

	output := strings.TrimSpace(string(out))

	isMaria := strings.Contains(strings.ToLower(output), "mariadb") || serverBinary == "mariadbd" || serverBinary == "mariadb"
	serviceKey := "mysql"
	serviceTitle := "MySQL"
	if isMaria {
		serviceKey = "mariadb"
		serviceTitle = "MariaDB"
	}

	var versionMatch string
	if serverBinary == "mysql" || serverBinary == "mariadb" {
		if isMaria {
			fromRegex := regexp.MustCompile(`from\s+([^\s,\-]+)`)
			if matches := fromRegex.FindStringSubmatch(output); len(matches) > 1 {
				versionMatch = matches[1]
			}
		} else {
			distribRegex := regexp.MustCompile(`Distrib\s+([^\s,]+)`)
			if matches := distribRegex.FindStringSubmatch(output); len(matches) > 1 {
				versionMatch = matches[1]
			}
		}
	} else {
		versionRegex := regexp.MustCompile(`\b(\d+\.\d+(?:\.\d+)?)\b`)
		versionMatch = versionRegex.FindString(output)
	}

	if versionMatch == "" {
		distribRegex := regexp.MustCompile(`Distrib\s+([^\s,]+)`)
		if matches := distribRegex.FindStringSubmatch(output); len(matches) > 1 {
			versionMatch = matches[1]
		}
	}

	if versionMatch == "" {
		fields := strings.Fields(output)
		for i := 0; i < len(fields)-1; i++ {
			if strings.EqualFold(fields[i], "Ver") {
				candidate := strings.Trim(fields[i+1], ",")
				if idx := strings.Index(candidate, "-"); idx > 0 {
					candidate = candidate[:idx]
				}
				versionMatch = candidate
				break
			}
		}
	}

	if versionMatch == "" {
		versionRegex := regexp.MustCompile(`\b(\d+\.\d+(?:\.\d+)?)\b`)
		versionMatch = versionRegex.FindString(output)
	}

	if versionMatch == "" {
		log.Error().Str("output", output).Msg("Unable to parse MySQL/MariaDB version")
		addToVersionErrors(fmt.Errorf("Unable to parse MySQL/MariaDB version"))
		return
	}

	versionMatch = strings.TrimSuffix(versionMatch, "-MariaDB")
	oldVersion := GatherVersion(serviceKey)
	if oldVersion != "" && oldVersion == versionMatch {
		log.Debug().Str("service", serviceTitle).Msg("Version unchanged")
		addToNotUpdated(AppVersion{Name: serviceTitle, OldVersion: oldVersion, NewVersion: versionMatch})
	} else if oldVersion != "" && oldVersion != versionMatch {
		log.Debug().Str("service", serviceTitle).Str("old_version", oldVersion).Str("new_version", versionMatch).Msg("Service has been updated")
		addToUpdated(AppVersion{Name: serviceTitle, OldVersion: oldVersion, NewVersion: versionMatch})
		CreateNews(serviceTitle, oldVersion, versionMatch, false)
	} else {
		log.Debug().Str("service", serviceTitle).Str("version", versionMatch).Msg("Storing initial version")
		addToNotUpdated(AppVersion{Name: serviceTitle, OldVersion: versionMatch})
	}

	StoreVersion(serviceKey, versionMatch)
}
