package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// proxysqlVersionRegex parses `proxysql --version`, which prints a single line:
//
//	ProxySQL version 3.0.11-797-g7c91137, codename Truls
//
// Everything up to the comma is kept: release builds report a plain "2.6.3" or
// a vendor build such as "2.5.5-percona-1.1", and git builds append the commit.
var proxysqlVersionRegex = regexp.MustCompile(`(?im)^ProxySQL\s+version\s+([^,\s]+)`)

// proxysqlImageRegex matches the ProxySQL image.
var proxysqlImageRegex = regexp.MustCompile(`(?i)(?:^|/)proxysql:([\w.-]+)$`)

// ProxySQLCheck detects the installed ProxySQL version and records updates.
func ProxySQLCheck() {
	if _, err := exec.LookPath("proxysql"); err == nil {
		out, err := exec.Command("proxysql", "--version").CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Msg("proxysql --version failed; will try docker fallback")
		} else {
			text := strings.TrimSpace(string(out))
			if m := proxysqlVersionRegex.FindStringSubmatch(text); len(m) == 2 {
				recordVersion("ProxySQL", "proxysql", m[1])
				return
			}
			log.Debug().Str("output", text).Msg("Could not parse ProxySQL version; will try docker fallback")
		}
	}

	if tag := dockerImageVersion(proxysqlImageRegex); tag != "" {
		recordVersion("ProxySQL", "proxysql", tag)
		return
	}

	log.Debug().Msg("ProxySQL version not detected; skipping")
	addToNotInstalled("ProxySQL")
}
