package osHealth

import (
	"fmt"
	"strconv"
	"strings"

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
	Name       string
	Status     string
	Used       string
	Total      string
	UsedPct    float64
	ScanStatus string
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
	
	// We'll use the common display utilities instead of defining styles here
		
	// Disk Usage section
	sb.WriteString(common.SectionTitle("Disk Usage"))
	sb.WriteString("\n")
	
	// Disk usage details
	for _, disk := range h.Disk {
		isSuccess := disk.UsedPct <= OsHealthConfig.Part_use_limit
		
		limits := strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "%"
		current := strconv.FormatFloat(disk.UsedPct, 'f', 0, 64) + "%"
		
		sb.WriteString(common.StatusListItem(
			disk.Mountpoint,
			"", // use default prefix
			limits,
			current,
			isSuccess))
		sb.WriteString("\n")
	}
	
	// System Load and RAM section
	sb.WriteString("\n")
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
