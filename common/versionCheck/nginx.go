package common

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// NginxCheck detects installed Nginx version using `nginx -v` and records updates.
func NginxCheck() {
	if _, err := exec.LookPath("nginx"); err != nil {
		log.Debug().Msg("nginx binary not found, skipping version check")
		addToNotInstalled("Nginx")
		return
	}

	// nginx prints version to stderr with -v
	out, err := exec.Command("nginx", "-v").CombinedOutput()
	if err != nil {
		// Some nginx builds return non-zero when only -v is given; still parse output
		log.Debug().Err(err).Msg("nginx -v returned non-zero; attempting to parse output")
	}

	text := strings.TrimSpace(string(out))
	// Expected like: "nginx version: nginx/1.24.0" or "nginx version: openresty/1.21.4.1"
	// Extract the part after the slash.
	version := ""
	re := regexp.MustCompile(`(?i)nginx version:\s+[^/]+/([\w\.-]+)`) // capture group for version
	if m := re.FindStringSubmatch(text); len(m) == 2 {
		version = m[1]
	}

	if version == "" {
		// Try alternative format with -V (uppercase) which prints longer config
		out2, _ := exec.Command("nginx", "-V").CombinedOutput()
		text2 := strings.TrimSpace(string(out2))
		if m := re.FindStringSubmatch(text2); len(m) == 2 {
			version = m[1]
		}
	}

	if version == "" {
		log.Error().Str("output", text).Msg("Unable to parse nginx version")
		addToVersionErrors(fmt.Errorf("Unable to parse nginx version"))
		return
	}

	oldVersion := GatherVersion("nginx")
	if oldVersion != "" && oldVersion == version {
		log.Debug().Msg("nginx version unchanged")
		addToNotUpdated(AppVersion{Name: "Nginx", OldVersion: oldVersion, NewVersion: version})
	} else if oldVersion != "" && oldVersion != version {
		log.Debug().Str("old_version", oldVersion).Str("new_version", version).Msg("nginx updated")
		addToUpdated(AppVersion{Name: "Nginx", OldVersion: oldVersion, NewVersion: version})
		CreateNews("Nginx", oldVersion, version, false)
	} else {
		log.Debug().Str("version", version).Msg("Storing initial nginx version")
		addToNotUpdated(AppVersion{Name: "Nginx", OldVersion: version})
	}

	StoreVersion("nginx", version)
}
