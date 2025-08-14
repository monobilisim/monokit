package common

import (
    "os/exec"
    "strings"

    "github.com/rs/zerolog/log"
)

// DockerCheck detects Docker client and server (engine) versions using
// `docker version`, compares with stored values, creates news on change,
// and persists current versions. It gracefully handles cases where the
// Docker daemon is not reachable by reporting only the client version.
func DockerCheck() {
    // Ensure docker CLI is available
    if _, err := exec.LookPath("docker"); err != nil {
        log.Debug().Msg("Docker CLI not found, skipping version check")
        return
    }

    out, err := exec.Command("docker", "version").CombinedOutput()
    if err != nil {
        // Even if daemon isn't running, the client section is usually printed.
        // We'll still attempt to parse whatever we received.
        log.Debug().Err(err).Msg("docker version returned non-zero exit; attempting to parse partial output")
    }

    raw := string(out)
    lines := strings.Split(raw, "\n")

    var section string // "client" | "server" | ""
    var currentComponent string

    var clientVersion string
    var serverVersion string

    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        if trimmed == "" {
            continue
        }

        lower := strings.ToLower(trimmed)

        switch {
        case strings.HasPrefix(lower, "client:"):
            section = "client"
            currentComponent = ""
            continue
        case strings.HasPrefix(lower, "server:"):
            section = "server"
            currentComponent = ""
            continue
        }

        // Track sub-component under Server section
        if section == "server" {
            // Lines like: "Engine:", "containerd:", "runc:", "docker-init:"
            if strings.HasSuffix(trimmed, ":") {
                name := strings.TrimSuffix(trimmed, ":")
                currentComponent = strings.ToLower(strings.TrimSpace(name))
                continue
            }
        }

        // Version extraction
        if strings.HasPrefix(lower, "version:") {
            // Expected format: "Version:          28.3.2"
            parts := strings.SplitN(trimmed, ":", 2)
            if len(parts) == 2 {
                version := strings.TrimSpace(parts[1])
                // Collapse multiple spaces
                if fields := strings.Fields(version); len(fields) > 0 {
                    version = fields[0]
                }

                if section == "client" && clientVersion == "" {
                    clientVersion = version
                } else if section == "server" && serverVersion == "" {
                    // Prefer Engine version when available
                    if currentComponent == "engine" || currentComponent == "" {
                        serverVersion = version
                    }
                }
            }
            continue
        }
    }

    // Report and persist client version
    if clientVersion != "" {
        log.Debug().Str("version", clientVersion).Msg("Detected Docker client version")
        oldClient := GatherVersion("docker-client")
        if oldClient != "" && oldClient == clientVersion {
            log.Debug().Msg("Docker client version unchanged")
        } else if oldClient != "" && oldClient != clientVersion {
            log.Debug().Str("old_version", oldClient).Str("new_version", clientVersion).Msg("Docker client updated")
            CreateNews("Docker Client", oldClient, clientVersion, false)
        } else {
            log.Debug().Str("version", clientVersion).Msg("Storing initial Docker client version")
        }
        StoreVersion("docker-client", clientVersion)
    } else {
        log.Debug().Msg("Docker client version not detected in output")
    }

    // Report and persist server (engine) version if present
    if serverVersion != "" {
        log.Debug().Str("version", serverVersion).Msg("Detected Docker engine version")
        oldServer := GatherVersion("docker-server")
        if oldServer != "" && oldServer == serverVersion {
            log.Debug().Msg("Docker engine version unchanged")
        } else if oldServer != "" && oldServer != serverVersion {
            log.Debug().Str("old_version", oldServer).Str("new_version", serverVersion).Msg("Docker engine updated")
            CreateNews("Docker Engine", oldServer, serverVersion, false)
        } else {
            log.Debug().Str("version", serverVersion).Msg("Storing initial Docker engine version")
        }
        StoreVersion("docker-server", serverVersion)
    } else {
        // Likely the daemon is not available; not an error.
        log.Debug().Msg("Docker engine version not detected (daemon may be unavailable)")
    }
}
