package traefikHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
)

// TraefikHealthData represents the health status of Traefik
type TraefikHealthData struct {
	Version     string
	LastChecked string
	Service     ServiceInfo
	Ports       PortsInfo
	Logs        LogsInfo
	IsHealthy   bool
}

// ServiceInfo represents Traefik service status
type ServiceInfo struct {
	Active bool
}

// PortsInfo represents the status of Traefik ports
type PortsInfo struct {
	PortStatus map[uint32]bool
	AllPortsOK bool
}

// LogsInfo represents Traefik log information
type LogsInfo struct {
	LastChecked string
	Errors      []LogEntry
	Warnings    []LogEntry
	HasIssues   bool
}

// LogEntry represents a single log entry from Traefik logs
type LogEntry struct {
	Time     string
	Level    string
	Message  string
	Error    string
	Provider string
	Domains  string
}

// NewTraefikHealthData creates a new TraefikHealthData structure with initialized fields
func NewTraefikHealthData() *TraefikHealthData {
	return &TraefikHealthData{
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
		Ports: PortsInfo{
			PortStatus: make(map[uint32]bool),
			AllPortsOK: true,
		},
		Logs: LogsInfo{
			Errors:    []LogEntry{},
			Warnings:  []LogEntry{},
			HasIssues: false,
		},
		IsHealthy: true, // Start optimistically
	}
}

// RenderCompact renders a compact view of the Traefik health data
func (t *TraefikHealthData) RenderCompact() string {
	var sb strings.Builder

	// Service Status section
	sb.WriteString(common.SectionTitle("Service Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Traefik Service",
		"active",
		t.Service.Active))
	sb.WriteString("\n")

	// Port Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Port Status"))
	sb.WriteString("\n")
	for port, status := range t.Ports.PortStatus {
		sb.WriteString(common.SimpleStatusListItem(
			fmt.Sprintf("Port %d", port),
			"open",
			status))
		sb.WriteString("\n")
	}

	// Log Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Log Status"))
	sb.WriteString("\n")

	if len(t.Logs.Errors) == 0 && len(t.Logs.Warnings) == 0 {
		sb.WriteString("No issues found in logs since last check.\n")
	} else {
		if len(t.Logs.Errors) > 0 {
			sb.WriteString(fmt.Sprintf("Found %d errors in logs.\n", len(t.Logs.Errors)))
			for _, entry := range t.Logs.Errors {
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", entry.Time, entry.Message))
			}
		}

		if len(t.Logs.Warnings) > 0 {
			sb.WriteString(fmt.Sprintf("Found %d warnings in logs.\n", len(t.Logs.Warnings)))
			for _, entry := range t.Logs.Warnings {
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", entry.Time, entry.Message))
			}
		}
	}

	return sb.String()
}

// RenderAll renders all Traefik health data as a single string
func (t *TraefikHealthData) RenderAll() string {
	// Use title and content with the common.DisplayBox function
	title := fmt.Sprintf("monokit traefikHealth v%s - %s", t.Version, t.LastChecked)
	content := t.RenderCompact()
	return common.DisplayBox(title, content)
}
