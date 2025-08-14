package common

import (
    "os/exec"
    "regexp"
    "strings"

    "github.com/rs/zerolog/log"
)

// PostalCheck detects Postal version and records updates.
// Tries `postal version` first; falls back to reading running container image tags via `docker ps`.
func PostalCheck() {
    // Case 1: postal CLI present
    if _, err := exec.LookPath("postal"); err == nil {
        if out, err := exec.Command("postal", "version").CombinedOutput(); err == nil {
            text := strings.TrimSpace(string(out))
            // Expected formats: "Postal v2.1.0" or just "2.1.0"
            re := regexp.MustCompile(`(?i)postal\s*v?([0-9]+\.[0-9]+\.[0-9]+)`) // capture semver
            version := ""
            if m := re.FindStringSubmatch(text); len(m) == 2 {
                version = m[1]
            } else {
                // If whole output is just a version
                re2 := regexp.MustCompile(`^v?([0-9]+\.[0-9]+\.[0-9]+)$`)
                if m := re2.FindStringSubmatch(text); len(m) == 2 {
                    version = m[1]
                }
            }

            if version != "" {
                recordPostalVersion(version)
                return
            }
        } else {
            log.Debug().Err(err).Msg("postal version failed; will try docker fallback")
        }
    }

    // Case 2: Use docker to infer version from image tag if postal containers exist
    if _, err := exec.LookPath("docker"); err == nil {
        if out, err := exec.Command("docker", "ps", "--format", "{{.Image}}\t{{.Names}}").Output(); err == nil {
            lines := strings.Split(strings.TrimSpace(string(out)), "\n")
            // Look for images like ghcr.io/postalserver/postal:2.1.0
            re := regexp.MustCompile(`(?i)postal[^:\s]*:([\w\.-]+)`) // tag after colon
            for _, line := range lines {
                if !strings.Contains(strings.ToLower(line), "postal") {
                    continue
                }
                image := strings.Fields(line)[0]
                if m := re.FindStringSubmatch(image); len(m) == 2 {
                    tag := m[1]
                    // Normalize typical semver-looking tags, ignore "latest"
                    if strings.ToLower(tag) != "latest" {
                        recordPostalVersion(tag)
                        return
                    }
                }
            }
        }
    }

    log.Debug().Msg("Postal version not detected; skipping")
}

func recordPostalVersion(version string) {
    old := GatherVersion("postal")
    if old != "" && old == version {
        log.Debug().Msg("Postal version unchanged")
    } else if old != "" && old != version {
        log.Debug().Str("old_version", old).Str("new_version", version).Msg("Postal updated")
        CreateNews("Postal", old, version, false)
    } else {
        log.Debug().Str("version", version).Msg("Storing initial Postal version")
    }
    StoreVersion("postal", version)
}
