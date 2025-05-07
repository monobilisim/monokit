package postalHealth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
)

// RenderCompact renders a compact view of Postal health data
func (data *PostalHealthData) RenderCompact() string {
	var sb strings.Builder

	// Overall status section
	sb.WriteString(common.SectionTitle("Postal Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Overall Status",
		data.Status,
		data.IsHealthy))
	sb.WriteString("\n")

	// Service status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Postal Services"))
	sb.WriteString("\n")

	for service, isRunning := range data.Services {
		sb.WriteString(common.SimpleStatusListItem(
			service,
			getStatusText(isRunning, "running"),
			isRunning))
		sb.WriteString("\n")
	}

	// Container status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Docker Containers"))
	sb.WriteString("\n")

	for _, container := range data.Containers {
		sb.WriteString(common.SimpleStatusListItem(
			container.Name,
			getStatusText(container.IsRunning, container.State),
			container.IsRunning))
		sb.WriteString("\n")
	}

	// MySQL status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("MySQL Status"))
	sb.WriteString("\n")

	for db, isConnected := range data.MySQLStatus {
		sb.WriteString(common.SimpleStatusListItem(
			db,
			getStatusText(isConnected, "connected"),
			isConnected))
		sb.WriteString("\n")
	}

	// Service health checks
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Service Health Checks"))
	sb.WriteString("\n")

	for service, isHealthy := range data.ServiceStatus {
		sb.WriteString(common.SimpleStatusListItem(
			service,
			getStatusText(isHealthy, "healthy"),
			isHealthy))
		sb.WriteString("\n")
	}

	// Message queue status
	if data.MessageQueue.Limit > 0 {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Message Queue"))
		sb.WriteString("\n")

		queueStatusText := fmt.Sprintf("%d/%d messages", data.MessageQueue.Count, data.MessageQueue.Limit)
		sb.WriteString(common.StatusListItem(
			"Queued Messages",
			queueStatusText,
			strconv.Itoa(data.MessageQueue.Limit),
			strconv.Itoa(data.MessageQueue.Count),
			data.MessageQueue.IsHealthy))
		sb.WriteString("\n")
	}

	// Held Messages
	if len(data.HeldMessages) > 0 {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Held Messages"))
		sb.WriteString("\n")

		// Find the longest server name for alignment
		maxNameLength := 0
		for _, server := range data.HeldMessages {
			nameLength := len(fmt.Sprintf("%s (postal-server-%d)", server.ServerName, server.ServerID))
			if nameLength > maxNameLength {
				maxNameLength = nameLength
			}
		}

		for _, server := range data.HeldMessages {
			serverName := fmt.Sprintf("%s (postal-server-%d)", server.ServerName, server.ServerID)
			// Pad the server name to align with the longest name
			paddedName := fmt.Sprintf("%-*s", maxNameLength, serverName)
			sb.WriteString(common.SimpleStatusListItem(
				paddedName,
				fmt.Sprintf("%d", server.Count),
				server.IsHealthy,
			))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// RenderAll renders a detailed view of all Postal health data
func (data *PostalHealthData) RenderAll() string {
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
