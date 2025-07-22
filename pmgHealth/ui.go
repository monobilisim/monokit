package pmgHealth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
)

// We can now use the PmgHealthData type defined in main.go

// RenderCompact renders a compact view of PMG health data
func (data *PmgHealthData) RenderCompact() string {
	var sb strings.Builder

	// Overall status section
	sb.WriteString(common.SectionTitle("PMG Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Overall Status",
		data.Status,
		data.IsHealthy))
	sb.WriteString("\n")

	// Service status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("PMG Services"))
	sb.WriteString("\n")

	for service, isRunning := range data.Services {
		sb.WriteString(common.SimpleStatusListItem(
			service,
			getStatusText(isRunning, "running"),
			isRunning))
		sb.WriteString("\n")
	}

	// PostgreSQL status
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("PostgreSQL Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"PostgreSQL",
		getStatusText(data.PostgresRunning, "running"),
		data.PostgresRunning))
	sb.WriteString("\n")

	// Mail queue status
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Mail Queue"))
	sb.WriteString("\n")

	var queueStatusText string
	if data.QueueStatus.IsHealthy {
		queueStatusText = "within limit"
	} else {
		queueStatusText = "exceeds limit"
	}

	sb.WriteString(common.StatusListItem(
		"Queued Messages",
		queueStatusText,
		strconv.Itoa(data.QueueStatus.Limit),
		strconv.Itoa(data.QueueStatus.Count),
		data.QueueStatus.IsHealthy))
	sb.WriteString("\n")

	// Mail Statistics - if enabled
	if data.MailStats.Enabled {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Mail Statistics (24h)"))
		sb.WriteString("\n")

		// Current 24h stats
		currentTrafficText := fmt.Sprintf("received: %d total: %d",
			data.MailStats.Last24hReceived, data.MailStats.Last24hTotal)
		sb.WriteString(common.StatusListItem(
			"Last 24h Traffic",
			getStatsStatusText(data.MailStats.IsNormal24h),
			strconv.Itoa(data.MailStats.Last24hSent),
			currentTrafficText,
			data.MailStats.IsNormal24h))
		sb.WriteString("\n")

		// Previous 24h stats for comparison (baseline)
		prevTrafficText := fmt.Sprintf("received: %d total: %d",
			data.MailStats.Prev24hReceived, data.MailStats.Prev24hTotal)
		sb.WriteString(common.StatusListItem(
			"Baseline (prev 24h)",
			"baseline",
			strconv.Itoa(data.MailStats.Prev24hSent),
			prevTrafficText,
			true)) // Always show as "ok" since it's just baseline data
		sb.WriteString("\n")

		// Threshold information
		sb.WriteString(common.SimpleStatusListItem(
			"Daily Threshold Factor",
			strconv.FormatFloat(data.MailStats.Threshold24h, 'f', 1, 64)+"x",
			true)) // Always show as "ok" since it's just configuration
		sb.WriteString("\n")

		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Mail Statistics (1h)"))
		sb.WriteString("\n")

		// 1-hour traffic statistics
		current1hTrafficText := fmt.Sprintf("received: %d total: %d",
			data.MailStats.Last1hReceived, data.MailStats.Last1hTotal)
		sb.WriteString(common.StatusListItem(
			"Last 1h Traffic",
			getStatsStatusText(data.MailStats.IsNormal1h),
			strconv.Itoa(data.MailStats.Last1hSent),
			current1hTrafficText,
			data.MailStats.IsNormal1h))
		sb.WriteString("\n")

		// Previous 1-hour stats for comparison (baseline)
		prev1hTrafficText := fmt.Sprintf("received: %d total: %d",
			data.MailStats.Prev1hReceived, data.MailStats.Prev1hTotal)
		sb.WriteString(common.StatusListItem(
			"Baseline (prev 1h)",
			"baseline",
			strconv.Itoa(data.MailStats.Prev1hSent),
			prev1hTrafficText,
			true)) // Always show as "ok" since it's just baseline data
		sb.WriteString("\n")

		// Threshold information
		sb.WriteString(common.SimpleStatusListItem(
			"Hourly Threshold Factor",
			strconv.FormatFloat(data.MailStats.Threshold1h, 'f', 1, 64)+"x",
			true)) // Always show as "ok" since it's just configuration
		sb.WriteString("\n")
	}

	// Version status - if available
	if data.VersionStatus.CurrentVersion != "unknown" {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Version Status"))
		sb.WriteString("\n")

		versionStatus := "Up-to-date"
		if !data.VersionStatus.IsUpToDate {
			versionStatus = "Update available"
		}

		sb.WriteString(common.SimpleStatusListItem(
			"Current Version",
			data.VersionStatus.CurrentVersion,
			true)) // Version display is always "ok"
		sb.WriteString("\n")

		sb.WriteString(common.SimpleStatusListItem(
			"Update Status",
			versionStatus,
			data.VersionStatus.IsUpToDate))
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderAll renders a detailed view of all PMG health data
func (data *PmgHealthData) RenderAll() string {
	// For now, we'll just use the compact view
	// In the future, this could be extended with more detailed information
	return data.RenderCompact()
}

// Helper function to get status text
func getStatusText(isOk bool, stateType string) string {
	if isOk {
		return stateType
	}
	return "not " + stateType
}

// getStatsStatusText returns a status text for mail statistics
func getStatsStatusText(isNormal bool) string {
	if isNormal {
		return "normal"
	}
	return "elevated"
}
