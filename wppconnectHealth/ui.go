package wppconnectHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
)

// RenderWppConnectHealth renders the health status of WPPConnect sessions
func RenderWppConnectHealth(data *WppConnectHealthData) {
	var sb strings.Builder

	// Overall status section
	sb.WriteString(common.SectionTitle("WPPConnect Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Overall Status",
		getStatusText(data.Healthy, "healthy"),
		data.Healthy))
	sb.WriteString("\n")

	// Stats section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Session Statistics"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Total Sessions",
		fmt.Sprintf("%d", data.TotalCount),
		true))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Healthy Sessions",
		fmt.Sprintf("%d", data.HealthyCount),
		true))
	sb.WriteString("\n")

	// Sessions section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Session Details"))
	sb.WriteString("\n")
	for _, session := range data.Sessions {
		sb.WriteString(common.SimpleStatusListItem(
			fmt.Sprintf("%s (Session %s)", session.ContactName, session.Session),
			session.Status,
			session.Healthy))
		sb.WriteString("\n")
	}

	// Create title with version and timestamp
	title := fmt.Sprintf("WPPConnect Health Status v%s - %s", data.Version, time.Now().Format("2006-01-02 15:04:05"))

	// Display the rendered content in a box
	renderedBox := common.DisplayBox(title, sb.String())
	fmt.Println(renderedBox)
}

// RenderWppConnectHealthCLI renders the health status of WPPConnect sessions for CLI output
func RenderWppConnectHealthCLI(data *WppConnectHealthData, version string) string {
	var sb strings.Builder

	// Overall status section
	sb.WriteString(common.SectionTitle("WPPConnect Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Overall Status",
		getStatusText(data.Healthy, "healthy"),
		data.Healthy))
	sb.WriteString("\n")

	// Stats section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Session Statistics"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Total Sessions",
		fmt.Sprintf("%d", data.TotalCount),
		true))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Healthy Sessions",
		fmt.Sprintf("%d", data.HealthyCount),
		true))
	sb.WriteString("\n")

	// Sessions section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Session Details"))
	sb.WriteString("\n")
	for _, session := range data.Sessions {
		sb.WriteString(common.SimpleStatusListItem(
			fmt.Sprintf("%s (Session %s)", session.ContactName, session.Session),
			session.Status,
			session.Healthy))
		sb.WriteString("\n")
	}

	// Create title with version and timestamp
	title := fmt.Sprintf("WPPConnect Health Status v%s - %s", version, time.Now().Format("2006-01-02 15:04:05"))

	// Display the rendered content in a box
	return common.DisplayBox(title, sb.String())
}

// getStatusText returns a status text based on the health state
func getStatusText(healthy bool, status string) string {
	if healthy {
		return status
	}
	return "not " + status
}
