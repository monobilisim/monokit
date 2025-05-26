// This file implements disk usage monitoring functionality
//
// It provides functions to:
// - Analyze disk partitions
// - Create tables for exceeded and normal partitions
//
// The main functions are:
// - analyzeDiskPartitions(): Analyzes disk partitions and returns DiskInfo for exceeded and all parts
// - createExceededTable(): Creates a table for partitions that exceeded the limit
// - createNormalTable(): Creates a table for all partitions when none exceed the limit
package osHealth

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	// issues "github.com/monobilisim/monokit/common/redmine/issues" // No longer directly used here
	"github.com/olekukonko/tablewriter"
	"github.com/shirou/gopsutil/v4/disk"
)

// analyzeDiskPartitions analyzes the disk partitions and returns DiskInfo for exceeded and all parts.
// It now returns []DiskInfo for better data structure.
func analyzeDiskPartitions(diskPartitions []disk.PartitionStat) ([]DiskInfo, []DiskInfo) {
	var exceededDIs, allDIs []DiskInfo

	for _, partition := range diskPartitions {
		// Check if the mountpoint should be excluded
		isExcluded := false
		for _, excludedMountpoint := range OsHealthConfig.Excluded_Mountpoints {
			if strings.HasPrefix(partition.Mountpoint, excludedMountpoint) {
				isExcluded = true
				break
			}
		}
		if isExcluded {
			common.LogDebug("Skipping excluded mountpoint: " + partition.Mountpoint)
			continue
		}

		if !slices.Contains(OsHealthConfig.Filesystems, partition.Fstype) {
			continue
		}

		usage, err := disk.Usage(partition.Mountpoint) // gopsutil disk.Usage
		if err != nil {
			common.LogError("An error occurred while fetching disk usage for " + partition.Mountpoint + "\n" + err.Error())
			continue
		}

		common.LogDebug("usage.Used: " + strconv.FormatUint(usage.Used, 10))
		common.LogDebug("usage.Total: " + strconv.FormatUint(usage.Total, 10))
		common.LogDebug("usage.UsedPercent: " + strconv.FormatFloat(usage.UsedPercent, 'f', 0, 64))

		currentDiskInfo := DiskInfo{ // DiskInfo is from osHealth/ui.go (same package)
			Device:     partition.Device,
			Mountpoint: partition.Mountpoint,
			Used:       common.ConvertBytes(usage.Used),
			Total:      common.ConvertBytes(usage.Total),
			UsedPct:    usage.UsedPercent,
			Fstype:     partition.Fstype,
		}
		allDIs = append(allDIs, currentDiskInfo)

		if usage.UsedPercent > OsHealthConfig.Part_use_limit {
			// common.PrettyPrint("Disk usage at "+partition.Mountpoint, common.Fail+" more than "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"%", usage.UsedPercent, true, false, false, 0)
			exceededDIs = append(exceededDIs, currentDiskInfo)
		} else {
			// common.PrettyPrint("Disk usage at "+partition.Mountpoint, common.Green+" less than "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"%", usage.UsedPercent, true, false, false, 0)
		}
	}
	return exceededDIs, allDIs
}

// createExceededTable creates a table for partitions that exceeded the limit
// It now takes []DiskInfo and converts it internally to [][]string for tablewriter
func createExceededTable(exceededParts []DiskInfo) (string, string) {
	var tableData [][]string
	for _, p := range exceededParts {
		tableData = append(tableData, []string{
			strconv.FormatFloat(p.UsedPct, 'f', 0, 64),
			p.Used,
			p.Total,
			p.Device,
			p.Mountpoint,
		})
	}

	output := &strings.Builder{}
	table := tablewriter.NewWriter(output)
	table.SetHeader([]string{"%", "Used", "Total", "Partition", "Mount Point"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(tableData)
	table.Render()

	tableOnly := output.String()
	fullMsg := "Partition usage level has exceeded to " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "% " + "for the following partitions;\n\n" + tableOnly

	// Write message to file, creating it if it doesn't exist
	err := os.WriteFile(common.TmpDir+"/"+common.Config.Identifier+"_disk_usage.txt", []byte(fullMsg), 0644)
	if err != nil {
		common.LogError("Failed to write disk usage report: " + err.Error())
	}

	return fullMsg, tableOnly
}

// createNormalTable creates a table for all partitions when none exceed the limit
// It now takes []DiskInfo and converts it internally to [][]string for tablewriter
func createNormalTable(allParts []DiskInfo) (string, string) {
	var tableData [][]string
	for _, p := range allParts {
		tableData = append(tableData, []string{
			strconv.FormatFloat(p.UsedPct, 'f', 0, 64),
			p.Used,
			p.Total,
			p.Device,
			p.Mountpoint,
		})
	}

	output := &strings.Builder{}
	table := tablewriter.NewWriter(output)
	table.SetHeader([]string{"%", "Used", "Total", "Partition", "Mount Point"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(tableData)
	table.Render()

	tableOnly := output.String()
	fullMsg := "All partitions are now under the limit of " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "%" + "\n\n" + tableOnly

	return fullMsg, tableOnly
}

// Note: The DiskUsage function has been removed. Its logic will be integrated into collectDiskInfo in main.go.
