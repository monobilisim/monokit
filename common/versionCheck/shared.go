package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// promCommonVersionRegex parses the version banner printed by tools built on
// prometheus/common, e.g.
//
//	tempo, version v3.0.0 (branch: main, revision: 2e3dabbc0)
//	Mimir, version 3.2.1 (branch: HEAD, revision: e49585d4)
//
// The leading "v" is optional: Tempo prints it, Mimir and Prometheus do not.
var promCommonVersionRegex = regexp.MustCompile(`(?im)^[\w-]+,\s+version\s+v?(\S+)`)

// floatingTags carry no version information, so a container running one tells us
// nothing about which release is deployed.
var floatingTags = map[string]bool{"latest": true, "main": true, "master": true, "stable": true}

// dockerImageVersion returns the tag of the first running container whose image
// matches re, with any leading "v" trimmed. It returns "" when docker is absent,
// the command fails, or nothing matches — all of which mean "not detected here",
// leaving the caller to fall through to its next detection method.
func dockerImageVersion(re *regexp.Regexp) string {
	if _, err := exec.LookPath("docker"); err != nil {
		return ""
	}

	out, err := exec.Command("docker", "ps", "--format", "{{.Image}}").Output()
	if err != nil {
		log.Debug().Err(err).Msg("docker ps failed during version detection")
		return ""
	}

	for _, image := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		m := re.FindStringSubmatch(strings.TrimSpace(image))
		if len(m) != 2 {
			continue
		}

		tag := strings.TrimPrefix(m[1], "v")
		if floatingTags[strings.ToLower(tag)] {
			continue
		}

		return tag
	}

	return ""
}

// recordVersion compares version with the value stored under storeKey, files a
// Redmine news item when it changed, and persists the current value.
func recordVersion(name string, storeKey string, version string) {
	log.Debug().Str("app", name).Str("version", version).Msg("Detected version")

	oldVersion := GatherVersion(storeKey)

	if oldVersion != "" && oldVersion == version {
		log.Debug().Str("app", name).Msg("Version unchanged")
		addToNotUpdated(AppVersion{Name: name, OldVersion: oldVersion, NewVersion: version})
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Str("app", name).Str("old_version", oldVersion).Str("new_version", version).Msg("Version has been updated")
		addToUpdated(AppVersion{Name: name, OldVersion: oldVersion, NewVersion: version})
		CreateNews(name, oldVersion, version, false)
	} else {
		log.Debug().Str("app", name).Str("version", version).Msg("Storing initial version")
		addToNotUpdated(AppVersion{Name: name, OldVersion: version})
	}

	StoreVersion(storeKey, version)
}
