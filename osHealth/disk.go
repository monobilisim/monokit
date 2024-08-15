package osHealth

import (
    "slices"
    "strconv"
    "strings"
    "github.com/olekukonko/tablewriter"
    "github.com/shirou/gopsutil/v4/disk"
    "github.com/monobilisim/mono-go/common"
)

func DiskUsage() {
    common.SplitSection("Disk Usage")

    var exceededParts [][]string
    diskPartitions, err := disk.Partitions(false)
    
    if err != nil {
        common.LogError("An error occurred while fetching disk partitions\n" + err.Error())
        return
    }

    for _, partition := range diskPartitions {
        
        if ! slices.Contains(OsHealthConfig.Filesystems, partition.Fstype) {
            continue
        }

        usage, _ := disk.Usage(partition.Mountpoint)
        
        if usage.UsedPercent > OsHealthConfig.Part_use_limit {
            common.PrettyPrint("Disk usage at " + partition.Mountpoint, common.Fail + " more than " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', -1, 64) + "%", usage.UsedPercent, true)
            exceededParts = append(exceededParts, []string{strconv.FormatFloat(usage.UsedPercent, 'f', 2, 64), common.ConvertBytes(usage.Used), common.ConvertBytes(usage.Total), partition.Device, partition.Mountpoint})
        } else {
            common.PrettyPrint("Disk usage at " + partition.Mountpoint, common.Green + " less than " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', -1, 64) + "%", usage.UsedPercent, true)
        }
    }

    if len(exceededParts) > 0 {
        output := &strings.Builder{}
        table := tablewriter.NewWriter(output)
        table.SetHeader([]string{"%", "Used", "Total", "Partition", "Mount Point"})
        table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
        table.SetCenterSeparator("|")
        table.AppendBulk(exceededParts)
        table.Render()
        msg := "Partition usage level has exceeded to " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 2, 64) + "% " + "for the following partitions;\n\n" + output.String()
        msg = strings.Replace(msg, "\n", `\n`, -1)
        common.AlarmCheckDown("disk", msg)
        common.RedmineCreate("disk", Config.Identifier + " - Diskteki bir (ya da birden fazla) bölümün doluluk seviyesi %"+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 2, 64)+" üstüne çıktı", output.String())
    } else { 
        common.AlarmCheckUp("disk", "All partitions are now under the limit of " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 2, 64) + "%")
    }
}
    

