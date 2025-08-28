package common

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/rs/zerolog/log"
)

// ZabbixCheck detects the installed Zabbix Server version (via zabbix_server -V),
// compares with stored value, creates news on change, and persists the current version.
func ZabbixCheck() {
	if _, err := exec.LookPath("zabbix_server"); err != nil {
		log.Debug().Msg("zabbix_server not found, skipping Zabbix version check")
		addToNotInstalled("Zabbix")
		return
	}

	out, err := exec.Command("zabbix_server", "-V").Output()
	if err != nil {
		log.Error().Err(err).Msg("Error getting Zabbix server version")
		addToVersionErrors(fmt.Errorf("Error getting Zabbix server version: %s", err.Error()))
		return
	}

	// Typical first line: "zabbix_server (Zabbix) 6.0.18"
	// Extract the first semantic version x.y or x.y.z from the whole output
	text := string(out)
	re := regexp.MustCompile(`\b(\d+\.\d+(?:\.\d+)?)\b`)
	version := re.FindString(text)

	if version == "" {
		log.Error().Str("output", text).Msg("Unable to parse Zabbix version")
		addToVersionErrors(fmt.Errorf("Unable to parse Zabbix version"))
		return
	}

	serviceKey := "zabbix"
	serviceTitle := "Zabbix"

	oldVersion := GatherVersion(serviceKey)
	if oldVersion != "" && oldVersion == version {
		log.Debug().Str("service", serviceTitle).Msg("Version unchanged")
		addToNotUpdated(AppVersion{Name: serviceTitle, OldVersion: oldVersion, NewVersion: version})
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Str("service", serviceTitle).Str("old_version", oldVersion).Str("new_version", version).Msg("Service has been updated")
		addToUpdated(AppVersion{Name: serviceTitle, OldVersion: oldVersion, NewVersion: version})
		CreateNews(serviceTitle, oldVersion, version, false)
	} else {
		log.Debug().Str("service", serviceTitle).Str("version", version).Msg("Storing initial version")
		addToNotUpdated(AppVersion{Name: serviceTitle, OldVersion: version})
	}

	StoreVersion(serviceKey, version)
}
