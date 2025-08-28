package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// RabbitMQCheck detects the installed RabbitMQ server version, compares with stored
// value, creates news on change, and persists the current version.
func RabbitMQCheck() {
	var version string

	// Preferred: rabbitmq-diagnostics server_version -> plain version
	if _, err := exec.LookPath("rabbitmq-diagnostics"); err == nil {
		out, err := exec.Command("rabbitmq-diagnostics", "server_version").Output()
		if err == nil {
			// Some distros print an info line before the version; extract only x.y[.z]
			version = extractRabbitMQVersion(string(out))
		} else {
			log.Debug().Err(err).Msg("rabbitmq-diagnostics server_version failed; will try rabbitmqctl")
		}
	}

	// Fallbacks using rabbitmqctl
	if version == "" {
		if _, err := exec.LookPath("rabbitmqctl"); err == nil {
			// Try simple version first (exists in newer releases)
			if out, err := exec.Command("rabbitmqctl", "version").Output(); err == nil {
				version = extractRabbitMQVersion(string(out))
			} else {
				// Try status and extract a semantic version
				if out2, err2 := exec.Command("rabbitmqctl", "status").Output(); err2 == nil {
					version = extractRabbitMQVersion(string(out2))
				} else {
					log.Debug().Err(err2).Msg("rabbitmqctl status failed")
				}
			}
		}
	}

	if version == "" {
		log.Debug().Msg("RabbitMQ binaries not found or version could not be determined; skipping")
		addToNotInstalled("RabbitMQ")
		return
	}

	serviceKey := "rabbitmq"
	serviceTitle := "RabbitMQ"

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

// extractRabbitMQVersion tries to find a semantic version like 3.12 or 3.12.8
// in an arbitrary command output. Returns empty string if not found.
func extractRabbitMQVersion(s string) string {
	txt := strings.TrimSpace(s)
	re := regexp.MustCompile(`\b(\d+\.\d+(?:\.\d+)?)\b`)
	return re.FindString(txt)
}
