package pmgHealth

import (
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
