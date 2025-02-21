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

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
)

// handleWalCheck handles the check and notification logic for WAL-G checks
func handleWalCheck(checkType, status string) {
	checkID := "wal_g_" + checkType + "_check"
	displayName := "WAL-G " + checkType + " check"
	
	if status != "OK" {
		common.PrettyPrintStr(displayName, false, "OK")
		common.AlarmCheckDown(checkID, displayName+" failed, "+checkType+" check status: "+status, false, "", "")
		issues.CheckDown(checkID, "WAL-G "+checkType+" kontrolü başarısız oldu", checkType+" durumu: "+status, false, 0)
	} else {
		common.PrettyPrintStr(displayName, true, "OK")
		common.AlarmCheckUp(checkID, displayName+" is now OK", false)
		issues.CheckUp(checkID, "WAL-G "+checkType+" kontrolü başarılı \n "+checkType+" durumu: "+status)
	}
}

// WalgVerify checks the integrity and timeline status of WAL-G backups
// by running 'wal-g wal-verify' command and reporting the results
func WalgVerify() {
	var integrityCheck string
	var timelineCheck string
	var err error
	cmd := exec.Command("wal-g", "wal-verify", "integrity", "timeline")
	cmd.Stderr = nil
	var out strings.Builder
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing command: %v\n", err))
		return
	}

	for _, line := range strings.Split(string(out.String()), "\n") {
		if strings.Contains(line, "integrity check status:") {
			integrityCheck = strings.Split(line, ": ")[1]
		}
		if strings.Contains(line, "timeline check status:") {
			timelineCheck = strings.Split(line, ": ")[1]
		}
	}

	handleWalCheck("integrity", integrityCheck)
	handleWalCheck("timeline", timelineCheck)
} 