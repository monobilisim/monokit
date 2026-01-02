package winHealth

import (
	"fmt"
	"strings"

	// "github.com/charmbracelet/lipgloss" // Removed as it's no longer used
	"github.com/monobilisim/monokit/common"
)

// WinHealthConfig is already defined in the package

// HealthData represents system health data
type HealthData struct {
	System          SystemInfo
	Disk            []DiskInfo
	Memory          MemoryInfo
	SystemLoad      SystemLoadInfo
	WindowsServices []WindowsServiceInfo
	License         LicenseInfo
}

// LicenseInfo represents Windows activation status
type LicenseInfo struct {
	Status        string // e.g. "Licensed", "OOB grace"
	Description   string // e.g. "Windows(R) Operating System, VOLUME_KMSCLIENT channel"
	IsLicensed    bool
	Remaining     string // e.g. "120 days" or "Permanent"
	RemainingDays int    // -1 if permanent or unknown, otherwise days
}

// SystemInfo represents basic system information
type SystemInfo struct {
	Hostname    string
	Uptime      string
	OS          string
	KernelVer   string
	LastChecked string
}

// DiskInfo represents disk partition information
type DiskInfo struct {
	Device     string
	Mountpoint string
	Used       string
	Total      string
	UsedPct    float64
	Fstype     string
}

// MemoryInfo represents memory usage information
type MemoryInfo struct {
	Used     string
	Total    string
	UsedPct  float64
	Limit    float64
	Exceeded bool
}

// SystemLoadInfo represents system load information
type SystemLoadInfo struct {
	Load1      float64
	Load5      float64
	Load15     float64
	CPUCount   int
	Multiplier float64
	Exceeded   bool
}

// WindowsServiceInfo represents windows service information
type WindowsServiceInfo struct {
	Name        string
	DisplayName string
	Status      string
	State       string
}

// NewHealthData creates a new HealthData
func NewHealthData() *HealthData {
	return &HealthData{
		Disk:            []DiskInfo{},
		WindowsServices: []WindowsServiceInfo{},
	}
}

// RenderCompact renders a compact view of all health data for a single-box display
func (h *HealthData) RenderCompact() string {
	var sb strings.Builder

	// System / License Section
	if common.Config.Identifier != "" || h.License.Status != "" {
		sb.WriteString(common.SectionTitle("System"))
		sb.WriteString("\n")

		if h.License.Status != "" && h.License.Status != "N/A" {
			// Show detailed license description if available as hover/extra? No, just keep it simple.
			// Use Description if relevant, or just Status.
			label := "License: " + h.License.Description
			if len(label) > 40 {
				label = label[:37] + "..."
			}

			statusDisplay := h.License.Status
			if h.License.Remaining != "" {
				if strings.Contains(h.License.Remaining, "days") || strings.Contains(h.License.Remaining, "hours") {
					statusDisplay += " (valid for " + h.License.Remaining + ")"
				} else {
					statusDisplay += " (" + h.License.Remaining + ")"
				}
			}

			sb.WriteString(common.SimpleStatusListItem(
				"Windows Activation",
				statusDisplay,
				h.License.IsLicensed))
			sb.WriteString("\n")
		}
	}

	if sb.Len() > 0 {
		sb.WriteString("\n")
	}

	// Disk Usage section - only show if there are disk partitions
	if len(h.Disk) > 0 {
		sb.WriteString(common.SectionTitle("Disk Usage"))
		sb.WriteString("\n")

		// Disk usage details
		for _, disk := range h.Disk {
			isSuccess := disk.UsedPct <= WinHealthConfig.Part_use_limit

			limits := fmt.Sprintf("%.0f%%", WinHealthConfig.Part_use_limit)
			current := fmt.Sprintf("%.0f%%", disk.UsedPct)

			sb.WriteString(common.StatusListItem(
				disk.Mountpoint,
				"", // use default prefix
				limits,
				current,
				isSuccess))
			sb.WriteString("\n")
		}
	}

	// System Load and RAM section
	// Add spacing only if we had a previous section (either Disk or ZFS Pools)
	if len(h.Disk) > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(common.SectionTitle("System Load and RAM"))
	sb.WriteString("\n")

	// System Load details
	loadLimit := float64(h.SystemLoad.CPUCount) * h.SystemLoad.Multiplier
	isLoadSuccess := !h.SystemLoad.Exceeded

	sb.WriteString(common.StatusListItem(
		"System Load",
		"", // use default prefix
		fmt.Sprintf("%.2f", loadLimit),
		fmt.Sprintf("%.2f", h.SystemLoad.Load5),
		isLoadSuccess))
	sb.WriteString("\n")

	// RAM usage details
	isRamSuccess := !h.Memory.Exceeded

	sb.WriteString(common.StatusListItem(
		"RAM Usage",
		"", // use default prefix
		fmt.Sprintf("%.0f%%", h.Memory.Limit),
		fmt.Sprintf("%.0f%%", h.Memory.UsedPct),
		isRamSuccess))

	// Windows Services section
	if len(h.WindowsServices) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(common.SectionTitle("Windows Services"))
		sb.WriteString("\n")

		for _, svc := range h.WindowsServices {
			isSuccess := svc.State == "running" || svc.State == "active"

			sb.WriteString(common.SimpleStatusListItem(
				svc.DisplayName,
				svc.State,
				isSuccess))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// RenderAll renders all health data as a single string
func (h *HealthData) RenderAll() string {
	return h.RenderCompact()
}
