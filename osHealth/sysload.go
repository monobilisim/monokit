package osHealth

import (
    "strconv"
    "github.com/shirou/gopsutil/v4/cpu"
    "github.com/shirou/gopsutil/v4/load"
    "github.com/monobilisim/monokit/common"
)

func SysLoad() {
    cpuCount, err := cpu.Counts(true)
    
    if err != nil {
        common.LogError(err.Error())
        return
    }
    
    loadLimit := float64(cpuCount) * OsHealthConfig.Load.Limit_Multiplier

    loadAvg, err := load.Avg()

    if err != nil {
        common.LogError(err.Error())
        return
    }

    if loadAvg.Load1 > loadLimit {
        common.PrettyPrint("System Load", common.Fail + " more than " + strconv.FormatFloat(loadLimit, 'f', 2, 64) + "%", loadAvg.Load1, false, true)
    } else {
        common.PrettyPrint("System Load", common.Green + " less than " + strconv.FormatFloat(loadLimit, 'f', 2, 64) + "%", loadAvg.Load1, false, true)
    }
}
