package winHealth

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/health"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/disk" // For GetDiskPartitions
	"github.com/shirou/gopsutil/v4/process"
	"github.com/spf13/cobra"
)

// WinHealthProvider implements the health.Provider interface
type WinHealthProvider struct{}

// Name returns the name of the provider
func (p *WinHealthProvider) Name() string {
	return "winHealth"
}

// Collect gathers OS health data.
// The 'hostname' parameter is ignored for osHealth as it collects local data.
func (p *WinHealthProvider) Collect(_ string) (interface{}, error) {
	// Initialize config if not already done (e.g. if called directly, not via CLI)
	// This is a simplified approach; a more robust solution might involve a dedicated config loader.
	if WinHealthConfig.Load.Issue_Multiplier == 0 { // Check if config is uninitialized
		common.ConfInit("win", &WinHealthConfig) // Load "win" config into WinHealthConfig
		if WinHealthConfig.Load.Issue_Multiplier == 0 {
			WinHealthConfig.Load.Issue_Multiplier = 1
		}
		if WinHealthConfig.Load.Issue_Interval == 0 {
			WinHealthConfig.Load.Issue_Interval = 15
		}
	}
	return collectHealthData("api"), nil // "api" as version, or make version dynamic
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "winHealth",
		EntryPoint: Main,
		Platform:   "windows", // Only runs on Windows
	})
	health.Register(&WinHealthProvider{})
}

type ProcessInfo struct {
	PID           int32
	Command       string
	Username      string
	CPUPercent    float64
	MemoryPercent float32
}

var allProcesses []ProcessInfo

// types.go
var WinHealthConfig WinHealth

func Main(cmd *cobra.Command, args []string) {
	if runtime.GOOS != "windows" {
		fmt.Println("winHealth is only supported on Windows. Use osHealth for Linux/macOS.")
		return
	}

	version := "2.3.0"
	common.ScriptName = "winHealth"
	common.TmpDir = common.TmpDir + "winHealth"
	common.Init()
	common.ConfInit("win", &WinHealthConfig)

	// Check service status with the Monokit server.
	// This initializes common.ClientURL, common.Config.Identifier,
	// checks if the service is enabled, and handles updates.
	common.WrapperGetServiceStatus("winHealth")

	// Initialize configuration defaults
	if WinHealthConfig.Load.Issue_Multiplier == 0 {
		WinHealthConfig.Load.Issue_Multiplier = 1
	}

	if WinHealthConfig.Load.Issue_Interval == 0 {
		WinHealthConfig.Load.Issue_Interval = 15
	}

	if WinHealthConfig.Load.Limit_Multiplier == 0 {
		WinHealthConfig.Load.Limit_Multiplier = WinHealthConfig.Load.Issue_Multiplier
	}

	if WinHealthConfig.License.Expiration_Limit == 0 {
		WinHealthConfig.License.Expiration_Limit = 30
	}

	// Collect health data
	healthData := collectHealthData(version)

	// Attempt to POST health data to the Monokit server
	if err := common.PostHostHealth("winHealth", healthData); err != nil {
		log.Error().Err(err).Msg("osHealth: failed to POST health data")
		// Continue execution even if POST fails, e.g., to display UI locally
	}

	// Display as a nice box UI
	displayBoxUI(healthData)

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

	// Collect license status (Windows only via build tags)
	healthData.License = GetWindowsLicenseStatus()

	// License Alarm Check
	log.Debug().Int("expiration_limit", WinHealthConfig.License.Expiration_Limit).Int("remaining_days", healthData.License.RemainingDays).Msg("Checking License Alarm")

	if healthData.License.RemainingDays != -1 && healthData.License.RemainingDays < WinHealthConfig.License.Expiration_Limit {
		// Alarm (Webhook) - English
		msg := fmt.Sprintf("Windows License expires soon (%s remaining)", healthData.License.Remaining)

		// Issue (Redmine) - Turkish Localization
		statusTR := healthData.License.Status
		if statusTR == "Licensed" {
			statusTR = "Lisanslı"
		} else if statusTR == "Unlicensed" {
			statusTR = "Lisanssız"
		} else if strings.Contains(statusTR, "Grace") {
			statusTR = "Deneme Sürümü (Grace)"
		} else if statusTR == "Notification" {
			statusTR = "Bildirim Modu"
		}

		remainingTR := strings.ReplaceAll(healthData.License.Remaining, "days", "gün")
		remainingTR = strings.ReplaceAll(remainingTR, "hours", "saat")

		issueTitle := fmt.Sprintf("%s Windows Lisansı %d gün içinde bitiyor", common.Config.Identifier, healthData.License.RemainingDays)

		issueMsg := fmt.Sprintf("Windows Lisans süresi dolmak üzere.\nMinimum Limit: %d gün\nDurum: %s\nKalan Süre: %s",
			WinHealthConfig.License.Expiration_Limit,
			statusTR,
			remainingTR)

		common.AlarmCheckDown("license", msg, false, "", "")
		issues.CheckDown("license", issueTitle, issueMsg, false, 0)
	} else {
		// Clear issue if valid
		remainingTR := strings.ReplaceAll(healthData.License.Remaining, "days", "gün")
		remainingTR = strings.ReplaceAll(remainingTR, "hours", "saat")
		remainingTR = strings.ReplaceAll(remainingTR, "Permanent", "Kalıcı")

		common.AlarmCheckUp("license", fmt.Sprintf("Windows License is valid (%s remaining)", healthData.License.Remaining), false)
		issues.CheckUp("license", fmt.Sprintf("Windows Lisansı geçerli (%s)", remainingTR))
	}

	// Collect system load
	healthData.SystemLoad = collectSystemLoad()

	// Collect Windows Services info
	// Collect Windows Services info
	if runtime.GOOS == "windows" && WinHealthConfig.Services.Enabled {
		healthData.WindowsServices = collectWindowsServicesInfo()
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

	if len(gopsutilDiskPartitions) == 0 {
		log.Warn().Msg("No disk partitions found via gopsutil")
	} else {
		log.Debug().Interface("partitions", gopsutilDiskPartitions).Msg("Found disk partitions")
	}

	// analyzeDiskPartitions now returns []DiskInfo, []DiskInfo
	exceededDIs, allDIs := analyzeDiskPartitions(gopsutilDiskPartitions)

	// Alarm and Redmine logic integrated here
	if len(exceededDIs) > 0 {
		fullMsg, tableOnly := createExceededTable(exceededDIs) // createExceededTable now takes []DiskInfo
		subject := common.Config.Identifier + " için disk doluluk seviyesi %" + strconv.FormatFloat(WinHealthConfig.Part_use_limit, 'f', 0, 64) + " üstüne çıktı"

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
		issues.CheckUp("disk", common.Config.Identifier+" için bütün disk bölümleri "+strconv.FormatFloat(WinHealthConfig.Part_use_limit, 'f', 0, 64)+"% altına indi, kapatılıyor."+"\n\n"+tableOnly)
		common.AlarmCheckUp("disk_redmineissue", "Disk usage normal, clearing any Redmine issue creation failure alarm", false)
	}

	// The function now returns allDIs which is []DiskInfo, suitable for the UI
	return allDIs
}

// Helper function to collect memory information
func collectMemoryInfo() MemoryInfo {
	var topMemory []ProcessInfo
	var alarmMsg string
	var issueMsg string

	memInfo := MemoryInfo{
		Limit: WinHealthConfig.Ram_Limit,
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
	memInfo.Exceeded = virtualMemory.UsedPercent > WinHealthConfig.Ram_Limit // For UI

	// Integrated Alarm and Redmine Logic from RamUsage()
	ramLimit := WinHealthConfig.Ram_Limit // This is memInfo.Limit

	if virtualMemory.UsedPercent > ramLimit && WinHealthConfig.Top_Processes.Ram_enabled {
		if len(allProcesses) <= 0 {
			allProcesses, err = GetTopProcesses()
			if err != nil {
				log.Error().Err(err).Msg("Error getting top processes")
			}
		}
		if WinHealthConfig.Top_Processes.Ram_enabled {
			topMemory = getTopProcessesBy(allProcesses, WinHealthConfig.Top_Processes.Ram_processes, func(p1, p2 *ProcessInfo) bool {
				return p1.MemoryPercent > p2.MemoryPercent
			})
		}
	}
	if len(topMemory) > 0 {
		processTable := FormatProcessesToMarkdown(topMemory)
		alarmMsg = "RAM usage limit has exceeded " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "% (Current: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64) + "%)\n\n" + processTable
		issueMsg = "Hafıza kullanımı: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64) + "%\n Hafıza limiti: " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "%\n\n" + processTable
	} else {
		alarmMsg = "RAM usage limit has exceeded " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "% (Current: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64) + "%)"
		issueMsg = "Hafıza kullanımı: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64) + "%\n Hafıza limiti: " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "%"
	}

	if virtualMemory.UsedPercent > ramLimit {
		common.AlarmCheckDown("ram", alarmMsg, false, "", "")
		issues.CheckDown("ram", common.Config.Identifier+" için hafıza kullanımı %"+strconv.FormatFloat(ramLimit, 'f', 0, 64)+" üstüne çıktı", issueMsg, false, 0)
	} else {
		common.AlarmCheckUp("ram", "RAM usage went below "+strconv.FormatFloat(ramLimit, 'f', 0, 64)+"% (Current: "+strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64)+"%)", false)
		issues.CheckUp("ram", common.Config.Identifier+" için hafıza kullanımı %"+strconv.FormatFloat(ramLimit, 'f', 0, 64)+" altına düştü")
	}

	return memInfo
}

// Helper function to collect system load information
func collectSystemLoad() SystemLoadInfo {
	loadInfo := SystemLoadInfo{
		Multiplier: WinHealthConfig.Load.Issue_Multiplier,
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

	// On Windows, LoadAvg is often 0 (Processor Queue Length).
	// Fallback to CPU Usage % approximation: Load = (CPU% / 100) * CPUCount
	if runtime.GOOS == "windows" && loadInfo.Load1 == 0 {
		cpuPercent, err := GetCPUPercent()
		if err == nil {
			approxLoad := (cpuPercent / 100.0) * float64(cpuCount)
			loadInfo.Load1 = approxLoad
			loadInfo.Load5 = approxLoad
			loadInfo.Load15 = approxLoad
			// Update the loadAvg struct too if we pass it around (SysLoad)
			loadAvg.Load1 = approxLoad
			loadAvg.Load5 = approxLoad
			loadAvg.Load15 = approxLoad
		}
	}

	// Check if load exceeds the limit
	limit := float64(cpuCount) * WinHealthConfig.Load.Issue_Multiplier
	loadInfo.Exceeded = loadAvg.Load5 > limit

	// Trigger sysload alerting via the dedicated function
	// SysLoad re-fetches load.Avg(), so we need to update it to use the new logic or pass the value.
	// However, SysLoad is a separate function. Let's update SysLoad in sysload.go as well.
	SysLoad(loadAvg, cpuCount)

	return loadInfo
}

func GetTopProcesses() ([]ProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to get processes: %w", err)
	}

	for _, p := range procs {
		p.CPUPercent()
	}
	time.Sleep(1 * time.Second)

	var processInfos []ProcessInfo
	for _, p := range procs {
		cpuPercent, err := p.CPUPercent()
		if err != nil {
			continue
		}

		memPercent, err := p.MemoryPercent()
		if err != nil {
			continue
		}

		if cpuPercent == 0 && memPercent == 0 {
			continue
		}

		cmd, _ := p.Name()
		user, _ := p.Username()

		processInfos = append(processInfos, ProcessInfo{
			PID:           p.Pid,
			Command:       cmd,
			Username:      user,
			CPUPercent:    cpuPercent,
			MemoryPercent: memPercent,
		})
	}

	return processInfos, nil
}

func getTopProcessesBy(processes []ProcessInfo, count int, less func(p1, p2 *ProcessInfo) bool) []ProcessInfo {
	procCopy := make([]ProcessInfo, len(processes))
	copy(procCopy, processes)

	sort.Slice(procCopy, func(i, j int) bool {
		return less(&procCopy[i], &procCopy[j])
	})

	if len(procCopy) > count {
		return procCopy[:count]
	}
	return procCopy
}

func FormatProcessesToMarkdown(processes []ProcessInfo) string {
	var sb strings.Builder

	sb.WriteString("| PID | User | Command | CPU (%) | Memory (%) |\n")
	sb.WriteString("|---|---|---|---|---|\n")

	for _, p := range processes {
		fmt.Fprintf(&sb, "| %d | %s | %s | %.2f | %.2f |\n",
			p.PID,
			p.Username,
			p.Command,
			p.CPUPercent,
			p.MemoryPercent,
		)
	}

	return sb.String()
}

// collectWindowsServicesInfo collects Windows services information
func collectWindowsServicesInfo() []WindowsServiceInfo {
	services, err := GetWindowsServices()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get windows services")
		return []WindowsServiceInfo{}
	}

	var filteredServices []WindowsServiceInfo

	// Default behavior: Include if Running OR Failed, unless filtering is enabled
	for _, svc := range services {
		// Filter by status if config is present and filtering is enabled
		if WinHealthConfig.Services.Enabled {
			// Check if specifically excluded
			if common.IsInArray(svc.Name, WinHealthConfig.Services.Exclude) {
				continue
			}

			// Check if specifically included (overrides status checks)
			if len(WinHealthConfig.Services.Include) > 0 {
				if common.IsInArray(svc.Name, WinHealthConfig.Services.Include) {
					filteredServices = append(filteredServices, svc)
					continue
				}
				// If specific include list exists and not in it, skip unless we want to fallthrough to status
				// Usually whitelist means ONLY these. Let's assume whitelist overrides everything else.
				continue
			}

			// Check status
			if len(WinHealthConfig.Services.Status) > 0 {
				if common.IsInArray(svc.Status, WinHealthConfig.Services.Status) {
					filteredServices = append(filteredServices, svc)
				}
				continue
			}
		}

		// Default behavior (if not enabled or no specific filters):
		// GetWindowsServices already returns Running or Failed (Auto Start but Stopped)
		// So we just add it.
		filteredServices = append(filteredServices, svc)
	}

	return filteredServices
}
