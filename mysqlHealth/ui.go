package mysqlHealth

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/monobilisim/monokit/common"
)

// MySQLHealthData represents MySQL health information
type MySQLHealthData struct {
	ConnectionInfo  ConnectionInfo
	ProcessInfo     ProcessInfo
	CertWaitingInfo CertificationWaitingInfo
	ClusterInfo     ClusterInfo
	PMM             PMMInfo
	Version         string
}

// ConnectionInfo contains basic MySQL connection information
type ConnectionInfo struct {
	Connected  bool
	ServerTime string
	Error      string
}

// ProcessInfo contains process count information
type ProcessInfo struct {
	Total          int
	Limit          int
	Exceeded       bool
	ProcessPercent float64
}

// CertificationWaitingInfo contains information about certification waiting processes
type CertificationWaitingInfo struct {
	Count    int
	Limit    int
	Exceeded bool
}

// ClusterInfo contains information about MySQL cluster status
type ClusterInfo struct {
	Enabled           bool
	InaccessibleCount int
	ClusterSize       int
	Status            string
	Nodes             []NodeInfo
	Synced            bool
}

// NodeInfo represents a MySQL cluster node
type NodeInfo struct {
	Name   string
	Status string
	Active bool
}

// PMMInfo contains Percona Monitoring and Management information
type PMMInfo struct {
	Enabled bool
	Status  string
	Active  bool
}

// NewMySQLHealthData creates a new MySQLHealthData
func NewMySQLHealthData() *MySQLHealthData {
	return &MySQLHealthData{
		ClusterInfo: ClusterInfo{
			Nodes: []NodeInfo{},
		},
	}
}

// RenderCompact renders a compact view of MySQL health data
func (m *MySQLHealthData) RenderCompact() string {
	var sb strings.Builder

	// Main sections in a single column layout like osHealth
	sb.WriteString(common.SectionTitle("MySQL Status"))
	sb.WriteString("\n")

	// Connection status
	isConnected := m.ConnectionInfo.Connected
	connectionStatus := "Connected"
	if !isConnected {
		connectionStatus = "Disconnected"
	}

	sb.WriteString(common.SimpleStatusListItem(
		"Connection",
		connectionStatus,
		isConnected))
	sb.WriteString("\n")

	// Process Count - Fix display for exceeds/within limit logic
	isProcessOK := !m.ProcessInfo.Exceeded
	processStatusPrefix := "within limit"
	if !isProcessOK {
		processStatusPrefix = "exceeds limit"
	}

	sb.WriteString(common.StatusListItem(
		"Process Count",
		processStatusPrefix,
		fmt.Sprintf("%d", m.ProcessInfo.Limit),
		fmt.Sprintf("%d", m.ProcessInfo.Total),
		isProcessOK))
	sb.WriteString("\n")

	// Certification Waiting - Fix display for exceeds/within limit logic
	isCertOK := !m.CertWaitingInfo.Exceeded
	certStatusPrefix := "within limit"
	if !isCertOK {
		certStatusPrefix = "exceeds limit"
	}

	sb.WriteString(common.StatusListItem(
		"Waiting Processes",
		certStatusPrefix,
		fmt.Sprintf("%d", m.CertWaitingInfo.Limit),
		fmt.Sprintf("%d", m.CertWaitingInfo.Count),
		isCertOK))
	sb.WriteString("\n")

	// Cluster Status section (if enabled)
	if m.ClusterInfo.Enabled {
		sb.WriteString("\n\n")
		sb.WriteString(common.SectionTitle("Cluster Status"))
		sb.WriteString("\n")

		// Overall cluster status
		isClusterStatusOK := m.ClusterInfo.Status == "Primary" || m.ClusterInfo.Status == "Primary-Primary"
		sb.WriteString(common.SimpleStatusListItem(
			"Status",
			m.ClusterInfo.Status,
			isClusterStatusOK))
		sb.WriteString("\n")

		// Sync status
		isClusterSyncOK := m.ClusterInfo.Synced
		syncStatus := "Not Synced"
		if isClusterSyncOK {
			syncStatus = "Synced"
		}

		sb.WriteString(common.SimpleStatusListItem(
			"Sync",
			syncStatus,
			isClusterSyncOK))
		sb.WriteString("\n")

		// Inaccessible clusters - use a clearer display format
		isInaccessibleOK := m.ClusterInfo.InaccessibleCount == 0
		nodesAccessStatus := "All Accessible"
		if !isInaccessibleOK {
			nodesAccessStatus = fmt.Sprintf("%d Inaccessible", m.ClusterInfo.InaccessibleCount)
		}

		sb.WriteString(common.SimpleStatusListItem(
			"Cluster Nodes",
			nodesAccessStatus,
			isInaccessibleOK))

		// Add cluster size display with custom color logic
		clusterSize := m.ClusterInfo.ClusterSize
		sb.WriteString("\n")

		// Create custom status item with green (success) color for cluster size
		contentStyle := lipgloss.NewStyle().
			Align(lipgloss.Left).
			PaddingLeft(8)

		itemStyle := lipgloss.NewStyle().
			Foreground(common.NormalTextColor)

		statusStyle := lipgloss.NewStyle().Foreground(common.SuccessColor)

		sizeStatus := fmt.Sprintf("%d Nodes", clusterSize)
		line := fmt.Sprintf("•  %-20s is %s", "Cluster Size", statusStyle.Render(sizeStatus))
		sb.WriteString(contentStyle.Render(itemStyle.Render(line)))

	}

	// PMM Status section (if enabled)
	if m.PMM.Enabled {
		sb.WriteString("\n\n")
		sb.WriteString(common.SectionTitle("Monitoring"))
		sb.WriteString("\n")

		isPmmActive := m.PMM.Active
		sb.WriteString(common.SimpleStatusListItem(
			"PMM Status",
			m.PMM.Status,
			isPmmActive))
	}

	return sb.String()
}

// RenderAll renders all MySQL health data as a single string
func (m *MySQLHealthData) RenderAll() string {
	// Use title and content with the common.DisplayBox function
	title := "monokit mysqlHealth"
	if m.Version != "" {
		title += " v" + m.Version
	}

	content := m.RenderCompact()
	return common.DisplayBox(title, content)
}
