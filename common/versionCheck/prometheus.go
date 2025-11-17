package common

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/rs/zerolog/log"
)

var prometheusVersionRegex = regexp.MustCompile(`version ([^ ]+)`)

func PrometheusCheck() (string, error) {
	_, err := exec.LookPath("prometheus")
	if err != nil {
		addToNotInstalled("Prometheus")
		return "", nil
	}

	out, err := exec.Command("prometheus", "--version").Output()
	if err != nil {
		errMsg := "Error getting Prometheus version: " + err.Error()
		log.Error().Msg(errMsg)
		addToVersionErrors(fmt.Errorf(errMsg))
		return "", fmt.Errorf("%s", errMsg)
	}

	outputStr := string(out)

	if outputStr == "" {
		errMsg := "`prometheus --version` returns empty"
		log.Error().Str("output", outputStr).Msg(errMsg)
		addToVersionErrors(fmt.Errorf(errMsg))
		return "", fmt.Errorf(errMsg)
	}

	matches := prometheusVersionRegex.FindStringSubmatch(outputStr)
	if len(matches) < 2 {
		errMsg := "Could not parse Prometheus version from output"
		log.Error().Str("output", outputStr).Msg(errMsg)
		addToVersionErrors(fmt.Errorf(errMsg))
		return "", fmt.Errorf(errMsg)
	}
	version := matches[1]

	log.Debug().Str("version", version).Msg("Detected Prometheus version")

	oldVersion := GatherVersion("prometheus")

	if oldVersion != "" && oldVersion == version {
		log.Debug().Msg("Prometheus version unchanged.")
		addToNotUpdated(AppVersion{Name: "Prometheus", OldVersion: oldVersion, NewVersion: version})
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("Prometheus has been updated")
		addToUpdated(AppVersion{Name: "Prometheus", OldVersion: oldVersion, NewVersion: version})
		CreateNews("Prometheus", oldVersion, version, false)
	} else {
		log.Debug().Msg("Storing initial Prometheus version: " + version)
		addToNotUpdated(AppVersion{Name: "Prometheus", OldVersion: version})
	}

	StoreVersion("prometheus", version)
	return version, nil
}
