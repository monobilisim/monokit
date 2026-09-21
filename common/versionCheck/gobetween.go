package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// gobetweenVersionRegex matches the bare version gobetween writes to stdout for
// `--version` ("0.8.1" on a line of its own).
var gobetweenVersionRegex = regexp.MustCompile(`(?m)^v?(\d+\.\d+\.\d+\S*)\s*$`)

// gobetweenBannerRegex is the fallback: gobetween also logs a timestamped
// "gobetween v0.8.1" banner to stderr on every start, which is all that is left
// if a build stops printing the bare version.
var gobetweenBannerRegex = regexp.MustCompile(`(?i)gobetween\s+v?(\d+\.\d+\.\d+\S*)`)

// gobetweenImageRegex matches the gobetween image (published as yyyar/gobetween).
var gobetweenImageRegex = regexp.MustCompile(`(?i)(?:^|/)gobetween:([\w.-]+)$`)

// GobetweenCheck detects the installed gobetween version and records updates.
func GobetweenCheck() {
	if _, err := exec.LookPath("gobetween"); err == nil {
		out, err := exec.Command("gobetween", "--version").CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Msg("gobetween --version failed; will try docker fallback")
		} else {
			text := strings.TrimSpace(string(out))
			for _, re := range []*regexp.Regexp{gobetweenVersionRegex, gobetweenBannerRegex} {
				if m := re.FindStringSubmatch(text); len(m) == 2 {
					recordVersion("Gobetween", "gobetween", m[1])
					return
				}
			}
			log.Debug().Str("output", text).Msg("Could not parse gobetween version; will try docker fallback")
		}
	}

	if tag := dockerImageVersion(gobetweenImageRegex); tag != "" {
		recordVersion("Gobetween", "gobetween", tag)
		return
	}

	log.Debug().Msg("Gobetween version not detected; skipping")
	addToNotInstalled("Gobetween")
}
