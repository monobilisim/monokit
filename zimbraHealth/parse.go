package zimbraHealth

import (
	"strings"

	"github.com/rs/zerolog/log"
)

// parseZmcontrolStatus parses `zmcontrol status` output into a list of
// services and a name->running map.
//
// Real top-level service entries always end with the literal token
// "Running" or "Stopped" (e.g. "\tmailbox                 Stopped").
// Sub-component continuation lines emitted under a stopped service
// (e.g. "\t\tmysql.server is not running.") never end with those
// tokens and are intentionally skipped here so they don't get parsed
// as phantom services and trigger false alarms / spurious restart
// attempts on names that don't exist in zmcontrol's vocabulary.
func parseZmcontrolStatus(statusOutput string) ([]ServiceInfo, map[string]bool) {
	var services []ServiceInfo
	statusMap := make(map[string]bool)

	for _, raw := range strings.Split(statusOutput, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "Host") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		var isRunning bool
		switch fields[len(fields)-1] {
		case "Running":
			isRunning = true
		case "Stopped":
			isRunning = false
		default:
			// Continuation/sub-component line (e.g. "mysql.server is
			// not running.") or an unknown status keyword. Skip it
			// instead of inventing a phantom service from the leading
			// text.
			log.Debug().Str("line", line).Msg("zmcontrol: skipping non-status line")
			continue
		}

		serviceName := strings.Join(fields[:len(fields)-1], " ")
		serviceName = strings.TrimPrefix(serviceName, "service ")
		serviceName = strings.TrimPrefix(serviceName, "carbonio-")

		services = append(services, ServiceInfo{Name: serviceName, Running: isRunning})
		statusMap[serviceName] = isRunning
	}

	return services, statusMap
}
