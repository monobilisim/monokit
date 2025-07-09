//go:build !linux

package osHealth

import (
	"github.com/rs/zerolog/log"
)

// SystemdLogs is a stub implementation for non-Linux platforms
func SystemdLogs() {
	log.Debug().Msg("Systemd logs collection not available on non-Linux platforms")
}
