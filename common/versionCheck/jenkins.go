package common

import (
	"fmt"
	"os/exec"

	"github.com/rs/zerolog/log"
)

func JenkinsCheck() (string, error) {
	_, err := exec.LookPath("jenkins")
	if err != nil {
		addToNotInstalled("Jenkins")
		return "", nil
	}

	out, err := exec.Command("jenkins", "--version").Output()
	if err != nil {
		errMsg := "Error getting Jenkins version: " + err.Error()
		log.Error().Msg(errMsg)
		addToVersionErrors(fmt.Errorf(errMsg))
		return "", fmt.Errorf("%s", errMsg)
	}

	version := string(out)

	if version == "" {
		errMsg := "`jenkins --version` returns empty"
		log.Error().Str("output", version).Msg(errMsg)
		addToVersionErrors(fmt.Errorf(errMsg))
		return "", fmt.Errorf(errMsg)
	}

	log.Debug().Str("version", version).Msg("Detected Jenkins version")

	oldVersion := GatherVersion("jenkins")

	if oldVersion != "" && oldVersion == version {
		log.Debug().Msg("Jenkins version unchanged.")
		addToNotUpdated(AppVersion{Name: "Jenkins", OldVersion: oldVersion, NewVersion: version})
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("Jenkins has been updated")
		addToUpdated(AppVersion{Name: "Jenkins", OldVersion: oldVersion, NewVersion: version})
		CreateNews("Jenkins", oldVersion, version, false)
	} else {
		log.Debug().Msg("Storing initial Jenkins version: " + version)
		addToNotUpdated(AppVersion{Name: "Jenkins", OldVersion: version})
	}

	StoreVersion("jenkins", version)
	return version, nil
}
