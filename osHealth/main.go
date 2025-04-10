package osHealth

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra"
)

// types.go
var OsHealthConfig OsHealth

func Main(cmd *cobra.Command, args []string) {
	version := "2.3.0"
	common.ScriptName = "osHealth"
	common.TmpDir = common.TmpDir + "osHealth"
	common.Init()
	common.ConfInit("os", &OsHealthConfig)

	// Initialize configuration defaults
	if OsHealthConfig.Load.Issue_Multiplier == 0 {
		OsHealthConfig.Load.Issue_Multiplier = 1
	}

	if OsHealthConfig.Load.Issue_Interval == 0 {
		OsHealthConfig.Load.Issue_Interval = 15
	}
	
	// Collect health data
	healthData := collectHealthData(version)
	
	// Display as a nice box UI
	displayBoxUI(healthData)
	
	// Process system logs in the background (Linux only)
	if runtime.GOOS == "linux" {
		SystemdLogs()
	}
}

// collectHealthData gathers all the health information
func collectHealthData(version string) *HealthData {
	// Create health data model
	healthData := NewHealthData()
	
	// Set system info
	hostname, _ := os.Hostname()
	healthData.System.Hostname = hostname
	healthData.System.OS = runtime.GOOS
	healthData.System.KernelVer = runtime.GOARCH
	healthData.System.LastChecked = time.Now().Format("2006-01-02 15:04:05")
	
	// Collect disk information
	healthData.Disk = collectDiskInfo()
	
	// Collect memory information
	healthData.Memory = collectMemoryInfo()
	
	// Collect system load
	healthData.SystemLoad = collectSystemLoad()
	
	// Collect ZFS info if available
	healthData.ZFSPools = collectZFSInfo()
	
	// Collect systemd unit information if on Linux
	if runtime.GOOS == "linux" {
		healthData.SystemdUnits = collectSystemdInfo()
	}
	
	return healthData
}

// displayBoxUI displays the health data in a nice box UI
func displayBoxUI(healthData *HealthData) {
	// Set up the title and content
	title := "monokit osHealth"
	content := healthData.RenderAll()
	
	// Format and print the output using common display utilities
	renderedBox := common.DisplayBox(title, content)
	
	fmt.Println(renderedBox)
}

// Helper function to collect disk information
func collectDiskInfo() []DiskInfo {
	var diskInfoList []DiskInfo
	
	diskPartitions, err := GetDiskPartitions(false)
	if err != nil {
		return diskInfoList
	}
	
	for _, partition := range diskPartitions {
		usage, err := GetDiskUsage(partition.Mountpoint)
		if err != nil {
			continue
		}
		
		diskInfo := DiskInfo{
			Device:     partition.Device,
			Mountpoint: partition.Mountpoint,
			Used:       common.ConvertBytes(usage.Used),
			Total:      common.ConvertBytes(usage.Total),
			UsedPct:    usage.UsedPercent,
			Fstype:     partition.Fstype,
		}
		
		diskInfoList = append(diskInfoList, diskInfo)
	}
	
	return diskInfoList
}

// Helper function to collect memory information
func collectMemoryInfo() MemoryInfo {
	memInfo := MemoryInfo{
		Limit: OsHealthConfig.Ram_Limit,
	}
	
	virtualMemory, err := GetVirtualMemory()
	if err != nil {
		return memInfo
	}
	
	memInfo.Used = common.ConvertBytes(virtualMemory.Used)
	memInfo.Total = common.ConvertBytes(virtualMemory.Total)
	memInfo.UsedPct = virtualMemory.UsedPercent
	memInfo.Exceeded = virtualMemory.UsedPercent > OsHealthConfig.Ram_Limit
	
	return memInfo
}

// Helper function to collect system load information
func collectSystemLoad() SystemLoadInfo {
	loadInfo := SystemLoadInfo{
		Multiplier: OsHealthConfig.Load.Issue_Multiplier,
	}
	
	loadAvg, err := GetLoadAvg()
	if err != nil {
		return loadInfo
	}
	
	cpuCount, err := GetCPUCount()
	if err != nil {
		cpuCount = 1
	}
	
	loadInfo.Load1 = loadAvg.Load1
	loadInfo.Load5 = loadAvg.Load5
	loadInfo.Load15 = loadAvg.Load15
	loadInfo.CPUCount = cpuCount
	
	// Check if load exceeds the limit
	limit := float64(cpuCount) * OsHealthConfig.Load.Issue_Multiplier
	loadInfo.Exceeded = loadAvg.Load5 > limit
	
	return loadInfo
}

// Helper function to collect ZFS information
func collectZFSInfo() []ZFSPoolInfo {
	var zfsPools []ZFSPoolInfo
	
	// ZFS collection logic here, if applicable
	// This would involve calling zpool status or similar commands
	
	return zfsPools
}

// Helper function to collect systemd information
func collectSystemdInfo() []SystemdUnitInfo {
	var systemdUnits []SystemdUnitInfo
	
	// Get important systemd units
	units, err := GetSystemdUnits()
	if err != nil {
		return systemdUnits
	}
	
	// Add important units
	for _, unit := range units {
		if unit.Type == "service" && (unit.State == "active" || unit.State == "failed") {
			systemdUnit := SystemdUnitInfo{
				Name:        unit.Name,
				Description: unit.Description,
				Status:      unit.SubState,
				Active:      unit.State == "active",
			}
			
			systemdUnits = append(systemdUnits, systemdUnit)
		}
	}
	
	return systemdUnits
}
