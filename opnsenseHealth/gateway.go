package opnsenseHealth

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

const gatewayStatusScript = "/usr/local/opnsense/scripts/routes/gateway_status.php"

type gatewayRaw struct {
	Name       flexString `json:"name"`
	Address    flexString `json:"address"`
	Status     flexString `json:"status"`
	Loss       flexString `json:"loss"`
	Delay      flexString `json:"delay"`
	StdDev     flexString `json:"stddev"`
	Monitor    flexString `json:"monitor"`
	StatusText flexString `json:"status_translated"`
}

// collectGatewayHealth reports per-gateway health from gateway_status.php. It
// returns nil when the script is not available.
func collectGatewayHealth() *GatewayStatus {
	if _, err := os.Stat(gatewayStatusScript); err != nil {
		log.Debug().Msg(gatewayStatusScript + " not found, skipping gateway check")
		return nil
	}

	php := resolveBinary("php", "/usr/local/bin/php")
	if php == "" {
		log.Debug().Msg("php not found, skipping gateway check")
		return nil
	}

	status := &GatewayStatus{}

	out, err := runCmd(php, gatewayStatusScript)
	if err != nil {
		status.Error = err.Error()
		common.AlarmCheckDown("opnsense_gateway", "Could not read gateway status: "+err.Error(), false, "", "")
		return status
	}

	// PHP encodes an empty result as [] rather than {}.
	payload := trimToJSON(out)
	if trimmed := strings.TrimSpace(string(payload)); trimmed == "[]" || trimmed == "{}" || trimmed == "" {
		common.AlarmCheckUp("opnsense_gateway", "Gateway status is readable again.", false)
		return status
	}

	entries, err := decodeJSONEntries(payload, []string{"name"}, "gateway_")
	if err != nil {
		status.Error = err.Error()
		common.AlarmCheckDown("opnsense_gateway", "Could not parse gateway status output: "+err.Error(), false, "", "")
		return status
	}
	common.AlarmCheckUp("opnsense_gateway", "Gateway status is readable again.", false)

	for _, entry := range entries {
		var raw gatewayRaw
		if err := json.Unmarshal(entry.raw, &raw); err != nil {
			log.Warn().Err(err).Str("gateway", entry.key).Msg("Could not parse gateway entry, skipping")
			continue
		}

		gw := GatewayInfo{
			Name:       entry.key,
			Address:    string(raw.Address),
			Monitor:    string(raw.Monitor),
			Status:     strings.ToLower(strings.TrimSpace(string(raw.Status))),
			StatusText: string(raw.StatusText),
			Loss:       string(raw.Loss),
			Delay:      string(raw.Delay),
			StdDev:     string(raw.StdDev),
		}
		if name := strings.TrimSpace(string(raw.Name)); name != "" {
			gw.Name = name
		}
		gw.LossPercent, gw.HasLoss = parseLossPercent(gw.Loss)

		evaluateGateway(&gw)
		status.Gateways = append(status.Gateways, gw)
	}

	return status
}

// classifyGateway decides whether a gateway counts as healthy. A gateway
// forced down administratively, or one still gathering its first samples, is
// not a fault and is marked as skipped instead.
func classifyGateway(gw *GatewayInfo, lossLimit float64) {
	// Forced down is an explicit admin action, so any existing alarm is stale
	// and should be released.
	if gw.Status == "force_down" {
		gw.Skipped = true
		gw.Healthy = true
		gw.Reason = "administratively forced down"
		return
	}

	// Pending means dpinger has no samples yet — on first run, but also for a
	// few seconds after any gateway or interface change. It says nothing about
	// health, so the current alarm state must be left exactly as it is.
	if strings.EqualFold(gw.StatusText, "Pending") {
		gw.Skipped = true
		gw.Pending = true
		gw.Healthy = true
		gw.Reason = "waiting for monitoring data"
		return
	}

	switch {
	case strings.Contains(gw.Status, "down") || strings.EqualFold(gw.StatusText, "Offline"):
		gw.Healthy = false
		gw.Reason = "offline"
	case gw.HasLoss && gw.LossPercent >= lossLimit:
		gw.Healthy = false
		gw.Reason = fmt.Sprintf("packet loss %.1f%% is at or above the %.1f%% limit", gw.LossPercent, lossLimit)
	default:
		gw.Healthy = true
	}
}

func evaluateGateway(gw *GatewayInfo) {
	classifyGateway(gw, OpnsenseHealthConfig.Gateway.LossLimit)

	alarmName := "opnsense_gateway_" + alarmSuffix(gw.Name)

	// Leave a pending gateway's alarm state untouched. Calling AlarmCheckUp here
	// would emit a false recovery for a gateway that is still down.
	if gw.Pending {
		return
	}
	if gw.Skipped {
		common.AlarmCheckUp(alarmName, fmt.Sprintf("Gateway %s is no longer being alarmed (%s).", gw.Name, gw.Reason), false)
		return
	}

	detail := fmt.Sprintf("Address: %s\nMonitor: %s\nLoss: %s\nDelay: %s\nStatus: %s",
		orDash(gw.Address), orDash(gw.Monitor), orDash(gw.Loss), orDash(gw.Delay), orDash(gw.StatusText))

	if gw.Healthy {
		common.AlarmCheckUp(alarmName, fmt.Sprintf("Gateway %s is healthy.\n%s", gw.Name, detail), false)
		return
	}
	common.AlarmCheckDown(alarmName, fmt.Sprintf("Gateway %s is unhealthy (%s).\n%s", gw.Name, gw.Reason, detail), false, "", "")
}

// parseLossPercent reads gateway_status.php's loss field, which arrives as
// "0.0 %". The second return value is false when the gateway has no monitor
// and therefore no loss figure to compare.
func parseLossPercent(loss string) (float64, bool) {
	cleaned := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(loss), "%"))
	if cleaned == "" || cleaned == "~" {
		return 0, false
	}
	v, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
