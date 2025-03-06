//go:build !linux

package osHealth

import (
	"github.com/monobilisim/monokit/common"
)

// SystemdLogs is a stub implementation for non-Linux platforms
func SystemdLogs() {
	common.LogDebug("Systemd logs collection not available on non-Linux platforms")
}
