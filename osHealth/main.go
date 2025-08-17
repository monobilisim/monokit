package osHealth

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/health"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/disk" // For GetDiskPartitions
	"github.com/spf13/cobra"
)

// OsHealthProvider implements the health.Provider interface
type OsHealthProvider struct{}

// Name returns the name of the provider
func (p *OsHealthProvider) Name() string {
	return "osHealth"
}

// Collect gathers OS health data.
// The 'hostname' parameter is ignored for osHealth as it collects local data.
func (p *OsHealthProvider) Collect(_ string) (interface{}, error) {
	// Initialize config if not already done (e.g. if called directly, not via CLI)
	// This is a simplified approach; a more robust solution might involve a dedicated config loader.
	if OsHealthConfig.Load.Issue_Multiplier == 0 { // Check if config is uninitialized
		common.ConfInit("os", &OsHealthConfig) // Load "os" config into OsHealthConfig
		if OsHealthConfig.Load.Issue_Multiplier == 0 {
			OsHealthConfig.Load.Issue_Multiplier = 1
		}
		if OsHealthConfig.Load.Issue_Interval == 0 {
			OsHealthConfig.Load.Issue_Interval = 15
		}
	}
	return collectHealthData("api"), nil // "api" as version, or make version dynamic
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "osHealth",
		EntryPoint: Main,
		Platform:   "any", // Runs on multiple OS, specific features checked internally
	})
	health.Register(&OsHealthProvider{})
}

// types.go
var OsHealthConfig OsHealth

func Main(cmd *cobra.Command, args []string) {
	version := "2.3.0"
	common.ScriptName = "osHealth"
	common.TmpDir = common.TmpDir + "osHealth"
	common.Init()
	common.ConfInit("os", &OsHealthConfig)

	// Check service status with the Monokit server.
	// This initializes common.ClientURL, common.Config.Identifier,
	// checks if the service is enabled, and handles updates.
	common.WrapperGetServiceStatus("osHealth")

	// Initialize configuration defaults
	if OsHealthConfig.Load.Issue_Multiplier == 0 {
		OsHealthConfig.Load.Issue_Multiplier = 1
	}

	if OsHealthConfig.Load.Issue_Interval == 0 {
		OsHealthConfig.Load.Issue_Interval = 15
	}

	// Collect health data
	healthData := collectHealthData(version)

	// Attempt to POST health data to the Monokit server
	if err := common.PostHostHealth("osHealth", healthData); err != nil {
		log.Error().Err(err).Msg("osHealth: failed to POST health data")
		// Continue execution even if POST fails, e.g., to display UI locally
	}

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

	// Collect ZFS dataset info and handle alarms
	healthData.ZFSDatasets = collectZFSDatasetInfo()
	var exceededDatasets []ZFSDatasetInfo
	for _, d := range healthData.ZFSDatasets {
		if d.UsedPct > OsHealthConfig.Part_use_limit {
			exceededDatasets = append(exceededDatasets, d)
		}
	}
	if len(exceededDatasets) > 0 {
		fullMsg, tableOnly := createExceededZFSDatasetTable(exceededDatasets)
		subject := common.Config.Identifier + " için ZFS dataset doluluk seviyesi %" + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + " üstüne çıktı"
		issues.CheckDown("zfsdataset", subject, tableOnly, false, 0)
		id := issues.Show("zfsdataset")
		if id != "" {
			fullMsg = fullMsg + "\n\n" + "Redmine Issue: " + common.Config.Redmine.Url + "/issues/" + id
			common.AlarmCheckUp("zfsdataset_redmineissue", "Redmine issue exists for ZFS dataset usage", false)
		} else {
			log.Debug().Msg("osHealth/main.go: issues.Show(\"zfsdataset\") returned empty. Proceeding without Redmine link in alarm.")
		}
		common.AlarmCheckDown("zfsdataset", fullMsg, false, "", "")
	} else if len(healthData.ZFSDatasets) > 0 {
		common.AlarmCheckUp("zfsdataset", "All ZFS datasets are below "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"% usage.", false)
		issues.CheckUp("zfsdataset", common.Config.Identifier+" için bütün ZFS datasetleri "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"% altına indi, kapatılıyor.")
		common.AlarmCheckUp("zfsdataset_redmineissue", "ZFS dataset usage normal, clearing any Redmine issue creation failure alarm", false)
	}

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

// Helper function to collect disk information and handle alarms/issues
func collectDiskInfo() []DiskInfo {
	gopsutilDiskPartitions, err := disk.Partitions(false) // Using gopsutil/disk directly
	if err != nil {
		log.Error().Err(err).Msg("An error occurred while fetching disk partitions")
		return []DiskInfo{} // Return empty list on error
	}

	// analyzeDiskPartitions now returns []DiskInfo, []DiskInfo
	exceededDIs, allDIs := analyzeDiskPartitions(gopsutilDiskPartitions)

	// Alarm and Redmine logic integrated here
	if len(exceededDIs) > 0 {
		fullMsg, tableOnly := createExceededTable(exceededDIs) // createExceededTable now takes []DiskInfo
		subject := common.Config.Identifier + " için disk doluluk seviyesi %" + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + " üstüne çıktı"

		issues.CheckDown("disk", subject, tableOnly, false, 0)
		id := issues.Show("disk")

		if id != "" {
			fullMsg = fullMsg + "\n\n" + "Redmine Issue: " + common.Config.Redmine.Url + "/issues/" + id
			common.AlarmCheckUp("disk_redmineissue", "Redmine issue exists for disk usage", false)
		} else {
			log.Debug().Msg("osHealth/main.go: issues.Show(\"disk\") returned empty. Proceeding without Redmine link in alarm.")
		}
		common.AlarmCheckDown("disk", fullMsg, false, "", "")

	} else {
		fullMsg, tableOnly := createNormalTable(allDIs) // createNormalTable now takes []DiskInfo
		common.AlarmCheckUp("disk", fullMsg, false)
		issues.CheckUp("disk", common.Config.Identifier+" için bütün disk bölümleri "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"% altına indi, kapatılıyor."+"\n\n"+tableOnly)
		common.AlarmCheckUp("disk_redmineissue", "Disk usage normal, clearing any Redmine issue creation failure alarm", false)
	}

	// The function now returns allDIs which is []DiskInfo, suitable for the UI
	return allDIs
}

// Helper function to collect memory information
func collectMemoryInfo() MemoryInfo {
	memInfo := MemoryInfo{
		Limit: OsHealthConfig.Ram_Limit,
	}

	virtualMemory, err := GetVirtualMemory() // from osHealth/utils.go
	if err != nil {
		log.Error().Err(err).Msg("Error getting virtual memory stats")
		// memInfo will have default (zero) values and Exceeded will be false.
		return memInfo
	}

	memInfo.Used = common.ConvertBytes(virtualMemory.Used)
	memInfo.Total = common.ConvertBytes(virtualMemory.Total)
	memInfo.UsedPct = virtualMemory.UsedPercent
	memInfo.Exceeded = virtualMemory.UsedPercent > OsHealthConfig.Ram_Limit // For UI

	// Integrated Alarm and Redmine Logic from RamUsage()
	ramLimit := OsHealthConfig.Ram_Limit // This is memInfo.Limit

	if virtualMemory.UsedPercent > ramLimit {
		common.AlarmCheckDown("ram", "RAM usage limit has exceeded "+strconv.FormatFloat(ramLimit, 'f', 0, 64)+"% (Current: "+strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64)+"%)", false, "", "")
		issues.CheckDown("ram", common.Config.Identifier+" için hafıza kullanımı %"+strconv.FormatFloat(ramLimit, 'f', 0, 64)+" üstüne çıktı", "Hafıza kullanımı: "+strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64)+"%\n Hafıza limiti: "+strconv.FormatFloat(ramLimit, 'f', 0, 64)+"%", false, 0)
	} else {
		common.AlarmCheckUp("ram", "RAM usage went below "+strconv.FormatFloat(ramLimit, 'f', 0, 64)+"% (Current: "+strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64)+"%)", false)
		issues.CheckUp("ram", common.Config.Identifier+" için hafıza kullanımı %"+strconv.FormatFloat(ramLimit, 'f', 0, 64)+" altına düştü")
	}

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

    // Trigger sysload alerting via the dedicated function
    SysLoad()

	return loadInfo
}

// Helper function to collect ZFS information
func collectZFSInfo() []ZFSPoolInfo {
	// Call ZFSHealth to get pool information and handle alarms/issues
	return ZFSHealth()
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
