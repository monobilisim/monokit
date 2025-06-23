package pritunlHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
)

// PritunlHealthData represents the health data for Pritunl VPN
type PritunlHealthData struct {
	Version       string
	LastChecked   string
	Servers       []ServerInfo
	Users         []UserInfo
	Organizations []OrganizationInfo
	IsHealthy     bool
}

// ServerInfo represents information about a Pritunl server
type ServerInfo struct {
	Name      string
	Status    string
	IsHealthy bool
}

// UserInfo represents information about a Pritunl user
type UserInfo struct {
	Name             string
	Organization     string
	Status           string
	ConnectedClients []ClientInfo
	IsHealthy        bool
}

// ClientInfo represents information about a connected client
type ClientInfo struct {
	IPAddress string
}

// OrganizationInfo represents information about a Pritunl organization
type OrganizationInfo struct {
	Name     string
	IsActive bool
}

// NewPritunlHealthData creates a new PritunlHealthData
func NewPritunlHealthData() *PritunlHealthData {
	return &PritunlHealthData{
		Servers:       []ServerInfo{},
		Users:         []UserInfo{},
		Organizations: []OrganizationInfo{},
		LastChecked:   time.Now().Format("2006-01-02 15:04:05"),
	}
}

// RenderCompact renders a compact view of Pritunl health data
func (p *PritunlHealthData) RenderCompact() string {
	var sb strings.Builder

	// Overall status section
	sb.WriteString(common.SectionTitle("Pritunl Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Overall Status",
		getStatusText(p.IsHealthy, "healthy"),
		p.IsHealthy))
	sb.WriteString("\n")

	// Server status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Server Status"))
	sb.WriteString("\n")
	for _, server := range p.Servers {
		sb.WriteString(common.SimpleStatusListItem(
			server.Name,
			server.Status,
			server.IsHealthy))
		sb.WriteString("\n")
	}

	// User status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("User Status"))
	sb.WriteString("\n")
	for _, user := range p.Users {
		// Create a status string that includes client count if any
		status := user.Status
		if len(user.ConnectedClients) > 0 {
			status = fmt.Sprintf("%s (%d clients)", status, len(user.ConnectedClients))
		}

		sb.WriteString(common.SimpleStatusListItem(
			fmt.Sprintf("%s (%s)", user.Name, user.Organization),
			status,
			user.IsHealthy))
		sb.WriteString("\n")

		// Show connected clients if any
		if len(user.ConnectedClients) > 0 {
			for _, client := range user.ConnectedClients {
				sb.WriteString(common.SimpleStatusListItem(
					"  └─ Client",
					client.IPAddress,
					true)) // Client connection is always considered healthy
				sb.WriteString("\n")
			}
		}
	}

	// Organization status section
	if len(p.Organizations) > 0 {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Organizations"))
		sb.WriteString("\n")
		for _, org := range p.Organizations {
			sb.WriteString(common.SimpleStatusListItem(
				org.Name,
				getStatusText(org.IsActive, "active"),
				org.IsActive))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// RenderAll renders all Pritunl health data as a single string
func (p *PritunlHealthData) RenderAll() string {
	// Use title and content with the common.DisplayBox function
	title := fmt.Sprintf("monokit pritunlHealth v%s - %s", p.Version, p.LastChecked)
	content := p.RenderCompact()
	return common.DisplayBox(title, content)
}

// RenderPritunlHealthCLI renders pritunl health data for CLI display (used by plugin)
func RenderPritunlHealthCLI(healthData *PritunlHealthData, monokitVersion string) string {
	// Create title with version information
	title := fmt.Sprintf("monokit pritunlHealth v%s - %s", healthData.Version, healthData.LastChecked)
	content := healthData.RenderCompact()
	return common.DisplayBox(title, content)
}

// getStatusText returns a status text based on the health state
func getStatusText(healthy bool, status string) string {
	if healthy {
		return status
	}
	return "not " + status
}
