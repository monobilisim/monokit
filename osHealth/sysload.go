// This file implements system load monitoring functionality
//
// It provides functions to:
// - Check system load
// - Generate alerts for system load
//
// The main functions are:
// - SysLoad(): Checks system load and generates alerts
package osHealth

import (
	"fmt"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
)

// checkLoadIssues checks the system load and creates issues to redmine
func checkLoadIssues(loadAvg *load.AvgStat, loadLimitIssue float64, cpuCount int) {
	if loadAvg.Load1 > loadLimitIssue {
		issues.CheckDown("sysload",
			fmt.Sprintf("%s için sistem yükü %.2f üstüne çıktı", common.Config.Identifier, loadLimitIssue),
			fmt.Sprintf("CPU sayısı: %d\nSistem yükü: %.2f\nLimit: %.2f", cpuCount, loadAvg.Load1, loadLimitIssue),
			true, OsHealthConfig.Load.Issue_Interval)
	} else {
		issues.CheckUp("sysload",
			fmt.Sprintf("Sistem yükü artık %.2f üstünde değil,\nSistem yükü: %.2f\nLimit: %.2f\nCPU sayısı: %d",
				loadLimitIssue, loadAvg.Load1, loadLimitIssue, cpuCount))
	}
}

// checkLoadAlarms checks the system load and generates alarms
func checkLoadAlarms(loadAvg *load.AvgStat, loadLimit float64) {
	if loadAvg.Load1 > loadLimit {
		common.PrettyPrint("System Load",
			fmt.Sprintf("%s more than %.2f",
				common.Fail, loadLimit),
			loadAvg.Load1, false, true, false, 0)

		common.AlarmCheckDown("sysload",
			fmt.Sprintf("System load has been more than %.2f for the last %.2f minutes (%.2f)",
				loadLimit, common.Config.Alarm.Interval, loadAvg.Load1), false, "", "")
	} else {
		common.PrettyPrint("System Load",
			fmt.Sprintf("%s less than %.2f", common.Green, loadLimit),
			loadAvg.Load1, false, true, false, 0)

		common.AlarmCheckUp("sysload",
			fmt.Sprintf("System load is now less than %.2f (%.2f)",
				loadLimit, loadAvg.Load1), false)
	}
}

// SysLoad analyzes the system load and sends the results to redmine and generates alarms
// It gets the cpu count, calculates the load limit and checks the system load
func SysLoad() {
	cpuCount, err := cpu.Counts(true)
	if err != nil {
		common.LogError(err.Error())
		return
	}

	loadLimit := float64(cpuCount) * OsHealthConfig.Load.Limit_Multiplier
	loadLimitIssue := float64(cpuCount) * OsHealthConfig.Load.Issue_Multiplier

	loadAvg, err := load.Avg()
	if err != nil {
		common.LogError(err.Error())
		return
	}

	checkLoadIssues(loadAvg, loadLimitIssue, cpuCount)
	checkLoadAlarms(loadAvg, loadLimit)
}
