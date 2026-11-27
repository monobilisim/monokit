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
    "time"

    "github.com/monobilisim/monokit/common"
    "github.com/monobilisim/monokit/common/healthdb"
    // issues "github.com/monobilisim/monokit/common/redmine/issues" // No longer directly used here
    "github.com/rs/zerolog/log"
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
			log.Debug().Msg("Skipping excluded mountpoint: " + partition.Mountpoint)
			continue
		}

		// Skip ZFS partitions as they are handled separately by dataset checks
		if partition.Fstype == "zfs" {
			log.Debug().Msg("Skipping ZFS partition (handled by dataset checks): " + partition.Mountpoint)
			continue
		}

		if !slices.Contains(OsHealthConfig.Filesystems, partition.Fstype) {
			continue
		}

		usage, err := disk.Usage(partition.Mountpoint) // gopsutil disk.Usage
		if err != nil {
			log.Error().Err(err).Msg("An error occurred while fetching disk usage for " + partition.Mountpoint + "\n")
			continue
		}

		log.Debug().
			Str("component", "osHealth").
			Str("function", "analyzeDiskPartitions").
			Str("mountpoint", partition.Mountpoint).
			Float64("usage_percent", usage.UsedPercent).
			Msg("Disk usage information")

		currentDiskInfo := DiskInfo{ // DiskInfo is from osHealth/ui.go (same package)
			Device:     partition.Device,
			Mountpoint: partition.Mountpoint,
			Used:       common.ConvertBytes(usage.Used),
			Total:      common.ConvertBytes(usage.Total),
			UsedPct:    usage.UsedPercent,
			Fstype:     partition.Fstype,
		}
		allDIs = append(allDIs, currentDiskInfo)

		// Use >= so a partition at exactly the configured limit (e.g. 90.0%)
		// is treated as "exceeded" instead of silently falling into the
		// "all partitions below limit, close Redmine issue" branch.
		wasExceeded := diskExceededState(partition.Mountpoint)
		exceeded := diskExceeded(usage.UsedPercent, OsHealthConfig.Part_use_limit, wasExceeded)
		if exceeded != wasExceeded {
			setDiskExceededState(partition.Mountpoint, exceeded)
		}
		if exceeded {
			exceededDIs = append(exceededDIs, currentDiskInfo)
		}
	}
	return exceededDIs, allDIs
}

// diskHysteresisPct is how far below Part_use_limit an exceeded partition must
// drop before it counts as normal again. Without it a partition hovering at
// the limit (92.96% vs 93%) closes and reopens the alarm and Redmine issue
// every few minutes.
const diskHysteresisPct = 1.0

// diskExceeded reports whether a partition is over the limit. A partition that
// was already over the limit stays so until it drops diskHysteresisPct below it.
func diskExceeded(usedPct, limit float64, wasExceeded bool) bool {
	if usedPct >= limit {
		return true
	}
	return wasExceeded && usedPct >= limit-diskHysteresisPct
}

func diskExceededKey(mountpoint string) string {
	return "disk-exceeded:" + mountpoint
}

// diskExceededState reports whether the previous run counted mountpoint as exceeded.
func diskExceededState(mountpoint string) bool {
	_, _, _, found, err := healthdb.GetJSON("osHealth", diskExceededKey(mountpoint))
	return err == nil && found
}

func setDiskExceededState(mountpoint string, exceeded bool) {
	if exceeded {
		_ = healthdb.PutJSON("osHealth", diskExceededKey(mountpoint), "true", nil, time.Now())
	} else {
		_ = healthdb.Delete("osHealth", diskExceededKey(mountpoint))
	}
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

    tableOnly := renderMarkdownTable([]string{"%", "Used", "Total", "Partition", "Mount Point"}, tableData)
	fullMsg := "Partition usage level has exceeded to " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "% " + "for the following partitions;\n\n" + tableOnly

	// Write message to file, creating it if it doesn't exist
	err := os.WriteFile(common.TmpDir+"/"+common.Config.Identifier+"_disk_usage.txt", []byte(fullMsg), 0644)
	if err != nil {
		log.Error().Err(err).Msg("Failed to write disk usage report: ")
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

    tableOnly := renderMarkdownTable([]string{"%", "Used", "Total", "Partition", "Mount Point"}, tableData)
	fullMsg := "All partitions are now under the limit of " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "%" + "\n\n" + tableOnly

	return fullMsg, tableOnly
}

// Note: The DiskUsage function has been removed. Its logic will be integrated into collectDiskInfo in main.go.
