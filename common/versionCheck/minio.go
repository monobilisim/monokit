package common

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// minioVersionRegex extracts the release string from `minio --version`, whose first
// line reads "minio version RELEASE.2024-01-16T16-07-38Z (commit-id=...)".
// Source builds report "DEVELOPMENT.GOGET" in the same position.
var minioVersionRegex = regexp.MustCompile(`(?im)^minio\s+version\s+(\S+)`)

// minioImageRegex matches the MinIO server image (minio/minio, quay.io/minio/minio,
// bitnami/minio) but not the mc client image minio/mc.
var minioImageRegex = regexp.MustCompile(`(?i)(?:^|/)minio:([\w.-]+)$`)

// MinIOCheck detects the installed MinIO version and records updates.
// Uses the minio binary when present; falls back to the running container image
// tag for containerised deployments.
func MinIOCheck() {
	// Case 1: minio binary present
	if _, err := exec.LookPath("minio"); err == nil {
		out, err := exec.Command("minio", "--version").CombinedOutput()
		if err != nil {
			log.Debug().Err(err).Msg("minio --version failed; will try docker fallback")
		} else {
			text := strings.TrimSpace(string(out))
			if m := minioVersionRegex.FindStringSubmatch(text); len(m) == 2 {
				recordVersion("MinIO", "minio", m[1])
				return
			}
			log.Debug().Str("output", text).Msg("Could not parse MinIO version output; will try docker fallback")
		}
	}

	// Case 2: infer the version from the running container image tag
	if tag := dockerImageVersion(minioImageRegex); tag != "" {
		recordVersion("MinIO", "minio", tag)
		return
	}

	log.Debug().Msg("MinIO version not detected; skipping")
	addToNotInstalled("MinIO")
}
