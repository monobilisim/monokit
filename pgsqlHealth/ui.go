//go:build linux

package pgsqlHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/monobilisim/monokit/common"
)

// PostgreSQLHealthData represents PostgreSQL health information
type PostgreSQLHealthData struct {
	ConnectionInfo  ConnectionInfo
	UptimeInfo      UptimeInfo
	ConnectionsInfo ConnectionsInfo
	QueriesInfo     QueriesInfo
	ClusterInfo     ClusterInfo
	WalGInfo        WalGInfo
	PMM             PMMInfo
	Version         string
	VersionInfo     VersionInfo
}

// ConnectionInfo contains basic PostgreSQL connection information
type ConnectionInfo struct {
	Connected bool
	Error     string
}

// UptimeInfo contains PostgreSQL uptime information
type UptimeInfo struct {
	Uptime     string
	StartTime  string
	ActiveTime string
}

// ConnectionsInfo contains information about PostgreSQL connections
type ConnectionsInfo struct {
	Active    int
	Limit     int
	Exceeded  bool
	UsageRate float64
}

// QueriesInfo contains information about running queries
type QueriesInfo struct {
	RunningQueries []QueryInfo
	LongRunning    int
	QueryLimit     int
}

// QueryInfo represents a single PostgreSQL query
type QueryInfo struct {
	PID      int
	Username string
	Database string
	Duration string
	State    string
	Query    string
}

// ClusterInfo contains information about PostgreSQL cluster status
type ClusterInfo struct {
	Enabled      bool
	Role         string
	Status       string
	Nodes        []NodeInfo
	IsReplicated bool
	IsHealthy    bool
}

// NodeInfo represents a PostgreSQL cluster node
type NodeInfo struct {
	Name    string
	Role    string
	State   string
	Host    string
	Port    int64
	Healthy bool
}

// WalGInfo contains WAL-G backup information
type WalGInfo struct {
	Enabled     bool
	LastBackup  string
	Status      string
	BackupCount int
	Healthy     bool
}

// PMMInfo contains Percona Monitoring and Management information
type PMMInfo struct {
	Enabled bool
	Status  string
	Active  bool
}

// VersionInfo contains PostgreSQL version information
type VersionInfo struct {
	Version       string
	NeedsUpdate   bool
	UpdateMessage string
}

// NewPostgreSQLHealthData creates a new PostgreSQLHealthData
func NewPostgreSQLHealthData() *PostgreSQLHealthData {
	return &PostgreSQLHealthData{
		ClusterInfo: ClusterInfo{
			Nodes: []NodeInfo{},
		},
		QueriesInfo: QueriesInfo{
			RunningQueries: []QueryInfo{},
		},
	}
}

// RenderCompact renders a compact view of PostgreSQL health data
func (p *PostgreSQLHealthData) RenderCompact() string {
	var sb strings.Builder

	// ====== PostgreSQL Access Category ======
	sb.WriteString(common.SectionTitle("PostgreSQL Access"))
	sb.WriteString("\n")

	// Connection status
	isConnected := p.ConnectionInfo.Connected
	connectionStatus := "Connected"
	if !isConnected {
		connectionStatus = "Disconnected"
	}

	sb.WriteString(common.SimpleStatusListItem(
		"Connection",
		connectionStatus,
		isConnected))
	sb.WriteString("\n")

	// Uptime (if connected)
	if isConnected && p.UptimeInfo.Uptime != "" {
		// Create a basic item display similar to SimpleStatusListItem but without the status coloring
		contentStyle := lipgloss.NewStyle().
			Align(lipgloss.Left).
			PaddingLeft(8)

		itemStyle := lipgloss.NewStyle().
			Foreground(common.NormalTextColor)

		line := fmt.Sprintf("•  %-20s is %s", "Uptime", p.UptimeInfo.Uptime)
		sb.WriteString(contentStyle.Render(itemStyle.Render(line)))
		sb.WriteString("\n")
	}

	// ====== Connections Category ======
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Active Connections"))
	sb.WriteString("\n")

	// Active Connections - show percentage of max connections
	if isConnected {
		isConnectionsOK := !p.ConnectionsInfo.Exceeded
		connectionsStatusPrefix := "within limit"
		if !p.ConnectionsInfo.Exceeded {
			connectionsStatusPrefix = "within limit"
		} else {
			connectionsStatusPrefix = "exceeds limit"
		}

		sb.WriteString(common.StatusListItem(
			"Active Connections",
			connectionsStatusPrefix,
			fmt.Sprintf("%d", p.ConnectionsInfo.Limit),
			fmt.Sprintf("%d", p.ConnectionsInfo.Active),
			isConnectionsOK))
		sb.WriteString("\n")
	}

	// ====== Version Check Category ======
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Version Check"))
	sb.WriteString("\n")

	// Version information
	if p.VersionInfo.Version != "" {
		sb.WriteString(common.SimpleStatusListItem(
			"PostgreSQL Version",
			p.VersionInfo.Version,
			true)) // Version itself is just informational, so always "true"
		sb.WriteString("\n")

		updateStatus := "Up-to-date"
		if p.VersionInfo.NeedsUpdate {
			updateStatus = p.VersionInfo.UpdateMessage
		}

		sb.WriteString(common.SimpleStatusListItem(
			"Updates",
			updateStatus,
			!p.VersionInfo.NeedsUpdate))
		sb.WriteString("\n")
	}

	// ====== Running Queries Category ======
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Running Queries"))
	sb.WriteString("\n")

	// Long running queries (if any)
	if isConnected {
		// Use the stored QueryLimit
		queryCount := len(p.QueriesInfo.RunningQueries)
		queryLimit := p.QueriesInfo.QueryLimit

		isQueriesOK := queryCount <= queryLimit
		statusPrefix := "within limit"
		if !isQueriesOK {
			statusPrefix = "exceeds limit"
		}

		sb.WriteString(common.StatusListItem(
			"Running Queries",
			statusPrefix,
			fmt.Sprintf("%d", queryLimit),
			fmt.Sprintf("%d", queryCount),
			isQueriesOK))
		sb.WriteString("\n")

		longRunningStatus := "None"
		isLongRunningOK := p.QueriesInfo.LongRunning == 0

		if !isLongRunningOK {
			longRunningStatus = fmt.Sprintf("%d Found", p.QueriesInfo.LongRunning)
		}

		sb.WriteString(common.SimpleStatusListItem(
			"Long Running Queries",
			longRunningStatus,
			isLongRunningOK))
	}

	// ====== Consul Status Section (if enabled) ======
	if DbHealthConfig.Postgres.Consul.Enabled {
		sb.WriteString("\n\n")
		sb.WriteString(common.SectionTitle("Consul Status"))
		sb.WriteString("\n")

		// Check if consul ports are open
		consulPortsOpen, err := checkConsulPorts()
		if err != nil {
			sb.WriteString(common.SimpleStatusListItem(
				"Consul Ports",
				"Error",
				false))
		} else if !consulPortsOpen {
			sb.WriteString(common.SimpleStatusListItem(
				"Consul Ports",
				"Not Open",
				false))
		} else {
			sb.WriteString(common.SimpleStatusListItem(
				"Consul Ports",
				"Open",
				true))
		}
		sb.WriteString("\n")

		// Check if consul is running
		consulServiceRunning, err := checkConsulService()
		if err != nil {
			sb.WriteString(common.SimpleStatusListItem(
				"Consul Status",
				"Error",
				false))
		} else if !consulServiceRunning {
			sb.WriteString(common.SimpleStatusListItem(
				"Consul Status",
				"Not Running",
				false))
		} else {
			sb.WriteString(common.SimpleStatusListItem(
				"Consul Status",
				"Running",
				true))
		}
		sb.WriteString("\n")

		// Get consul members
		members, err := getConsulMembers(consulURL)
		if err != nil {
			sb.WriteString(common.SimpleStatusListItem(
				"Consul Members",
				"Error",
				false))
		} else {
			// Create a basic item display for each member
			contentStyle := lipgloss.NewStyle().
				Align(lipgloss.Left).
				PaddingLeft(8)

			itemStyle := lipgloss.NewStyle().
				Foreground(common.NormalTextColor)

			statusStyle := lipgloss.NewStyle().Foreground(common.SuccessColor)

			for _, member := range members {
				status := strings.ToLower(member.Status)
				statusStyle = lipgloss.NewStyle().Foreground(common.SuccessColor) // Default to green
				if status != "passing" {
					statusStyle = lipgloss.NewStyle().Foreground(common.ErrorColor)
				}
				line := fmt.Sprintf("•  %-20s is %s", member.Node, statusStyle.Render(status))
				sb.WriteString(contentStyle.Render(itemStyle.Render(line)))
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")

		// Get consul catalog
		catalog, err := getConsulCatalog(consulURL)
		fmt.Println("Catalog: ", catalog)
		if err != nil {
			sb.WriteString(common.SimpleStatusListItem(
				"Consul Catalog",
				"Error",
				false))
			fmt.Println("Error: ", err)
		} else {
			// Create a basic item display for each service
			contentStyle := lipgloss.NewStyle().
				Align(lipgloss.Left).
				PaddingLeft(8)

			itemStyle := lipgloss.NewStyle().
				Foreground(common.NormalTextColor)

			statusStyle := lipgloss.NewStyle().Foreground(common.SuccessColor)

			for _, service := range catalog.Services {
				status := "running"
				if service.State != "running" {
					statusStyle = lipgloss.NewStyle().Foreground(common.ErrorColor)
					status = service.State
				}
				line := fmt.Sprintf("•  %-20s is %s", service.Name, statusStyle.Render(status))
				sb.WriteString(contentStyle.Render(itemStyle.Render(line)))
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	// ====== HAProxy Status Section (if enabled) ======
	if DbHealthConfig.Postgres.Haproxy.Enabled {
		sb.WriteString("\n\n")
		sb.WriteString(common.SectionTitle("HAProxy Status"))
		sb.WriteString("\n")

		haproxyServiceInstalled, haproxyServiceRunning := checkHAProxyService()

		statusTextService := "Not Installed"
		if haproxyServiceInstalled {
			statusTextService = "Installed"
		}
		sb.WriteString(common.SimpleStatusListItem(
			"HAProxy Service",
			statusTextService,
			haproxyServiceInstalled))
		sb.WriteString("\n")

		if haproxyServiceInstalled {
			statusTextState := "Not Running"
			if haproxyServiceRunning {
				statusTextState = "Running"
			}
			sb.WriteString(common.SimpleStatusListItem(
				"HAProxy State",
				statusTextState,
				haproxyServiceRunning))
			sb.WriteString("\n")

			if haproxyServiceRunning {
				bindPortsAreOpen := checkHAProxyBindPortsOpen()
				statusTextPorts := "No bind ports accessible"
				if bindPortsAreOpen {
					statusTextPorts = "At least one bind port accessible"
				}
				sb.WriteString(common.SimpleStatusListItem(
					"HAProxy Bind Ports",
					statusTextPorts,
					bindPortsAreOpen))
				sb.WriteString("\n")

				// Check specific stats port 8404
				statsPort8404Open := checkSpecificHAProxyPort(8404, "stats_8404")
				statusTextStatsPort := "Not Accessible"
				if statsPort8404Open {
					statusTextStatsPort = "Accessible"
				}
				sb.WriteString(common.SimpleStatusListItem(
					"HAProxy Stats (8404)",
					statusTextStatsPort,
					statsPort8404Open))
				sb.WriteString("\n")

			} else {
				sb.WriteString(common.SimpleStatusListItem(
					"HAProxy Bind Ports",
					"Not checked (service not running)",
					true)) // true to avoid red color for an expected state
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString(common.SimpleStatusListItem(
				"HAProxy State",
				"N/A (service not installed)",
				true))
			sb.WriteString("\n")
			sb.WriteString(common.SimpleStatusListItem(
				"HAProxy Bind Ports",
				"N/A (service not installed)",
				true))
			sb.WriteString("\n")
			// Also show stats port as N/A if service is not installed
			sb.WriteString(common.SimpleStatusListItem(
				"HAProxy Stats (8404)",
				"N/A (service not installed)",
				true))
			sb.WriteString("\n")
		}
	}

	// ====== Cluster Status Section (if enabled) ======
	if p.ClusterInfo.Enabled {
		sb.WriteString("\n\n")
		sb.WriteString(common.SectionTitle("Cluster Status"))
		sb.WriteString("\n")

		// Node role
		roleStatus := p.ClusterInfo.Role
		isRolePrimary := strings.ToLower(roleStatus) == "master" || strings.ToLower(roleStatus) == "primary"

		sb.WriteString(common.SimpleStatusListItem(
			"Role",
			roleStatus,
			isRolePrimary))
		sb.WriteString("\n")

		// Cluster health
		isClusterHealthy := p.ClusterInfo.IsHealthy
		clusterHealth := "Healthy"
		if !isClusterHealthy {
			clusterHealth = "Unhealthy"
		}

		sb.WriteString(common.SimpleStatusListItem(
			"Cluster Health",
			clusterHealth,
			isClusterHealthy))
		sb.WriteString("\n")

		// Overall replication status
		isReplicated := p.ClusterInfo.IsReplicated
		replicationStatus := "Replicating"
		if !isReplicated {
			replicationStatus = "Not Replicating"
		}

		sb.WriteString(common.SimpleStatusListItem(
			"Replication",
			replicationStatus,
			isReplicated))

		// Add cluster size display
		clusterSize := len(p.ClusterInfo.Nodes)
		sb.WriteString("\n")

		// Create custom status item with yellow (warning) color for cluster size > 0
		contentStyle := lipgloss.NewStyle().
			Align(lipgloss.Left).
			PaddingLeft(8)

		itemStyle := lipgloss.NewStyle().
			Foreground(common.NormalTextColor)

		statusStyle := lipgloss.NewStyle().Foreground(common.SuccessColor)
		if clusterSize > 0 {
			statusStyle = lipgloss.NewStyle().Foreground(common.WarningColor)
		}

		sizeStatus := fmt.Sprintf("%d Nodes", clusterSize)
		line := fmt.Sprintf("•  %-20s is %s", "Cluster Size", statusStyle.Render(sizeStatus))
		sb.WriteString(contentStyle.Render(itemStyle.Render(line)))

		// List nodes if available
		if len(p.ClusterInfo.Nodes) > 0 {
			sb.WriteString("\n\n")
			sb.WriteString(common.SectionTitle("Nodes"))
			sb.WriteString("\n")

			for i, node := range p.ClusterInfo.Nodes {
				nodeName := node.Name
				if nodeName == "" {
					nodeName = fmt.Sprintf("Node %d", i+1)
				}

				sb.WriteString(common.SimpleStatusListItem(
					nodeName,
					node.State,
					node.Healthy))

				if i < len(p.ClusterInfo.Nodes)-1 {
					sb.WriteString("\n")
				}
			}
		}
	}

	// ====== WAL-G Backup Section (if enabled) ======
	if p.WalGInfo.Enabled {
		sb.WriteString("\n\n")
		sb.WriteString(common.SectionTitle("Backups"))
		sb.WriteString("\n")

		isWalGHealthy := p.WalGInfo.Healthy
		lastBackup := p.WalGInfo.LastBackup
		if lastBackup == "" {
			lastBackup = "None"
		}

		sb.WriteString(common.SimpleStatusListItem(
			"WAL-G Backup",
			p.WalGInfo.Status,
			isWalGHealthy))
		sb.WriteString("\n")

		// Create a basic item display similar to SimpleStatusListItem but without the status coloring
		contentStyle := lipgloss.NewStyle().
			Align(lipgloss.Left).
			PaddingLeft(8)

		itemStyle := lipgloss.NewStyle().
			Foreground(common.NormalTextColor)

		line := fmt.Sprintf("•  %-20s: %s", "Last Backup", lastBackup)
		sb.WriteString(contentStyle.Render(itemStyle.Render(line)))
	}

	// ====== PMM Status Section (if enabled) ======
	if p.PMM.Enabled {
		sb.WriteString("\n\n")
		sb.WriteString(common.SectionTitle("Monitoring"))
		sb.WriteString("\n")

		isPmmActive := p.PMM.Active
		sb.WriteString(common.SimpleStatusListItem(
			"PMM Status",
			p.PMM.Status,
			isPmmActive))
	}

	return sb.String()
}

// RenderAll renders all PostgreSQL health data as a single string
func (p *PostgreSQLHealthData) RenderAll() string {
	// Use title and content with the common.DisplayBox function
	title := "monokit pgsqlHealth"
	if p.Version != "" {
		title += " v" + p.Version
	}

	// Add timestamp similar to the old UI
	title += " - " + time.Now().Format("2006-01-02 15:04:05")

	content := p.RenderCompact()
	return common.DisplayBox(title, content)
}
