package osHealth

import (
    "strconv"
    "github.com/shirou/gopsutil/v4/mem"
    "github.com/monobilisim/monokit/common"
    issues "github.com/monobilisim/monokit/common/redmine/issues"
)


func RamUsage() {
    virtualMemory, err := mem.VirtualMemory()

    if err != nil {
        common.LogError(err.Error())
        return
    }

    ramLimit := OsHealthConfig.Ram_Limit

    if virtualMemory.UsedPercent > ramLimit {
        common.PrettyPrint("RAM Usage", common.Fail + " more than " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "%", virtualMemory.UsedPercent, true, false, false, 0)
        common.AlarmCheckDown("ram", "RAM usage limit has exceeded " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "% (Current: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64) + "%)", false)
        issues.CheckDown("ram", common.Config.Identifier + " için hafıza kullanımı " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "%'nin üstüne çıktı", "Hafıza kullanımı: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64) + "%\n Hafıza limiti: " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "%", false, 0)
    } else {
        common.PrettyPrint("RAM Usage", common.Green + " less than " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "%", virtualMemory.UsedPercent, true, false, false, 0)
        common.AlarmCheckUp("ram", "RAM usage went below " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "% (Current: " + strconv.FormatFloat(virtualMemory.UsedPercent, 'f', 0, 64) + "%)", false)
        issues.CheckUp("ram", common.Config.Identifier + " için hafıza kullanımı " + strconv.FormatFloat(ramLimit, 'f', 0, 64) + "%'nin altına düştü")
    }
}

