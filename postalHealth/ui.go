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

	// Shared DB checks run on the node holding Postal's tasks role
	if data.DBChecksNode != "" {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Postal DB Checks"))
		sb.WriteString("\n")
		sb.WriteString(common.SimpleStatusListItem("Queue / Held / Locks", "running on "+data.DBChecksNode, true))
		sb.WriteString("\n")
	}

	// Message queue status
	if data.MessageQueue.Limit > 0 {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Message Queue"))
		sb.WriteString("\n")

		sb.WriteString(common.StatusListItem(
			fmt.Sprintf("Backlog > %d min", data.MessageQueue.BacklogMinutes),
			fmt.Sprintf("%d/%d messages", data.MessageQueue.Backlog, data.MessageQueue.Limit),
			strconv.Itoa(data.MessageQueue.Limit),
			strconv.Itoa(data.MessageQueue.Backlog),
			data.MessageQueue.IsHealthy))
		sb.WriteString("\n")
		sb.WriteString(common.SimpleStatusListItem("Total Queued", strconv.Itoa(data.MessageQueue.Count), true))
		sb.WriteString("\n")
	}

	// Stale queued message locks
	if data.StaleLocks.CheckStatus {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Stale Queue Locks"))
		sb.WriteString("\n")
		sb.WriteString(common.SimpleStatusListItem(
			fmt.Sprintf("Locked > %d min", data.StaleLocks.ThresholdMinutes),
			strconv.Itoa(data.StaleLocks.Count),
			data.StaleLocks.IsHealthy))
		sb.WriteString("\n")
	}

	// Held Messages (servers with any held message only)
	var heldServers []ServerHeldMessages
	for _, server := range data.HeldMessages {
		if server.Count > 0 || server.Suppressed > 0 {
			heldServers = append(heldServers, server)
		}
	}
	if len(heldServers) > 0 {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Held Messages"))
		sb.WriteString("\n")

		// Find the longest server name for alignment
		maxNameLength := 0
		for _, server := range heldServers {
			nameLength := len(fmt.Sprintf("%s (postal-server-%d)", server.ServerName, server.ServerID))
			if nameLength > maxNameLength {
				maxNameLength = nameLength
			}
		}

		for _, server := range heldServers {
			serverName := fmt.Sprintf("%s (postal-server-%d)", server.ServerName, server.ServerID)
			// Pad the server name to align with the longest name
			paddedName := fmt.Sprintf("%-*s", maxNameLength, serverName)
			sb.WriteString(common.SimpleStatusListItem(
				paddedName,
				fmt.Sprintf("%d (suppression list: %d)", server.Count, server.Suppressed),
				server.IsHealthy,
			))
			sb.WriteString("\n")
		}
	}

	// SSL Certificate status
	if data.SSLCert.CheckStatus {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("SSL Certificate"))
		sb.WriteString("\n")
		sslStatusText := fmt.Sprintf("%s:%d - %d day(s) left", data.SSLCert.Host, data.SSLCert.Port, data.SSLCert.DaysUntilExpiry)
		sb.WriteString(common.SimpleStatusListItem(
			"SMTP SSL Cert",
			sslStatusText,
			!data.SSLCert.ExpiringSoon))
		sb.WriteString("\n")
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
