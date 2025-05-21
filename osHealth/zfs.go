// This file implements ZFS pool monitoring functionality
//
// It provides functions to:
// - Monitor ZFS pools health status
// - Create tables for ZFS pool information
// - Try to clear degraded pools
//
// The main functions are:
// - ZFSHealth(): Monitors ZFS pools health and reports issues
// - createZFSPoolsTable(): Creates a formatted table of ZFS pool information
// - tryToClearPools(): Attempts to clear errors on degraded ZFS pools
package osHealth

import (
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/olekukonko/tablewriter"
)

// ZFSHealth checks the health status of ZFS pools and reports any issues
// It replicates the functionality of the zfs-health-check Ansible playbook
func ZFSHealth() []ZFSPoolInfo {
	// Skip if ZFS is not in OsHealthConfig.Filesystems
	if !slices.Contains(OsHealthConfig.Filesystems, "zfs") {
		// Silently skip ZFS health check if not configured
		return nil
	}

	// Check if zpool command exists
	if _, err := exec.LookPath("zpool"); err != nil {
		// Silently fail if zpool command is not found
		return nil
	}

	// Get ZFS pool status
	cmd := exec.Command("zpool", "status", "-x")
	output, err := cmd.CombinedOutput()

	// Handle the case where zpool status errors out but ZFS is still installed
	// (e.g., no pools exist, or we don't have permission to check pools)
	if err != nil {
		common.PrettyPrintStr("ZFS Pool Status", false, "Unable to check ZFS pools status: "+strings.TrimSpace(string(output)))
		return nil
	}

	poolStatusOutput := strings.TrimSpace(string(output))
	common.PrettyPrintStr("ZFS Pool Status", true, poolStatusOutput)

	// Get detailed pool information
	cmd = exec.Command("zpool", "list", "-H", "-o", "name,health,used,size,scan")
	output, err = cmd.CombinedOutput()
	if err != nil {
		common.LogError("An error occurred while fetching ZFS pool list\n" + err.Error())
		return nil
	}

	// Parse pool information and identify degraded pools
	var degradedPools []string
	var poolsInfo [][]string
	var zfsPools []ZFSPoolInfo

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	// Skip if no output (no pools)
	if len(lines) == 1 && lines[0] == "" {
		common.PrettyPrintStr("ZFS Pools", true, "No ZFS pools found")
		return nil
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		poolName := fields[0]
		poolHealth := fields[1]
		poolUsed := fields[2]
		poolTotal := fields[3]
		poolScan := fields[4]

		// Calculate used percentage
		usedBytes, _ := strconv.ParseInt(poolUsed, 10, 64)
		totalBytes, _ := strconv.ParseInt(poolTotal, 10, 64)
		usedPct := float64(usedBytes) / float64(totalBytes) * 100

		// Create ZFSPoolInfo for UI
		zfsPool := ZFSPoolInfo{
			Name:       poolName,
			Status:     poolHealth,
			Used:       common.ConvertBytes(uint64(usedBytes)),
			Total:      common.ConvertBytes(uint64(totalBytes)),
			UsedPct:    usedPct,
			ScanStatus: poolScan,
		}
		zfsPools = append(zfsPools, zfsPool)

		poolsInfo = append(poolsInfo, []string{poolName, poolHealth})

		if poolHealth != "ONLINE" {
			degradedPools = append(degradedPools, poolName)
			// Add visual feedback for degraded pools
			common.PrettyPrintStr("Pool "+poolName, false, poolHealth)
		} else {
			common.PrettyPrintStr("Pool "+poolName, true, poolHealth)
		}
	}

	if len(degradedPools) > 0 {
		// Create table with degraded pools
		table := createZFSPoolsTable(poolsInfo)

		// Try to clear the degraded pools
		common.LogInfo("Found degraded ZFS pools, attempting to clear them...")

		// Call the function without saving the result
		tryToClearPools(degradedPools)

		// Get status after clearing
		cmd = exec.Command("zpool", "status", "-x")
		output, err := cmd.CombinedOutput()
		if err != nil {
			common.LogError("An error occurred while fetching ZFS pool status after clearing\n" + err.Error())
		}

		statusAfterClearing := strings.TrimSpace(string(output))
		isHealthyAfterClearing := statusAfterClearing == "all pools are healthy"

		if isHealthyAfterClearing {
			// Pools are healthy now
			message := "ZFS pools have been restored to healthy state after clearing.\n\n" + table
			common.AlarmCheckUp("zfs_health", message, false)
			issues.CheckUp("zfs_health", common.Config.Identifier+" için ZFS pool(lar) sağlıklı duruma getirildi.\n\n"+table)
		} else {
			// Pools are still degraded
			message := "ZFS pools are still in degraded state after clearing attempt.\n\n" +
				"Current status:\n" + statusAfterClearing + "\n\n" +
				"Detailed pool information:\n" + table

			issues.CheckDown("zfs_health", common.Config.Identifier+" için ZFS pool(lar) sağlıklı değil", table, false, 0)

			// Create redmine issue if it doesn't exist
			id := issues.Show("zfs_health")
			if id == "" {
				common.AlarmCheckDown("zfs_health_redmineissue", "Redmine issue could not be created for ZFS health", false, "", "")
				common.AlarmCheckDown("zfs_health", message, false, "", "")
			} else {
				common.AlarmCheckUp("zfs_health_redmineissue", "Redmine issue has been created for ZFS health", false)
				message = message + "\n\n" + "Redmine Issue: " + common.Config.Redmine.Url + "/issues/" + id
				common.AlarmCheckDown("zfs_health", message, false, "", "")
			}
		}
	} else {
		// All pools are healthy, create informational table
		if len(poolsInfo) > 0 {
			table := createZFSPoolsTable(poolsInfo)
			message := "All ZFS pools are healthy.\n\n" + table
			common.AlarmCheckUp("zfs_health", message, false)
		} else {
			common.AlarmCheckUp("zfs_health", "No ZFS pools found or all are healthy.", false)
		}

		// Close any existing issues
		issues.CheckUp("zfs_health", common.Config.Identifier+" için tüm ZFS poolları sağlıklı durumda.")
	}

	return zfsPools
}

// createZFSPoolsTable creates a formatted table of ZFS pool information
func createZFSPoolsTable(poolsInfo [][]string) string {
	output := &strings.Builder{}
	table := tablewriter.NewWriter(output)
	table.SetHeader([]string{"Pool Name", "Health Status"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(poolsInfo)
	table.Render()

	return output.String()
}

// tryToClearPools attempts to clear errors on degraded ZFS pools
// Returns the list of pools that were successfully cleared
func tryToClearPools(degradedPools []string) []string {
	var clearedPools []string

	for _, pool := range degradedPools {
		common.LogInfo("Attempting to clear ZFS pool: " + pool)
		cmd := exec.Command("zpool", "clear", "-F", pool)
		if err := cmd.Run(); err != nil {
			common.LogError("Failed to clear ZFS pool " + pool + ": " + err.Error())
			continue
		}

		clearedPools = append(clearedPools, pool)
		common.LogInfo("Successfully cleared ZFS pool: " + pool)
	}

	return clearedPools
}
