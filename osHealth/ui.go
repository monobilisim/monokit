package osHealth

import (
	"fmt"
	"math"
	"strings"

	// "github.com/charmbracelet/lipgloss" // Removed as it's no longer used
	"github.com/monobilisim/monokit/common"
)

// OsHealthConfig is already defined in the package

// HealthData represents system health data
type HealthData struct {
	System       SystemInfo
	Disk         []DiskInfo
	Memory       MemoryInfo
	SystemLoad   SystemLoadInfo
	ZFSPools     []ZFSPoolInfo
	ZFSDatasets  []ZFSDatasetInfo
	SystemdUnits []SystemdUnitInfo
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

// ZFSPoolInfo represents ZFS pool information
type ZFSPoolInfo struct {
	Name    string
	Status  string
	Used    string
	Total   string
	UsedPct float64
}

// ZFSDatasetInfo represents ZFS dataset usage information
type ZFSDatasetInfo struct {
	Name    string
	Used    string
	Avail   string
	UsedPct float64
}

// SystemdUnitInfo represents systemd unit information
type SystemdUnitInfo struct {
	Name        string
	Description string
	Status      string
	Active      bool
}

// NewHealthData creates a new HealthData
func NewHealthData() *HealthData {
	return &HealthData{
		Disk:         []DiskInfo{},
		ZFSPools:     []ZFSPoolInfo{},
		SystemdUnits: []SystemdUnitInfo{},
	}
}

// RenderCompact renders a compact view of all health data for a single-box display
func (h *HealthData) RenderCompact() string {
	var sb strings.Builder

	// Disk Usage section - only show if there are disk partitions
	if len(h.Disk) > 0 {
		sb.WriteString(common.SectionTitle("Disk Usage"))
		sb.WriteString("\n")

		// Disk usage details
		for _, disk := range h.Disk {
			isSuccess := disk.UsedPct <= OsHealthConfig.Part_use_limit

			limits := fmt.Sprintf("%.0f%%", OsHealthConfig.Part_use_limit)
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

	// ZFS Pools section if any exist
	if len(h.ZFSPools) > 0 {
		// Add spacing only if we had a previous section
		if len(h.Disk) > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(common.SectionTitle("ZFS Pools"))
		sb.WriteString("\n")

		for _, pool := range h.ZFSPools {
			isSuccess := pool.Status == "ONLINE"

			// Format pool status string using SimpleStatusListItem
			labelText := pool.Name
			statusDescription := ""
			if isSuccess {
				statusDescription = "ONLINE"
			} else {
				statusDescription = fmt.Sprintf("ONLINE (actual: %s)", pool.Status)
			}
			sb.WriteString(common.SimpleStatusListItem(labelText, statusDescription, isSuccess))
			sb.WriteString("\n")

			// Format usage string using SimpleStatusListItem for consistent "is" formatting
			usageLabelText := pool.Name + " usage"
			// Use math.Floor to match zpool list behavior (truncate instead of round)
			usageStatusDescription := fmt.Sprintf("%s / %s (%.0f%%)", pool.Used, pool.Total, math.Floor(pool.UsedPct))
			sb.WriteString(common.SimpleStatusListItem(usageLabelText, usageStatusDescription, true)) // true for isSuccess to keep it green/neutral
			sb.WriteString("\n")
		}
	}

	// System Load and RAM section
	// Add spacing only if we had a previous section (either Disk or ZFS Pools)
	if len(h.Disk) > 0 || len(h.ZFSPools) > 0 {
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

	return sb.String()
}

// RenderAll renders all health data as a single string
func (h *HealthData) RenderAll() string {
	return h.RenderCompact()
}
