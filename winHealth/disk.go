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
package winHealth

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	// issues "github.com/monobilisim/monokit/common/redmine/issues" // No longer directly used here
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/disk"
)

// volumeKind classifies a volume by the kind of device backing it, so that
// disk usage checks can be limited to storage the host actually owns.
type volumeKind int

const (
	// volumeUnknown is used when the drive type cannot be determined; such
	// volumes are monitored, so an unexpected value never silences a check.
	volumeUnknown volumeKind = iota
	volumeFixed
	volumeRemovable
	volumeCdrom
	volumeNetwork
	volumeRamdisk
)

// skipVolume reports whether a volume must be left out of the disk usage
// checks entirely - both the reported table and the alarms - along with the
// reason, for logging. Keeping the two in sync is deliberate: a volume shown
// as full but never alarmed on is worse than one that is not shown at all.
func skipVolume(kind volumeKind, opts []string, config WinHealth) (bool, string) {
	switch kind {
	case volumeCdrom:
		if !config.Disk.Monitor_Cdrom {
			return true, "CD-ROM or mounted disk image"
		}
	case volumeRemovable:
		if !config.Disk.Monitor_Removable {
			return true, "removable drive"
		}
	case volumeNetwork:
		if config.Disk.Monitor_Network != nil && !*config.Disk.Monitor_Network {
			return true, "network drive"
		}
	}

	// A read-only volume reports 100% usage whatever it holds, and nothing
	// can be freed on it. This also covers install images mounted as a
	// read-only NTFS/ReFS virtual disk rather than as a CD-ROM drive.
	if !config.Disk.Monitor_Readonly && slices.Contains(opts, "ro") {
		return true, "read-only volume"
	}

	return false, ""
}

// analyzeDiskPartitions analyzes the disk partitions and returns DiskInfo for exceeded and all parts.
// It now returns []DiskInfo for better data structure.
func analyzeDiskPartitions(diskPartitions []disk.PartitionStat) ([]DiskInfo, []DiskInfo) {
	var exceededDIs, allDIs []DiskInfo

	for _, partition := range diskPartitions {
		// Check if the mountpoint should be excluded
		isExcluded := false
		for _, excludedMountpoint := range WinHealthConfig.Excluded_Mountpoints {
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

		// Skip volumes that are not real storage of this host, such as the
		// CD-ROM drive an installer mounts its downloaded image on. They sit
		// at 100% usage by definition, so listing them is misleading and no
		// alarm raised for them could ever be resolved.
		if skip, reason := skipVolume(getVolumeKind(partition.Mountpoint), partition.Opts, WinHealthConfig); skip {
			log.Debug().
				Str("mountpoint", partition.Mountpoint).
				Str("fstype", partition.Fstype).
				Strs("opts", partition.Opts).
				Str("reason", reason).
				Msg("Skipping volume that is not monitored for disk usage")
			continue
		}

		if len(WinHealthConfig.Filesystems) > 0 && !slices.Contains(WinHealthConfig.Filesystems, partition.Fstype) {
			log.Debug().Msg("Skipping filesystem: " + partition.Fstype + " (not in allowed list)")
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

		// Mirror of osHealth/disk.go: use >= so a partition at exactly the
		// configured limit counts as "exceeded" instead of triggering the
		// bare-else close branch with a misleading "below limit" message.
		if usage.UsedPercent >= WinHealthConfig.Part_use_limit {
			exceededDIs = append(exceededDIs, currentDiskInfo)
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
			p.Mountpoint,
		})
	}

	tableOnly := renderMarkdownTable([]string{"%", "Used", "Total", "Drive"}, tableData)
	fullMsg := "Partition usage level has exceeded to " + strconv.FormatFloat(WinHealthConfig.Part_use_limit, 'f', 0, 64) + "% " + "for the following partitions;\n\n" + tableOnly

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
			p.Mountpoint,
		})
	}

	tableOnly := renderMarkdownTable([]string{"%", "Used", "Total", "Drive"}, tableData)
	fullMsg := "All partitions are now under the limit of " + strconv.FormatFloat(WinHealthConfig.Part_use_limit, 'f', 0, 64) + "%" + "\n\n" + tableOnly

	return fullMsg, tableOnly
}

// Note: The DiskUsage function has been removed. Its logic will be integrated into collectDiskInfo in main.go.
