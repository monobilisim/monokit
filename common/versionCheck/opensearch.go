package common

import (
	"os"
	"regexp"

	"github.com/rs/zerolog/log"
)

// opensearchJarRegex matches the core OpenSearch jar, e.g. "opensearch-3.8.0.jar".
// Requiring a digit straight after the dash keeps it off the sibling jars that
// share the prefix, such as opensearch-core-3.8.0.jar or opensearch-cli-3.8.0.jar.
var opensearchJarRegex = regexp.MustCompile(`^opensearch-(\d[\w.-]*)\.jar$`)

// opensearchImageRegex matches the OpenSearch server image but not
// opensearchproject/opensearch-dashboards.
var opensearchImageRegex = regexp.MustCompile(`(?i)(?:^|/)opensearch:([\w.-]+)$`)

// opensearchLibPaths are the lib directories of a packaged OpenSearch install.
var opensearchLibPaths = []string{
	"/usr/share/opensearch/lib",
	"/opt/opensearch/lib",
}

// OpenSearchCheck detects the installed OpenSearch version and records updates.
//
// The core jar carries the release in its filename, which is both cheaper and
// tidier than `opensearch --version`: that boots a JVM and prints its banner
// after a block of unrelated JVM deprecation warnings.
func OpenSearchCheck() {
	for _, dir := range opensearchLibPaths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if m := opensearchJarRegex.FindStringSubmatch(entry.Name()); len(m) == 2 {
				recordVersion("OpenSearch", "opensearch", m[1])
				return
			}
		}

		log.Debug().Str("dir", dir).Msg("No OpenSearch core jar found; will try docker fallback")
	}

	if tag := dockerImageVersion(opensearchImageRegex); tag != "" {
		recordVersion("OpenSearch", "opensearch", tag)
		return
	}

	log.Debug().Msg("OpenSearch version not detected; skipping")
	addToNotInstalled("OpenSearch")
}
