package osHealth

import (
    "strconv"
    "github.com/shirou/gopsutil/v4/mem"
    "github.com/monobilisim/mono-go/common"
)


func RamUsage() {
    virtualMemory, err := mem.VirtualMemory()

    if err != nil {
        common.LogError(err.Error())
        return
    }

    ramLimit := OsHealthConfig.Ram_Limit

    if virtualMemory.UsedPercent > ramLimit {
        common.PrettyPrint("RAM Usage", common.Fail + " more than " + strconv.FormatFloat(ramLimit, 'f', 2, 64) + "%", virtualMemory.UsedPercent, true)
        common.AlarmCheckDown("ram", "RAM usage limit has exceeded " + strconv.FormatFloat(ramLimit, 'f', 2, 64) + "% (Current: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 2, 64) + "%)")
    } else {
        common.PrettyPrint("RAM Usage", common.Green + " less than " + strconv.FormatFloat(ramLimit, 'f', 2, 64) + "%", virtualMemory.UsedPercent, true)
        common.AlarmCheckUp("ram", "RAM usage went below " + strconv.FormatFloat(ramLimit, 'f', 2, 64) + "% (Current: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 2, 64) + "%)")
    }
}

