package rmqHealth

import (
	"fmt"
	"strings"
	"time"

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
	IsHealthy   bool
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
		IsHealthy: true, // Start optimistically
	}
}

// RenderCompact renders a compact view of the RabbitMQ health data
func (r *RmqHealthData) RenderCompact() string {
	var sb strings.Builder

	// Service Status section
	sb.WriteString(common.SectionTitle("Service Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"RabbitMQ Service",
		"active",
		r.Service.Active))
	sb.WriteString("\n")

	// Management Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Management Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Management Plugin",
		"enabled",
		r.Management.Enabled))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Management Service",
		"active",
		r.Management.Active))
	sb.WriteString("\n")

	// Port Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Port Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"AMQP Port (5672)",
		"open",
		r.Ports.AMQP))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Management Port (15672)",
		"open",
		r.Ports.Management))
	sb.WriteString("\n")
	for port, status := range r.Ports.OtherPorts {
		sb.WriteString(common.SimpleStatusListItem(
			fmt.Sprintf("Port %s", port),
			"open",
			status))
		sb.WriteString("\n")
	}

	// API Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("API Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Management API",
		"connected",
		r.API.Connected))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
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
			sb.WriteString(common.SimpleStatusListItem(
				fmt.Sprintf("Node %s", node.Name),
				"running",
				node.IsRunning))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// RenderAll renders all RabbitMQ health data as a single string
func (r *RmqHealthData) RenderAll() string {
	// Use title and content with the common.DisplayBox function
	title := fmt.Sprintf("monokit rmqHealth v%s - %s", r.Version, r.LastChecked)
	content := r.RenderCompact()
	return common.DisplayBox(title, content)
}
