package osHealth

import (
    "os"
    "slices"
    "strconv"
    "strings"
    "github.com/olekukonko/tablewriter"
    "github.com/shirou/gopsutil/v4/disk"
    "github.com/monobilisim/monokit/common"
)

func DiskUsage() {
    common.SplitSection("Disk Usage")

    var exceededParts [][]string
    var allParts [][]string
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
            common.PrettyPrint("Disk usage at " + partition.Mountpoint, common.Fail + " more than " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "%", usage.UsedPercent, true, false)
            exceededParts = append(exceededParts, []string{strconv.FormatFloat(usage.UsedPercent, 'f', 0, 64), common.ConvertBytes(usage.Used), common.ConvertBytes(usage.Total), partition.Device, partition.Mountpoint})
        } else {
            common.PrettyPrint("Disk usage at " + partition.Mountpoint, common.Green + " less than " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "%", usage.UsedPercent, true, false)
        }
        allParts = append(allParts, []string{strconv.FormatFloat(usage.UsedPercent, 'f', 0, 64), common.ConvertBytes(usage.Used), common.ConvertBytes(usage.Total), partition.Device, partition.Mountpoint})
    }

    if len(exceededParts) > 0 {
        output := &strings.Builder{}
        table := tablewriter.NewWriter(output)
        table.SetHeader([]string{"%", "Used", "Total", "Partition", "Mount Point"})
        table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
        table.SetCenterSeparator("|")
        table.AppendBulk(exceededParts)
        table.Render()
        msg := "Partition usage level has exceeded to " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "% " + "for the following partitions;\n\n" + output.String()
        msg = strings.Replace(msg, "\n", `\n`, -1)
        
        // Check if file exists 
        if _, err := os.Stat(common.TmpDir + "/" + common.Config.Identifier + "_disk_usage.txt"); os.IsNotExist(err) {
            os.WriteFile(common.TmpDir + "/" + common.Config.Identifier + "_disk_usage.txt", []byte(msg), 0644)
        } else {
            // Read file
            fileContent, _ := os.ReadFile(common.TmpDir + "/" + common.Config.Identifier + "_disk_usage.txt")

            // Check if the message is the same
            if string(fileContent) != msg {
                common.RedmineUpdate("disk", output.String())
            }
            
            // Write msg to file
            os.WriteFile(common.TmpDir + "/" + common.Config.Identifier + "_disk_usage.txt", []byte(msg), 0644)
        }


        common.RedmineCreate("disk", common.Config.Identifier + " - Diskteki bir (ya da birden fazla) bölümün doluluk seviyesi %"+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+" üstüne çıktı", output.String())
        
        id := common.RedmineShow("disk")

        if id == "" {
            common.AlarmCheckDown("disk_redmineissue", "Redmine issue could not be created for disk usage")
            common.AlarmCheckDown("disk", msg)
        } else {
            common.AlarmCheckUp("disk_redmineissue", "Redmine issue has been created for disk usage")
            common.AlarmCheckDown("disk", msg + "\n\n" + "Redmine Issue: " + common.Config.Redmine.Url + "/issues/" + id)
        }

    } else {
        output := &strings.Builder{}
        table := tablewriter.NewWriter(output)
        table.SetHeader([]string{"%", "Used", "Total", "Partition", "Mount Point"})
        table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
        table.SetCenterSeparator("|")
        table.AppendBulk(allParts)
        table.Render()
        msg := "All partitions are now under the limit of " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "%" + "\n\n" + output.String()
        msg = strings.Replace(msg, "\n", `\n`, -1)
        
        common.AlarmCheckUp("disk", msg)
        common.RedmineClose("disk", common.Config.Identifier + " - Bütün disk bölümleri "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"% altına indi, kapatılıyor." + "\n\n" + output.String())
    }
}
    

