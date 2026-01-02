// This file implements system load monitoring functionality
//
// It provides functions to:
// - Check system load
// - Generate alerts for system load
//
// The main functions are:
// - SysLoad(): Checks system load and generates alerts
package winHealth

import (
	"fmt"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/load"
)

// checkLoadIssues checks the system load and creates issues to redmine
func checkLoadIssues(loadAvg *load.AvgStat, loadLimitIssue float64, cpuCount int, topCPU []ProcessInfo) {
	var msg string
	if len(topCPU) > 0 {
		processTable := FormatProcessesToMarkdown(topCPU)
		msg = fmt.Sprintf("CPU sayısı: %d\nSistem yükü: %.2f\nLimit: %.2f\n%s", cpuCount, loadAvg.Load1, loadLimitIssue, processTable)
	} else {
		msg = fmt.Sprintf("CPU sayısı: %d\nSistem yükü: %.2f\nLimit: %.2f", cpuCount, loadAvg.Load1, loadLimitIssue)
	}
	if loadAvg.Load1 > loadLimitIssue {
		issues.CheckDown("sysload",
			fmt.Sprintf("%s için sistem yükü %.2f üstüne çıktı", common.Config.Identifier, loadLimitIssue),
			msg,
			true, WinHealthConfig.Load.Issue_Interval)
	} else {
		issues.CheckUp("sysload",
			fmt.Sprintf("Sistem yükü artık %.2f üstünde değil,\nSistem yükü: %.2f\nLimit: %.2f\nCPU sayısı: %d",
				loadLimitIssue, loadAvg.Load1, loadLimitIssue, cpuCount))
	}
}

// checkLoadAlarms checks the system load and generates alarms
func checkLoadAlarms(loadAvg *load.AvgStat, loadLimit float64, topCPU []ProcessInfo) {
	var msg string
	if len(topCPU) > 0 {
		processTable := FormatProcessesToMarkdown(topCPU)
		msg = fmt.Sprintf("System load has been more than %.2f for the last %.2f minutes (%.2f)\n\n%s",
			loadLimit, common.Config.Alarm.Interval, loadAvg.Load1, processTable)
	} else {
		msg = fmt.Sprintf("System load has been more than %.2f for the last %.2f minutes (%.2f)",
			loadLimit, common.Config.Alarm.Interval, loadAvg.Load1)
	}

	if loadAvg.Load1 > loadLimit {
		common.AlarmCheckDown("sysload", msg, false, "", "")
	} else {
		common.AlarmCheckUp("sysload",
			fmt.Sprintf("System load is now less than %.2f (%.2f)",
				loadLimit, loadAvg.Load1), false)
	}
}

func SysLoad(loadAvg *load.AvgStat, cpuCount int) {
	var topCPU []ProcessInfo

	loadLimit := float64(cpuCount) * WinHealthConfig.Load.Limit_Multiplier
	loadLimitIssue := float64(cpuCount) * WinHealthConfig.Load.Issue_Multiplier

	if (loadAvg.Load1 > loadLimitIssue || loadAvg.Load1 > loadLimit) && WinHealthConfig.Top_Processes.Load_enabled {
		if len(allProcesses) <= 0 {
			var err error
			allProcesses, err = GetTopProcesses()
			if err != nil {
				log.Error().Err(err).Msg("Error getting top processes")
			}
		}
		if WinHealthConfig.Top_Processes.Load_enabled {
			topCPU = getTopProcessesBy(allProcesses, WinHealthConfig.Top_Processes.Load_processes, func(p1, p2 *ProcessInfo) bool {
				return p1.CPUPercent > p2.CPUPercent
			})
		}
	}

	checkLoadIssues(loadAvg, loadLimitIssue, cpuCount, topCPU)
	checkLoadAlarms(loadAvg, loadLimit, topCPU)
}
