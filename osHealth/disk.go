// This file implements disk usage monitoring functionality
//
// It provides functions to:
// - Analyze disk partitions
// - Create tables for exceeded and normal partitions
// - Generate alerts for disk usage
//
// The main functions are:
// - analyzeDiskPartitions(): Analyzes disk partitions and returns exceeded and all parts
// - createExceededTable(): Creates a table for partitions that exceeded the limit
// - createNormalTable(): Creates a table for all partitions when none exceed the limit
// - DiskUsage(): Analyzes disk usage and sends the results to redmine
package osHealth

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/olekukonko/tablewriter"
	"github.com/shirou/gopsutil/v4/disk"
)

// analyzeDiskPartitions analyzes the disk partitions and returns the exceeded and all parts
// exceededParts: partitions that are exceeded the limit
// allParts: all partitions
func analyzeDiskPartitions(diskPartitions []disk.PartitionStat) ([][]string, [][]string) {
	var exceededParts, allParts [][]string

	for _, partition := range diskPartitions {

		if !slices.Contains(OsHealthConfig.Filesystems, partition.Fstype) {
			continue
		}

		usage, err := disk.Usage(partition.Mountpoint)

		if err != nil {
			common.LogError("An error occurred while fetching disk usage for " + partition.Mountpoint + "\n" + err.Error())
			continue
		}

		if usage.UsedPercent > OsHealthConfig.Part_use_limit {
			common.PrettyPrint("Disk usage at "+partition.Mountpoint, common.Fail+" more than "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"%", usage.UsedPercent, true, false, false, 0)
			exceededParts = append(exceededParts, []string{strconv.FormatFloat(usage.UsedPercent, 'f', 0, 64), common.ConvertBytes(usage.Used), common.ConvertBytes(usage.Total), partition.Device, partition.Mountpoint})
		} else {
			common.PrettyPrint("Disk usage at "+partition.Mountpoint, common.Green+" less than "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"%", usage.UsedPercent, true, false, false, 0)
		}
		allParts = append(allParts, []string{strconv.FormatFloat(usage.UsedPercent, 'f', 0, 64), common.ConvertBytes(usage.Used), common.ConvertBytes(usage.Total), partition.Device, partition.Mountpoint})
	}

	return exceededParts, allParts
}

// createExceededTable creates a table for partitions that exceeded the limit
func createExceededTable(exceededParts [][]string) (string, string) {
	output := &strings.Builder{}
	table := tablewriter.NewWriter(output)
	table.SetHeader([]string{"%", "Used", "Total", "Partition", "Mount Point"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(exceededParts)
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
func createNormalTable(allParts [][]string) (string, string) {
	output := &strings.Builder{}
	table := tablewriter.NewWriter(output)
	table.SetHeader([]string{"%", "Used", "Total", "Partition", "Mount Point"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(allParts)
	table.Render()

	tableOnly := output.String()
	fullMsg := "All partitions are now under the limit of " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "%" + "\n\n" + tableOnly

	return fullMsg, tableOnly
}

// DiskUsage analyzes the disk usage and sends the results to redmine
func DiskUsage() {
	common.SplitSection("Disk Usage")

	diskPartitions, err := disk.Partitions(false)
	if err != nil {
		common.LogError("An error occurred while fetching disk partitions\n" + err.Error())
		return
	}

	exceededParts, allParts := analyzeDiskPartitions(diskPartitions)

if len(exceededParts) > 0 {
	fullMsg, tableOnly := createExceededTable(exceededParts)
	subject := common.Config.Identifier + " için disk doluluk seviyesi %" + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + " üstüne çıktı"

// This handles interval-based creation/update attempts via Redmine
issues.CheckDown("disk", subject, tableOnly, false, 0)

// Attempt to get the Redmine issue ID associated with the 'disk' service.
// issues.Show reads the log file managed by CheckDown/CheckUp/Create/Close.
id := issues.Show("disk")

// Check if we got a valid ID.
if id != "" {
// ID exists, append the Redmine link to the alarm message.
fullMsg = fullMsg + "\n\n" + "Redmine Issue: " + common.Config.Redmine.Url + "/issues/" + id
common.AlarmCheckUp("disk_redmineissue", "Redmine issue exists for disk usage", false) // Clear any previous "creation failed" alarm
} else {
// ID is empty. This could be because CheckDown's interval hasn't passed yet,
// or there was an issue communicating with Redmine during CheckDown or Show.
// Do not log a separate "redmine issue creation failed" alarm here.
// Just proceed with the standard disk alarm without the Redmine link.
common.LogDebug("osHealth/disk.go: issues.Show(\"disk\") returned empty. Proceeding without Redmine link in alarm.")
// Optional: We could potentially still log a specific failure if needed, but avoiding disk_redmineissue alarm as requested.
// common.AlarmCheckDown("disk_redmine_lookup_failed", "Could not retrieve Redmine issue ID for disk usage", false, "", "")
}

// Always send the standard disk alarm, with or without the Redmine link included in fullMsg.
common.AlarmCheckDown("disk", fullMsg, false, "", "")

} else {
// Disk usage is normal, close the Redmine issue if it exists.
fullMsg, tableOnly := createNormalTable(allParts) // Regenerate tableOnly for CheckUp message
common.AlarmCheckUp("disk", fullMsg, false)       // Clear the standard disk alarm
issues.CheckUp("disk", common.Config.Identifier+" için bütün disk bölümleri "+strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64)+"% altına indi, kapatılıyor."+"\n\n"+tableOnly)
common.AlarmCheckUp("disk_redmineissue", "Disk usage normal, clearing any Redmine issue creation failure alarm", false) // Also clear the specific failure alarm if it existed
}
}
