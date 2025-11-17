package common

import (
	"fmt"
	"os/exec"

	"github.com/rs/zerolog/log"
)

func AsteriskCheck() (string, error) {
	_, err := exec.LookPath("asterisk")
	if err != nil {
		addToNotInstalled("Asterisk")
		return "", nil
	}

	out, err := exec.Command("asterisk", "-V").Output()
	if err != nil {
		errMsg := "Error getting Asterisk version: " + err.Error()
		log.Error().Msg(errMsg)
		addToVersionErrors(fmt.Errorf(errMsg))
		return "", fmt.Errorf("%s", errMsg)
	}

	version := string(out)

	if version == "" {
		errMsg := "`asterisk -V` returns empty"
		log.Error().Str("output", version).Msg(errMsg)
		addToVersionErrors(fmt.Errorf(errMsg))
		return "", fmt.Errorf(errMsg)
	}

	log.Debug().Str("version", version).Msg("Detected Asterisk version")

	oldVersion := GatherVersion("asterisk")

	if oldVersion != "" && oldVersion == version {
		log.Debug().Msg("Asterisk version unchanged.")
		addToNotUpdated(AppVersion{Name: "Asterisk", OldVersion: oldVersion, NewVersion: version})
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("Asterisk has been updated")
		addToUpdated(AppVersion{Name: "Asterisk", OldVersion: oldVersion, NewVersion: version})
		CreateNews("Asterisk", oldVersion, version, false)
	} else {
		log.Debug().Msg("Storing initial Asterisk version: " + version)
		addToNotUpdated(AppVersion{Name: "Asterisk", OldVersion: version})
	}

	StoreVersion("asterisk", version)
	return version, nil
}
