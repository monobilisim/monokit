package osHealth

import (
    "strconv"
    "github.com/shirou/gopsutil/v4/cpu"
    "github.com/shirou/gopsutil/v4/load"
    "github.com/monobilisim/monokit/common"
    issues "github.com/monobilisim/monokit/common/redmine/issues"
)

func SysLoad() {
    cpuCount, err := cpu.Counts(true)
    
    if err != nil {
        common.LogError(err.Error())
        return
    }
   
    loadLimitIssueMultiplier := OsHealthConfig.Load.Limit_Multiplier * OsHealthConfig.Load.Issue_Multiplier

    loadLimit := float64(cpuCount) * OsHealthConfig.Load.Limit_Multiplier

    loadLimitIssue := loadLimit * loadLimitIssueMultiplier

    loadAvg, err := load.Avg()

    if err != nil {
        common.LogError(err.Error())
        return
    }
    
    if loadAvg.Load1 > loadLimitIssue {
		issues.CheckDown("sysload", common.Config.Identifier + " için sistem yükü " + strconv.FormatFloat(loadLimitIssue, 'f', 2, 64) + " üstüne çıktı", "CPU sayısı: " + strconv.Itoa(cpuCount) + "\n Sistem yükü: " + strconv.FormatFloat(loadAvg.Load1, 'f', 2, 64) + "\n Limit: " + strconv.FormatFloat(loadLimitIssue, 'f', 2, 64), true, OsHealthConfig.Load.Issue_Interval)
    } else {
		issues.CheckUp("sysload", "Sistem yükü artık " + strconv.FormatFloat(loadLimitIssue, 'f', 2, 64) + " üstünde değil, Sistem yükü: " + strconv.FormatFloat(loadAvg.Load1, 'f', 2, 64) + "\n Limit: " + strconv.FormatFloat(loadLimitIssue, 'f', 2, 64) + "\n CPU sayısı: " + strconv.Itoa(cpuCount))
    }

    if loadAvg.Load1 > loadLimit {
        common.PrettyPrint("System Load", common.Fail + " more than " + strconv.FormatFloat(loadLimit, 'f', 2, 64) + "%", loadAvg.Load1, false, true, false, 0)
        common.AlarmCheckDown("sysload", "System load is now more than " + strconv.FormatFloat(loadLimit, 'f', 2, 64) + "%, System load: " + strconv.FormatFloat(loadAvg.Load1, 'f', 2, 64), false)
    } else {
        common.PrettyPrint("System Load", common.Green + " less than " + strconv.FormatFloat(loadLimit, 'f', 2, 64) + "%", loadAvg.Load1, false, true, false, 0)
        common.AlarmCheckUp("sysload", "System load is now less than " + strconv.FormatFloat(loadLimit, 'f', 2, 64) + "%, System load: " + strconv.FormatFloat(loadAvg.Load1, 'f', 2, 64), false)
    }
}
