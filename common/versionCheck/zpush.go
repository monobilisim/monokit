package common

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	monocommon "github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
	"github.com/rs/zerolog/log"
)

// zpushVersionPath holds the Z-Push core version, e.g. "2.7.7\n". The Zimbra
// backend ships separately (Release N in backend/zimbra/) and is not tracked
// here: the file describes the core only.
const zpushVersionPath = "/opt/z-push/version"

// zpushReleaseAPI returns the newest published Z-Push core release. endoflife.date
// has no z-push product, so the GitHub releases endpoint is the only source.
const zpushReleaseAPI = "https://api.github.com/repos/Z-Hub/Z-Push/releases/latest"

// ZPushCheck records the installed Z-Push core version and alarms when upstream
// has published a newer one.
func ZPushCheck() {
	content, err := os.ReadFile(zpushVersionPath)
	if err != nil {
		log.Debug().Msg("z-push not found, skipping Z-Push version check")
		addToNotInstalled("Z-Push")
		return
	}

	version := strings.TrimSpace(string(content))
	if version == "" {
		log.Error().Str("path", zpushVersionPath).Msg("Empty Z-Push version file")
		addToVersionErrors(fmt.Errorf("empty Z-Push version file %s", zpushVersionPath))
		return
	}

	recordVersion("Z-Push", "zpush", version)
	checkZPushLatest(version)
}

// checkZPushLatest alarms once per upstream release when a newer Z-Push exists.
// The stored state is keyed by the installed version, so upgrading the local
// install re-arms the check instead of inheriting the previous silence.
func checkZPushLatest(current string) {
	key := "zpush_" + current
	jsonStr, _, _, found, _ := healthdb.GetJSON("zpush_release", key)

	var state struct {
		NotifiedVersion string    `json:"notified_version"`
		LastCheckTime   time.Time `json:"last_check_time"`
	}
	if found && jsonStr != "" {
		json.Unmarshal([]byte(jsonStr), &state)
	}

	if state.LastCheckTime.After(time.Now().Add(-24 * time.Hour)) {
		return
	}

	latest, err := fetchZPushLatest()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to fetch latest Z-Push release")
		return
	}

	if zpushShouldAlarm(latest, current, state.NotifiedVersion) {
		msg := fmt.Sprintf("[versionCheck - %s] [:warning:] New Z-Push release: %s is out (installed: %s)", monocommon.Config.Identifier, latest, current)
		log.Info().Str("latest", latest).Str("installed", current).Msg("New Z-Push release available")
		monocommon.Alarm(msg, "", "", false)
		state.NotifiedVersion = latest
	}

	state.LastCheckTime = time.Now()
	b, _ := json.Marshal(state)
	healthdb.PutJSON("zpush_release", key, string(b), nil, time.Now())
}

// zpushShouldAlarm reports whether upstream's latest release warrants a
// notification: it must be strictly newer than the installed version, and not
// already announced. Comparing against the installed version (rather than a
// stored one) means an upgrade silently re-arms the check.
func zpushShouldAlarm(latest, current, alreadyNotified string) bool {
	return compareVersions(latest, current) > 0 && alreadyNotified != latest
}

// fetchZPushLatest returns the tag of the newest Z-Push release with the leading
// "v" trimmed, matching how the local version file is formatted.
func fetchZPushLatest() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", zpushReleaseAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Monokit/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}

	tag := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if tag == "" {
		return "", fmt.Errorf("empty tag_name in github response")
	}

	return tag, nil
}
