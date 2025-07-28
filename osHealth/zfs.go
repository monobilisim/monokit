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
	"fmt"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/olekukonko/tablewriter"
	"github.com/rs/zerolog/log"
)

// parseBytesFromString converts a string like "1.23G", "500M", "2T" to uint64 bytes.
// It handles common suffixes (K, M, G, T, P, E) case-insensitively.
// ZFS output often uses single letter suffixes and sometimes decimals.
func parseBytesFromString(s string) (uint64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	var multiplier float64 = 1
	var suffix string
	var numberPart string

	// Find the last character that is not a digit or '.'
	lastChar := s[len(s)-1]
	if lastChar >= 'A' && lastChar <= 'Z' {
		suffix = string(lastChar)
		numberPart = s[:len(s)-1]
	} else {
		numberPart = s // Assume just bytes if no suffix
	}

	switch suffix {
	case "K": // Kilobytes
		multiplier = 1024
	case "M": // Megabytes
		multiplier = 1024 * 1024
	case "G": // Gigabytes
		multiplier = 1024 * 1024 * 1024
	case "T": // Terabytes
		multiplier = 1024 * 1024 * 1024 * 1024
	case "P": // Petabytes
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	case "E": // Exabytes
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	case "": // Bytes
		multiplier = 1
	default:
		// If suffix is present but not recognized, assume it's part of the number (e.g. zpool might output '3.67G')
		// In this case, the previous logic correctly split, but the suffix was not standard.
		// Let's try to parse numberPart as float64 and if suffix was not empty but unrecognized, error out.
		// However, zpool list uses single letters. If 'B' for Bytes is used, it's handled by suffix = ""
		if suffix != "" { // A suffix was found but not matched
			return 0, fmt.Errorf("unrecognized suffix: %s in %s", suffix, s)
		}
	}

	// Handle cases like "3.67G" - numberPart will be "3.67"
	val, err := strconv.ParseFloat(numberPart, 64)
	if err != nil {
		// If it's not a float, try to parse as int (for cases like "1024" without suffix)
		intVal, intErr := strconv.ParseUint(numberPart, 10, 64)
		if intErr != nil {
			return 0, fmt.Errorf("invalid number format: %s in %s (float parse err: %v, int parse err: %v)", numberPart, s, err, intErr)
		}
		val = float64(intVal)
	}

	return uint64(val * multiplier), nil
}

// createExceededZFSDatasetTable creates a table for datasets that exceeded the usage limit
func createExceededZFSDatasetTable(exceededDatasets []ZFSDatasetInfo) (string, string) {
	var tableData [][]string
	for _, d := range exceededDatasets {
		tableData = append(tableData, []string{
			strconv.FormatFloat(math.Floor(d.UsedPct), 'f', 0, 64),
			d.Used,
			d.Avail,
			d.Name,
		})
	}
	output := &strings.Builder{}
	table := tablewriter.NewWriter(output)
	table.SetHeader([]string{"%", "Used", "Avail", "Dataset"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(tableData)
	table.Render()

	tableOnly := output.String()
	fullMsg := "ZFS dataset usage level has exceeded " + strconv.FormatFloat(OsHealthConfig.Part_use_limit, 'f', 0, 64) + "% for the following datasets;\n\n" + tableOnly

	return fullMsg, tableOnly
}

// collectZFSDatasetInfo parses `zfs list -H -p -o name,used,avail` and returns []ZFSDatasetInfo
func collectZFSDatasetInfo() []ZFSDatasetInfo {
	if !slices.Contains(OsHealthConfig.Filesystems, "zfs") {
		return nil
	}
	if _, err := exec.LookPath("zfs"); err != nil {
		return nil
	}
	cmd := exec.Command("zfs", "list", "-H", "-p", "-o", "name,used,avail")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Str("component", "osHealth").Str("operation", "collectZFSDatasetInfo").Msg("Failed to run zfs list: " + err.Error())
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var datasets []ZFSDatasetInfo
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		usedStr := fields[1]
		availStr := fields[2]
		used, err1 := strconv.ParseUint(usedStr, 10, 64)
		avail, err2 := strconv.ParseUint(availStr, 10, 64)
		if err1 != nil || err2 != nil {
			log.Error().Str("dataset", name).Msg("Failed to parse used/avail for dataset")
			continue
		}
		total := used + avail
		usedPct := 0.0
		if total > 0 {
			usedPct = float64(used) * 100 / float64(total)
		}
		datasets = append(datasets, ZFSDatasetInfo{
			Name:    name,
			Used:    common.ConvertBytes(used),
			Avail:   common.ConvertBytes(avail),
			UsedPct: usedPct,
		})
	}
	return datasets
}

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
		return nil
	}

	// Get detailed pool information
	cmd = exec.Command("zpool", "list", "-H", "-o", "name,health,allocated,size")
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Str("component", "osHealth").Str("operation", "ZFSHealth").Str("action", "zpool_list_failed").Msg("An error occurred while fetching ZFS pool list. Error: " + err.Error() + ". Output: " + strings.TrimSpace(string(output)))
		return nil
	}

	// Parse pool information and identify degraded pools
	var degradedPools []string
	var poolsInfo [][]string
	var zfsPools []ZFSPoolInfo

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	// Skip if no output (no pools)
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		poolName := fields[0]
		poolHealth := fields[1]
		poolUsedStr := fields[2]  // Keep as string for display
		poolTotalStr := fields[3] // Keep as string for display

		// Parse strings to numeric bytes for percentage calculation
		usedNumericBytes, errUsed := parseBytesFromString(poolUsedStr)
		if errUsed != nil {
			log.Error().Err(errUsed).Str("component", "osHealth").Str("operation", "ZFSHealth").Str("action", "parse_bytes_from_string_failed").Str("pool_name", poolName).Str("pool_used_str", poolUsedStr).Msg("Error parsing used bytes for pool")
		}
		totalNumericBytes, errTotal := parseBytesFromString(poolTotalStr)
		if errTotal != nil {
			log.Error().Err(errTotal).Str("component", "osHealth").Str("operation", "ZFSHealth").Str("action", "parse_bytes_from_string_failed").Str("pool_name", poolName).Str("pool_total_str", poolTotalStr).Msg("Error parsing total bytes for pool")
		}

		usedPct := 0.0
		if totalNumericBytes > 0 {
			usedPct = float64(usedNumericBytes) / float64(totalNumericBytes) * 100
		}

		// Create ZFSPoolInfo for UI
		zfsPool := ZFSPoolInfo{
			Name:    poolName,
			Status:  poolHealth,
			Used:    poolUsedStr,  // Use the original string from zpool list
			Total:   poolTotalStr, // Use the original string from zpool list
			UsedPct: usedPct,
		}
		zfsPools = append(zfsPools, zfsPool)

		poolsInfo = append(poolsInfo, []string{poolName, poolHealth})

		if poolHealth != "ONLINE" {
			degradedPools = append(degradedPools, poolName)
		}
	}

	if len(degradedPools) > 0 {
		// Create table with degraded pools (based on initial state) // This line will be removed
		// initialTable := createZFSPoolsTable(poolsInfo) // This line will be removed

		log.Info().Msg("Found degraded ZFS pools, attempting to clear them...")
		tryToClearPools(degradedPools)

		// Re-fetch and re-parse pool information AFTER clearing attempt
		cmd = exec.Command("zpool", "list", "-H", "-o", "name,health,allocated,size")
		outputAfterClear, errAfterClear := cmd.CombinedOutput()
		var updatedPoolsInfo [][]string
		var updatedZfsPools []ZFSPoolInfo // This will be the final return value for UI

		if errAfterClear != nil {
			log.Error().Err(errAfterClear).Str("component", "osHealth").Str("operation", "ZFSHealth").Str("action", "zpool_list_after_clear_failed").Msg("An error occurred while fetching ZFS pool list after clearing. Error: " + errAfterClear.Error() + ". Output: " + strings.TrimSpace(string(outputAfterClear)))
			// Fallback to original zfsPools if re-fetch fails, but table might be stale
			updatedPoolsInfo = poolsInfo
			updatedZfsPools = zfsPools
		} else {
			linesAfterClear := strings.Split(strings.TrimSpace(string(outputAfterClear)), "\n")
			if !(len(linesAfterClear) == 1 && linesAfterClear[0] == "") { // Check if not empty output
				for _, line := range linesAfterClear {
					if line == "" {
						continue
					}
					fields := strings.Fields(line)
					if len(fields) < 4 {
						continue
					}
					poolName := fields[0]
					poolHealth := fields[1]
					poolUsedStrAfterClear := fields[2]  // Keep as string for display
					poolTotalStrAfterClear := fields[3] // Keep as string for display

					// Parse strings to numeric bytes for percentage calculation
					usedNumericBytesAfterClear, errUsedAC := parseBytesFromString(poolUsedStrAfterClear)
					if errUsedAC != nil {
						log.Error().Err(errUsedAC).Str("component", "osHealth").Str("operation", "ZFSHealth").Str("action", "parse_bytes_from_string_failed").Str("pool_name", poolName).Str("pool_used_str_after_clear", poolUsedStrAfterClear).Msg("Error parsing used bytes after clear for pool")
					}
					totalNumericBytesAfterClear, errTotalAC := parseBytesFromString(poolTotalStrAfterClear)
					if errTotalAC != nil {
						log.Error().Err(errTotalAC).Str("component", "osHealth").Str("operation", "ZFSHealth").Str("action", "parse_bytes_from_string_failed").Str("pool_name", poolName).Str("pool_total_str_after_clear", poolTotalStrAfterClear).Msg("Error parsing total bytes after clear for pool")
					}

					usedPctAfterClear := 0.0
					if totalNumericBytesAfterClear > 0 {
						usedPctAfterClear = float64(usedNumericBytesAfterClear) / float64(totalNumericBytesAfterClear) * 100
					}

					updatedZfsPools = append(updatedZfsPools, ZFSPoolInfo{
						Name:    poolName,
						Status:  poolHealth,
						Used:    poolUsedStrAfterClear,  // Use the original string
						Total:   poolTotalStrAfterClear, // Use the original string
						UsedPct: usedPctAfterClear,
					})
					updatedPoolsInfo = append(updatedPoolsInfo, []string{poolName, poolHealth})
				}
			} else {
				// No pools after clear, or error parsing. Fallback.
				log.Info().Msg("No ZFS pools found or error parsing after clearing attempt. Using initial pool data for table.")
				updatedPoolsInfo = poolsInfo
				updatedZfsPools = zfsPools
			}
		}
		zfsPools = updatedZfsPools                          // Update zfsPools to be returned by the function for UI
		finalTable := createZFSPoolsTable(updatedPoolsInfo) // Table for alarm message based on updated state

		// Get status after clearing using zpool status -x for definitive health check
		cmd = exec.Command("zpool", "status", "-x")
		output, err := cmd.CombinedOutput() // output and err are re-used from initial zpool status check
		if err != nil {
			log.Error().Err(err).Msg("An error occurred while fetching ZFS pool status after clearing\n")
			// If zpool status -x fails here, we might not be able to determine actual health
			// Consider original status or a generic error message
		}

		statusAfterClearing := strings.TrimSpace(string(output))
		isHealthyAfterClearing := statusAfterClearing == "all pools are healthy"

		if isHealthyAfterClearing {
			message := "ZFS pools have been restored to healthy state after clearing.\n\n" + finalTable
			common.AlarmCheckUp("zfs_health", message, false)
			issues.CheckUp("zfs_health", common.Config.Identifier+" için ZFS pool(lar) sağlıklı duruma getirildi.\n\n"+finalTable)
		} else {
			message := "ZFS pools are still in degraded state after clearing attempt.\n\n" +
				"Current status:\n" + statusAfterClearing + "\n\n" +
				"Detailed pool information:\n" + finalTable // Use the updated table

			issues.CheckDown("zfs_health", common.Config.Identifier+" için ZFS pool(lar) sağlıklı değil", finalTable, false, 0)

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
		// All pools are healthy from the start
		if len(poolsInfo) > 0 {
			table := createZFSPoolsTable(poolsInfo)
			message := "All ZFS pools are healthy.\n\n" + table
			common.AlarmCheckUp("zfs_health", message, false)
		} else {
			common.AlarmCheckUp("zfs_health", "No ZFS pools found or all are healthy.", false)
		}
		issues.CheckUp("zfs_health", common.Config.Identifier+" için tüm ZFS poolları sağlıklı durumda.")
	}

	return zfsPools // Return the zfsPools (updated if clear was attempted)
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
		log.Info().Str("component", "osHealth").Str("operation", "tryToClearPools").Str("action", "zpool_clear_attempt").Str("pool", pool).Msg("Attempting to clear ZFS pool")
		cmd := exec.Command("zpool", "clear", "-F", pool)
		if err := cmd.Run(); err != nil {
			log.Error().Err(err).Str("component", "osHealth").Str("operation", "tryToClearPools").Str("action", "zpool_clear_failed").Str("pool", pool).Msg("Failed to clear ZFS pool")
			continue
		}

		clearedPools = append(clearedPools, pool)
		log.Info().Str("component", "osHealth").Str("operation", "tryToClearPools").Str("action", "zpool_clear_success").Str("pool", pool).Msg("Successfully cleared ZFS pool")
	}

	return clearedPools
}
