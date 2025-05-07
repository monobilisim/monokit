// This file implements the WAL-G verification functionality
//
// It provides the following functions:
// - handleWalCheck(): Handles the check and notification logic for WAL-G checks
// - WalgVerify(): Runs the WAL-G verification command and handles the results
package pgsqlHealth

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
)

// WalGData holds information about WAL-G backup status
type WalGData struct {
	Status      string
	LastBackup  string
	BackupCount int
	Healthy     bool
}

// handleWalCheck handles the check and notification logic for WAL-G checks
// and updates the provided WalGData with the check results
func handleWalCheck(walGData *WalGData, checkType, status string) {
	checkID := "wal_g_" + checkType + "_check"
	displayName := "WAL-G " + checkType + " check"

	// Update the WalGData status
	if walGData.Status == "" {
		walGData.Status = status
	} else {
		walGData.Status = walGData.Status + ", " + checkType + ": " + status
	}

	// Update health status
	if status != "OK" {
		walGData.Healthy = false
		common.AlarmCheckDown(checkID, displayName+" failed, "+checkType+" check status: "+status, false, "", "")
		issues.CheckDown(checkID, "WAL-G "+checkType+" kontrolü başarısız oldu", checkType+" durumu: "+status, false, 0)
	} else {
		common.AlarmCheckUp(checkID, displayName+" is now OK", false)
		issues.CheckUp(checkID, "WAL-G "+checkType+" kontrolü başarılı \n "+checkType+" durumu: "+status)
	}
}

// WalgVerify checks the integrity and timeline status of WAL-G backups
// by running 'wal-g wal-verify' command and reporting the results
func WalgVerify() (*WalGData, error) {
	walGData := &WalGData{
		Healthy: true, // Assume healthy until proven otherwise
	}

	var integrityCheck string
	var timelineCheck string
	var err error

	// Verify the WAL-G backups
	cmd := exec.Command("wal-g", "wal-verify", "integrity", "timeline")
	cmd.Stderr = nil
	var out strings.Builder
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("Error executing command: %v", err)
		common.LogError(errMsg)
		walGData.Status = "Error: " + errMsg
		walGData.Healthy = false
		return walGData, err
	}

	for _, line := range strings.Split(string(out.String()), "\n") {
		if strings.Contains(line, "integrity check status:") {
			integrityCheck = strings.Split(line, ": ")[1]
		}
		if strings.Contains(line, "timeline check status:") {
			timelineCheck = strings.Split(line, ": ")[1]
		}
	}

	// Get the last backup info
	cmdBackup := exec.Command("wal-g", "backup-list", "--json")
	var outBackup strings.Builder
	cmdBackup.Stdout = &outBackup
	errBackup := cmdBackup.Run()

	if errBackup == nil {
		// Process backup list output to find the most recent backup
		backupList := outBackup.String()
		if backupList != "" {
			lines := strings.Split(backupList, "\n")
			if len(lines) > 0 {
				// Get most recent backup time from the first backup in the list
				for _, line := range lines {
					if strings.Contains(line, "time") {
						parts := strings.Split(line, "\"time\":\"")
						if len(parts) > 1 {
							timeStr := strings.Split(parts[1], "\"")[0]
							walGData.LastBackup = timeStr

							// Try to parse time and calculate how old the backup is
							t, err := time.Parse(time.RFC3339, timeStr)
							if err == nil {
								duration := time.Since(t)
								hours := int(duration.Hours())
								if hours > 24 {
									walGData.Status += ", Last backup is " + fmt.Sprintf("%d", hours/24) + " days old"
									if hours > 48 {
										walGData.Healthy = false
									}
								}
							}
							break
						}
					}
				}

				// Count the backups
				walGData.BackupCount = len(lines) - 1 // Subtract 1 for empty line at end
			}
		}
	}

	handleWalCheck(walGData, "integrity", integrityCheck)
	handleWalCheck(walGData, "timeline", timelineCheck)

	return walGData, nil
}
