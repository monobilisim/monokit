package rmqHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/monobilisim/monokit/common"
)

// RmqHealthData represents the health status of RabbitMQ
type RmqHealthData struct {
	Version     string
	LastChecked string
	Service     ServiceInfo
	Management  ManagementInfo
	Ports       PortsInfo
	API         ApiInfo
	Cluster     ClusterInfo
	Queues      QueuesInfo
	IsHealthy   bool
}

// QueuesInfo represents the aggregate health status of all queues
type QueuesInfo struct {
	FetchOK          bool
	TotalCount       int
	StoppedCount     int
	NoConsumerCount  int
	HighMessageCount int
	UnsyncedCount    int
	Items            []QueueHealthItem
}

// QueueHealthItem represents the health status of a single queue
type QueueHealthItem struct {
	Name                   string
	State                  string
	Type                   string
	Node                   string
	Messages               int
	MessagesUnacknowledged int
	Consumers              int
	Stopped                bool
	NoConsumer             bool
	HighMessages           bool
	SyncStatus             QueueSyncStatus
}

// QueueSyncStatus represents mirror/replica sync status of a queue
type QueueSyncStatus struct {
	TotalReplicas  int
	SyncedReplicas int
	IsFullySynced  bool
}

// ServiceInfo represents RabbitMQ service status
type ServiceInfo struct {
	Active bool
}

// ManagementInfo represents RabbitMQ management plugin status
type ManagementInfo struct {
	Enabled bool
	Active  bool
}

// PortsInfo represents the status of RabbitMQ ports
type PortsInfo struct {
	AMQP       bool // 5672
	Management bool // 15672
	OtherPorts map[string]bool
}

// ApiInfo represents RabbitMQ API connectivity status
type ApiInfo struct {
	Connected  bool
	Reachable  bool
	OverviewOK bool
}

// ClusterInfo represents RabbitMQ cluster information
type ClusterInfo struct {
	Nodes     []NodeInfo
	IsHealthy bool
}

// NodeInfo represents a RabbitMQ cluster node
type NodeInfo struct {
	Name      string
	IsRunning bool
}

// NewRmqHealthData creates a new RmqHealthData structure with initialized fields
func NewRmqHealthData() *RmqHealthData {
	return &RmqHealthData{
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
		Ports: PortsInfo{
			OtherPorts: make(map[string]bool),
		},
		Cluster: ClusterInfo{
			Nodes:     []NodeInfo{},
			IsHealthy: true,
		},
		Queues: QueuesInfo{
			Items: []QueueHealthItem{},
		},
		IsHealthy: true, // Start optimistically
	}
}

// RenderCompact renders a compact view of the RabbitMQ health data
func (r *RmqHealthData) RenderCompact() string {
	var sb strings.Builder

	// Service Status section
	sb.WriteString(common.SectionTitle("Service Status"))
	sb.WriteString("\n")
	sb.WriteString(simpleStatusListItem(
		"RabbitMQ Service",
		"active",
		r.Service.Active))
	sb.WriteString("\n")

	// Management Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Management Status"))
	sb.WriteString("\n")
	sb.WriteString(simpleStatusListItem(
		"Management Plugin",
		"enabled",
		r.Management.Enabled))
	sb.WriteString("\n")
	sb.WriteString(simpleStatusListItem(
		"Management Service",
		"active",
		r.Management.Active))
	sb.WriteString("\n")

	// Port Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Port Status"))
	sb.WriteString("\n")
	sb.WriteString(simpleStatusListItem(
		"AMQP Port (5672)",
		"open",
		r.Ports.AMQP))
	sb.WriteString("\n")
	sb.WriteString(simpleStatusListItem(
		"Management Port (15672)",
		"open",
		r.Ports.Management))
	sb.WriteString("\n")
	for port, status := range r.Ports.OtherPorts {
		sb.WriteString(simpleStatusListItem(
			fmt.Sprintf("Port %s", port),
			"open",
			status))
		sb.WriteString("\n")
	}

	// API Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("API Status"))
	sb.WriteString("\n")
	sb.WriteString(simpleStatusListItem(
		"Management API",
		"connected",
		r.API.Connected))
	sb.WriteString("\n")
	sb.WriteString(simpleStatusListItem(
		"Overview API",
		"reachable",
		r.API.OverviewOK))
	sb.WriteString("\n")

	// Cluster Status section
	if len(r.Cluster.Nodes) > 0 {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Cluster Status"))
		sb.WriteString("\n")
		for _, node := range r.Cluster.Nodes {
			sb.WriteString(simpleStatusListItem(
				fmt.Sprintf("Node %s", node.Name),
				"running",
				node.IsRunning))
			sb.WriteString("\n")
		}
	}

	// Queue Health section
	if r.Queues.FetchOK && r.Queues.TotalCount > 0 {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Queue Health"))
		sb.WriteString("\n")

		// Özet satırı
		summaryOK := r.Queues.StoppedCount == 0 && r.Queues.UnsyncedCount == 0
		summaryLabel := fmt.Sprintf("Queues (%d total)", r.Queues.TotalCount)
		summaryState := fmt.Sprintf("all healthy")
		if !summaryOK {
			summaryState = fmt.Sprintf("%d stopped, %d unsynced", r.Queues.StoppedCount, r.Queues.UnsyncedCount)
		}
		sb.WriteString(simpleStatusListItem(summaryLabel, summaryState, summaryOK))
		sb.WriteString("\n")

		if r.Queues.NoConsumerCount > 0 {
			sb.WriteString(simpleStatusListItem(
				"Consumers",
				fmt.Sprintf("%d queues without consumer", r.Queues.NoConsumerCount),
				false))
			sb.WriteString("\n")
		}

		if r.Queues.HighMessageCount > 0 {
			sb.WriteString(simpleStatusListItem(
				"Message Backlog",
				fmt.Sprintf("%d queues above threshold", r.Queues.HighMessageCount),
				false))
			sb.WriteString("\n")
		}

		// Sorunlu kuyrukları listele
		for _, q := range r.Queues.Items {
			hasIssue := q.Stopped || q.NoConsumer || q.HighMessages || !q.SyncStatus.IsFullySynced
			if !hasIssue {
				continue
			}

			var issues []string
			if q.Stopped {
				issues = append(issues, fmt.Sprintf("state=%s", q.State))
			}
			if q.NoConsumer {
				issues = append(issues, "no consumer")
			}
			if q.HighMessages {
				issues = append(issues, fmt.Sprintf("messages=%d", q.Messages))
			}
			if !q.SyncStatus.IsFullySynced && q.SyncStatus.TotalReplicas > 0 {
				issues = append(issues, fmt.Sprintf("sync=%d/%d", q.SyncStatus.SyncedReplicas, q.SyncStatus.TotalReplicas-1))
			}

			label := fmt.Sprintf("  %s", q.Name)
			detail := strings.Join(issues, ", ")
			sb.WriteString(simpleStatusListItem(label, detail, false))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (r *RmqHealthData) RenderAll() string {
	title := fmt.Sprintf("monokit rmqHealth v%s - %s", r.Version, r.LastChecked)
	content := r.RenderCompact()
	return common.DisplayBox(title, content)
}

func simpleStatusListItem(label string, expectedState string, isSuccess bool) string {
	statusStyle := lipgloss.NewStyle().Foreground(common.SuccessColor)
	if !isSuccess {
		statusStyle = lipgloss.NewStyle().Foreground(common.ErrorColor)
		expectedState = "not " + expectedState
	}

	contentStyle := lipgloss.NewStyle().
		Align(lipgloss.Left).
		PaddingLeft(8)

	itemStyle := lipgloss.NewStyle().
		Foreground(common.NormalTextColor)
	line := fmt.Sprintf("•  %-20s is %s",
		label,
		statusStyle.Render(expectedState))

	return contentStyle.Render(itemStyle.Render(line))
}
