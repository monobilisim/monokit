package osHealth

import (
    "slices"
    "strconv"
    "github.com/shirou/gopsutil/v4/disk"
    "github.com/monobilisim/mono-go/common"
)

type Part struct {
    Mountpoint string
    UsedPercent float64
}

func DiskUsage() {
    common.SplitSection("Disk Usage")

    exceededParts := []Part{}
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
            exceededParts = append(exceededParts, Part{Mountpoint: partition.Mountpoint, UsedPercent: usage.UsedPercent})
        } else {
            common.PrettyPrint("Disk usage at " + partition.Mountpoint, common.Green + " less than " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', -1, 64) + "%", usage.UsedPercent, true)
        }
    }

    //if len(exceededParts) > 0 {
    //    for _, exceededPart := range exceededParts {
            //fmt.Printf("Mountpoint: %s, Used Percent: %f\n", exceededPart.Mountpoint, exceededPart.UsedPercent)
    //    }
    //}
}
    

