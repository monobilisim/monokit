package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// phpVersionRegex matches the first line of `php --version` / `php-fpm -v`:
//
//	PHP 8.5.10 (cli) (built: Sep 19 2026 00:23:53) (NTS)
//	PHP 8.5.10 (fpm-fcgi) (built: Sep 19 2026 00:25:25) (NTS)
//
// Distro builds put their package revision in the same token
// ("PHP 8.1.2-1ubuntu2.14"), so the trailing \S* keeps it rather than
// silently reporting a version the host is not actually running.
var phpVersionRegex = regexp.MustCompile(`(?im)^PHP\s+(\d+\.\d+\.\d+\S*)`)

// phpVersionCommands are tried in order. php-fpm is the fallback for hosts that
// run PHP only behind a web server and have no CLI binary installed.
var phpVersionCommands = [][]string{
	{"php", "--version"},
	{"php-fpm", "-v"},
}

// PHPCheck detects the installed PHP version and records updates.
//
// Only the default php/php-fpm on PATH is reported: a host with several
// co-installed versions (php8.1 alongside php8.3) records whichever one the
// alternatives system currently points at.
func PHPCheck() {
	for _, argv := range phpVersionCommands {
		if _, err := exec.LookPath(argv[0]); err != nil {
			continue
		}

		out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Str("command", argv[0]).Msg("PHP version command failed")
			continue
		}

		text := strings.TrimSpace(string(out))
		if m := phpVersionRegex.FindStringSubmatch(text); len(m) == 2 {
			recordVersion("PHP", "php", m[1])
			return
		}
		log.Debug().Str("command", argv[0]).Str("output", text).Msg("Could not parse PHP version")
	}

	log.Debug().Msg("PHP version not detected; skipping")
	addToNotInstalled("PHP")
}
