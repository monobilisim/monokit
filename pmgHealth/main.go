//go:build linux

package pmgHealth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os" // Import os for file checks
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	mail "github.com/monobilisim/monokit/common/mail"
	redmineIssues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// DetectPmg checks if Proxmox Mail Gateway seems to be installed.
// It looks for the pmgversion command and the pmgproxy service.
func DetectPmg() bool {
	// 1. Check for pmgversion command
	if _, err := exec.LookPath("pmgversion"); err != nil {
		log.Debug().Msg("pmgHealth auto-detection failed: 'pmgversion' command not found in PATH.")
		return false
	}
	log.Debug().Msg("pmgHealth auto-detection: 'pmgversion' command found.")

	// 2. Check for /etc/pmg directory
	if _, err := os.Stat("/etc/pmg"); os.IsNotExist(err) {
		log.Debug().Msg("pmgHealth auto-detection failed: '/etc/pmg' directory not found.")
		return false
	}
	log.Debug().Msg("pmgHealth auto-detection: '/etc/pmg' directory found.")

	// 3. Check if pmgproxy service exists/is active (using common function)
	// We can just check for one key service. If pmgproxy is there, it's likely PMG.
	if !common.SystemdUnitExists("pmgproxy.service") {
		log.Debug().Msg("pmgHealth auto-detection failed: 'pmgproxy.service' systemd unit not found.")
		return false
	}
	log.Debug().Msg("pmgHealth auto-detection: 'pmgproxy.service' systemd unit found.")

	log.Debug().Msg("pmgHealth auto-detected successfully.")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "pmgHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectPmg, // Add the auto-detect function
	})
}

var MailHealthConfig mail.MailHealth

// CheckPmgServices checks the status of PMG services and returns a map of service statuses
func CheckPmgServices(skipOutput bool) map[string]bool {
	pmgServices := []string{"pmgproxy.service", "pmg-smtp-filter.service", "postfix@-.service"}
	serviceStatus := make(map[string]bool)

	for _, service := range pmgServices {
		isActive := common.SystemdUnitActive(service)
		serviceStatus[service] = isActive

		if isActive {
			common.AlarmCheckUp(service, service+" is working again", false)
		} else {
			common.AlarmCheckDown(service, service+" is not running", false, "", "")
		}
	}

	return serviceStatus
}

// PostgreSQLStatus checks if PostgreSQL is running and returns its status
func PostgreSQLStatus(skipOutput bool) bool {
	cmd := exec.Command("pg_isready", "-q")
	err := cmd.Run()
	isRunning := err == nil

	if !isRunning {
		common.AlarmCheckDown("postgres", "PostgreSQL is not running", false, "", "")
	} else {
		common.AlarmCheckUp("postgres", "PostgreSQL is now running", false)
	}

	return isRunning
}

// QueuedMessages checks the number of queued mail messages and returns queue information
func QueuedMessages(skipOutput bool) (int, int, bool) {
	// Execute the mailq command
	cmd := exec.Command("mailq")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	if err != nil {
		log.Error().Err(err).Msg("Error running mailq: ")
		common.AlarmCheckDown("mailq_run", "Error running mailq: "+err.Error(), false, "", "")
		return 0, MailHealthConfig.Pmg.Queue_Limit, false
	} else {
		common.AlarmCheckUp("mailq_run", "mailq command executed successfully", false)
	}

	// Compile a regex to match lines that start with A-F or 0-9
	re := regexp.MustCompile("^[A-F0-9]")

	// Split the output into lines and count matches
	lines := bytes.Split(out.Bytes(), []byte("\n"))
	count := 0
	for _, line := range lines {
		if re.Match(line) {
			count++
		}
	}

	isHealthy := count < MailHealthConfig.Pmg.Queue_Limit

	if isHealthy {
		common.AlarmCheckUp("queued_msg", "Number of queued messages is acceptable - "+strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit), false)
	} else {
		common.AlarmCheckDown("queued_msg", "Number of queued messages is above limit - "+strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit), false, "", "")
	}

	return count, MailHealthConfig.Pmg.Queue_Limit, isHealthy
}

// getMailStats retrieves current mail statistics from PMG for the given time range
func getMailStats(startUnix, endUnix int64) (sent, received int, err error) {
	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "get_mail_stats").
		Int64("start_time", startUnix).
		Int64("end_time", endUnix).
		Msg("Fetching mail statistics from PMG")

	// Check if pmgsh command exists
	if _, err := exec.LookPath("pmgsh"); err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "pmgsh_check").
			Msg("pmgsh command not found")
		return 0, 0, fmt.Errorf("pmgsh command not found: %w", err)
	}

	// Execute pmgsh get /statistics/mail command with time range
	cmd := exec.Command("pmgsh", "get", "/statistics/mail",
		"--starttime", strconv.FormatInt(startUnix, 10),
		"--endtime", strconv.FormatInt(endUnix, 10))

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "pmgsh_execute").
			Str("stderr", stderr.String()).
			Msg("Error executing pmgsh command")
		return 0, 0, fmt.Errorf("error executing pmgsh: %w, stderr: %s", err, stderr.String())
	}

	// Parse JSON response
	var stats PmgMailStatistics
	if err := json.Unmarshal(out.Bytes(), &stats); err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "json_parse").
			Str("output", out.String()).
			Msg("Error parsing pmgsh JSON response")
		return 0, 0, fmt.Errorf("error parsing JSON response: %w", err)
	}

	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "stats_parsed").
		Int("count_out", stats.CountOut).
		Int("count_in", stats.CountIn).
		Int("total_count", stats.Count).
		Msg("Mail statistics parsed successfully")

	return stats.CountOut, stats.CountIn, nil
}

// statsCheck24h performs 24-hour mail statistics comparison and alarm checking
func statsCheck24h() error {
	// Check if statistics alarm is enabled
	if !MailHealthConfig.Pmg.Email_monitoring.Enabled {
		log.Debug().
			Str("component", "pmgHealth").
			Str("action", "stats_check_24h").
			Msg("Mail statistics alarm is disabled, skipping check")
		return nil
	}

	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "stats_check_24h").
		Msg("Starting 24-hour mail statistics check")

	// Calculate time ranges
	now := time.Now().Unix()
	last24hStart := now - 24*3600 // 24 hours ago
	prev24hStart := now - 48*3600 // 48 hours ago

	// Get mail statistics for both periods
	lastSent, lastReceived, err := getMailStats(last24hStart, now)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "stats_check_24h").
			Msg("Failed to get last 24h statistics")
		return fmt.Errorf("failed to get last 24h statistics: %w", err)
	}

	prevSent, prevReceived, err := getMailStats(prev24hStart, last24hStart)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "stats_check_24h").
			Msg("Failed to get previous 24h statistics")
		return fmt.Errorf("failed to get previous 24h statistics: %w", err)
	}

	// Calculate totals
	lastTotal := lastSent + lastReceived
	prevTotal := prevSent + prevReceived

	// Avoid division by zero - use minimum of 1 for comparison
	prevTotalForComparison := prevTotal
	if prevTotalForComparison == 0 {
		prevTotalForComparison = 1
	}

	// Calculate threshold
	threshold := MailHealthConfig.Pmg.Email_monitoring.Threshold_factor.Daily
	thresholdValue := float64(prevTotalForComparison) * threshold

	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "stats_check_24h").
		Int("last_24h_sent", lastSent).
		Int("last_24h_received", lastReceived).
		Int("last_24h_total", lastTotal).
		Int("prev_24h_sent", prevSent).
		Int("prev_24h_received", prevReceived).
		Int("prev_24h_total", prevTotal).
		Float64("threshold_factor", threshold).
		Float64("threshold_value", thresholdValue).
		Msg("Mail statistics comparison")

	// Check if traffic exceeded threshold
	if float64(lastTotal) >= thresholdValue {
		// Traffic is abnormally high - trigger alarm
		message := fmt.Sprintf("High mail traffic detected: %d messages (last 24h) vs %d messages (previous 24h), threshold: %.1fx = %.0f",
			lastTotal, prevTotal, threshold, thresholdValue)

		common.AlarmCheckDown("pmg_mail_stats", message, false, "", "")

		// Create Redmine issue for threshold violation
		createRedmineIssueForThreshold("24 saat", lastTotal, prevTotal, threshold, thresholdValue)

		log.Warn().
			Str("component", "pmgHealth").
			Str("action", "stats_check_24h").
			Str("alarm_message", message).
			Msg("Mail traffic alarm triggered")
	} else {
		// Traffic is normal - clear any existing alarm
		message := fmt.Sprintf("Mail traffic normal: %d messages (last 24h) vs %d messages (previous 24h), threshold: %.1fx = %.0f",
			lastTotal, prevTotal, threshold, thresholdValue)

		common.AlarmCheckUp("pmg_mail_stats", message, false)

		// Close Redmine issue when traffic returns to normal
		closeRedmineIssueForThreshold("24 saat", lastTotal, prevTotal, threshold)

		log.Debug().
			Str("component", "pmgHealth").
			Str("action", "stats_check_24h").
			Str("status_message", message).
			Msg("Mail traffic within normal range")
	}

	return nil
}

// statsCheck1h performs 1-hour mail statistics comparison and alarm checking
func statsCheck1h() error {
	// Check if statistics alarm is enabled
	if !MailHealthConfig.Pmg.Email_monitoring.Enabled {
		log.Debug().
			Str("component", "pmgHealth").
			Str("action", "stats_check_1h").
			Msg("Mail statistics alarm is disabled, skipping check")
		return nil
	}

	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "stats_check_1h").
		Msg("Starting 1-hour mail statistics check")

	// Calculate time ranges
	now := time.Now().Unix()
	last1hStart := now - 3600   // 1 hour ago
	prev1hStart := now - 2*3600 // 2 hours ago

	// Get mail statistics for both periods
	lastSent, lastReceived, err := getMailStats(last1hStart, now)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "stats_check_1h").
			Msg("Failed to get last 1h statistics")
		return fmt.Errorf("failed to get last 1h statistics: %w", err)
	}

	prevSent, prevReceived, err := getMailStats(prev1hStart, last1hStart)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "stats_check_1h").
			Msg("Failed to get previous 1h statistics")
		return fmt.Errorf("failed to get previous 1h statistics: %w", err)
	}

	// Calculate totals
	lastTotal := lastSent + lastReceived
	prevTotal := prevSent + prevReceived

	// Avoid division by zero - use minimum of 1 for comparison
	prevTotalForComparison := prevTotal
	if prevTotalForComparison == 0 {
		prevTotalForComparison = 1
	}

	// Calculate threshold
	threshold := MailHealthConfig.Pmg.Email_monitoring.Threshold_factor.Hourly
	thresholdValue := float64(prevTotalForComparison) * threshold

	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "stats_check_1h").
		Int("last_1h_sent", lastSent).
		Int("last_1h_received", lastReceived).
		Int("last_1h_total", lastTotal).
		Int("prev_1h_sent", prevSent).
		Int("prev_1h_received", prevReceived).
		Int("prev_1h_total", prevTotal).
		Float64("threshold_factor", threshold).
		Float64("threshold_value", thresholdValue).
		Msg("Mail statistics comparison")

	// Check if traffic exceeded threshold
	if float64(lastTotal) >= thresholdValue {
		// Traffic is abnormally high - trigger alarm
		message := fmt.Sprintf("High mail traffic detected: %d messages (last 1h) vs %d messages (previous 1h), threshold: %.1fx = %.0f",
			lastTotal, prevTotal, threshold, thresholdValue)

		common.AlarmCheckDown("pmg_mail_stats_1h", message, false, "", "")

		// Create Redmine issue for threshold violation
		//createRedmineIssueForThreshold("1 saat", lastTotal, prevTotal, threshold, thresholdValue)

		log.Warn().
			Str("component", "pmgHealth").
			Str("action", "stats_check_1h").
			Str("alarm_message", message).
			Msg("Mail traffic alarm triggered")
	} else {
		// Traffic is normal - clear any existing alarm
		message := fmt.Sprintf("Mail traffic normal: %d messages (last 1h) vs %d messages (previous 1h), threshold: %.1fx = %.0f",
			lastTotal, prevTotal, threshold, thresholdValue)

		common.AlarmCheckUp("pmg_mail_stats_1h", message, false)

		// Close Redmine issue when traffic returns to normal
		closeRedmineIssueForThreshold("1 saat", lastTotal, prevTotal, threshold)

		log.Debug().
			Str("component", "pmgHealth").
			Str("action", "stats_check_1h").
			Str("status_message", message).
			Msg("Mail traffic within normal range")
	}

	return nil
}

// createRedmineIssueForThreshold creates a Redmine issue for mail threshold violations
func createRedmineIssueForThreshold(timeframe string, currentTotal, prevTotal int, threshold float64, thresholdValue float64) string {
	subject := fmt.Sprintf("%s için PMG yüksek mail trafiği uyarısı", common.Config.Identifier)
	message := fmt.Sprintf(`Proxmox Mail Gateway üzerinde yüksek mail trafiği tespit edildi.

**Trafik Detayları:**
- Zaman Çerçevesi: %s
- Mevcut Trafik: %d mesaj
- Önceki Dönem: %d mesaj
- Eşik Faktörü: %.1fx
- Eşik Değeri: %.0f mesaj
- Aşım Miktarı: %d mesaj (%%%.1f)`,
		timeframe,
		currentTotal,
		prevTotal,
		threshold,
		thresholdValue,
		currentTotal-int(thresholdValue),
		(float64(currentTotal)/thresholdValue-1)*100)

	// Use redmine's CheckDown to create or update issue with proper timing controls
	serviceName := fmt.Sprintf("pmg_mail_traffic_%s", strings.ReplaceAll(timeframe, " ", "_"))
	redmineIssues.CheckDown(serviceName, subject, message, false, 0)

	// Get the created issue ID using the Show function from redmine issues package
	issueId := redmineIssues.Show(serviceName)

	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "create_redmine_issue").
		Str("timeframe", timeframe).
		Str("issue_id", issueId).
		Int("current_total", currentTotal).
		Int("prev_total", prevTotal).
		Float64("threshold", threshold).
		Msg("Created Redmine issue for mail threshold violation")

	return issueId
}

// closeRedmineIssueForThreshold closes/updates a Redmine issue when traffic returns to normal
func closeRedmineIssueForThreshold(timeframe string, currentTotal, prevTotal int, threshold float64) {
	if !common.Config.Redmine.Enabled {
		return
	}

	serviceName := fmt.Sprintf("pmg_mail_traffic_%s", strings.ReplaceAll(timeframe, " ", "_"))
	message := fmt.Sprintf(`Mail trafiği normal seviyelere döndü.

**Trafik Detayları:**
- Mevcut Trafik: %d mesaj
- Önceki Dönem: %d mesaj
- Eşik Faktörü: %.1fx`,
		currentTotal,
		prevTotal,
		threshold)

	redmineIssues.CheckUp(serviceName, message)

	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "close_redmine_issue").
		Str("timeframe", timeframe).
		Int("current_total", currentTotal).
		Int("prev_total", prevTotal).
		Float64("threshold", threshold).
		Msg("Closed Redmine issue - mail traffic returned to normal")
}

// CheckPmgcmSyncDaily runs the daily pmgcm sync check
// This should be called once per day via cron or systemd timer, not in regular health checks
// pmgcm sync exits with code 0 on master nodes and non-zero on non-master nodes
func CheckPmgcmSyncDaily(skipOutput bool) (bool, string) {
	// Check if pmgcm command exists
	if _, err := exec.LookPath("pmgcm"); err != nil {
		log.Debug().
			Str("component", "pmgHealth").
			Str("action", "pmgcm_sync_check").
			Msg("pmgcm command not found, assuming single node setup")
		return true, "pmgcm not available"
	}

	// First check pmgcm status to see if cluster is defined
	statusCmd := exec.Command("pmgcm", "status")
	var statusOut bytes.Buffer
	var statusStderr bytes.Buffer
	statusCmd.Stdout = &statusOut
	statusCmd.Stderr = &statusStderr

	statusErr := statusCmd.Run()
	statusOutput := strings.TrimSpace(statusOut.String())

	// Check if status output contains "no cluster defined"
	if strings.Contains(strings.ToLower(statusOutput), "no cluster defined") {
		log.Debug().
			Str("component", "pmgHealth").
			Str("action", "pmgcm_status_check").
			Str("status_output", statusOutput).
			Msg("No cluster defined, skipping pmgcm sync")
		return true, "no cluster defined, sync skipped"
	}

	// If status command failed, log it but continue with sync attempt
	if statusErr != nil {
		log.Warn().
			Err(statusErr).
			Str("component", "pmgHealth").
			Str("action", "pmgcm_status_check").
			Str("stdout", statusOutput).
			Str("stderr", statusStderr.String()).
			Msg("pmgcm status command failed, proceeding with sync attempt")
	}

	// Run pmgcm sync
	cmd := exec.Command("pmgcm", "sync")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		// Exit code != 0 - send alarm with stdout + stderr
		errorMsg := fmt.Sprintf("pmgcm sync failed (exit code != 0)\nstdout: %s\nstderr: %s", out.String(), stderr.String())
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "pmgcm_sync").
			Str("stdout", out.String()).
			Str("stderr", stderr.String()).
			Msg("pmgcm sync command failed")
		common.AlarmCheckDown("pmgcm_sync", errorMsg, true, "", "")
		return false, errorMsg
	}

	// Exit code 0 - success
	log.Debug().
		Str("component", "pmgHealth").
		Str("action", "pmgcm_sync").
		Str("stdout", out.String()).
		Msg("pmgcm sync completed successfully")
	common.AlarmCheckUp("pmgcm_sync", "pmgcm sync completed successfully", true)
	return true, "pmgcm sync successful"
}

// CheckPmgHealth performs all PMG health checks and returns a data structure with the results
func CheckPmgHealth(skipOutput bool) *PmgHealthData {
	data := &PmgHealthData{
		IsHealthy: true, // Start with assumption it's healthy
		Services:  make(map[string]bool),
	}

	// Check PMG services
	data.Services = CheckPmgServices(skipOutput)

	// Check PostgreSQL status
	data.PostgresRunning = PostgreSQLStatus(skipOutput)

	// Check queued messages
	queueCount, queueLimit, queueHealthy := QueuedMessages(skipOutput)
	data.QueueStatus.Count = queueCount
	data.QueueStatus.Limit = queueLimit
	data.QueueStatus.IsHealthy = queueHealthy

	// Check cluster sync status - only run at 00:00 (midnight)
	now := time.Now()
	if now.Hour() == 0 && now.Minute() == 0 {
		syncHealthy, syncStatus := CheckPmgcmSyncDaily(skipOutput)
		data.ClusterSyncStatus.IsMaster = syncHealthy && syncStatus == "pmgcm sync successful"
		data.ClusterSyncStatus.SyncHealthy = syncHealthy
		data.ClusterSyncStatus.Status = syncStatus
		if !syncHealthy {
			data.ClusterSyncStatus.LastError = syncStatus
			// Don't mark overall health as unhealthy for pmgcm sync issues
		}
	} else {
		// Not midnight - set default status
		data.ClusterSyncStatus.IsMaster = false
		data.ClusterSyncStatus.SyncHealthy = true
		data.ClusterSyncStatus.Status = "Waiting for midnight check"
	}

	// Get mail statistics if enabled
	data.MailStats.Enabled = MailHealthConfig.Pmg.Email_monitoring.Enabled
	if data.MailStats.Enabled {
		// Calculate time ranges
		now := time.Now().Unix()
		last24hStart := now - 24*3600 // 24 hours ago
		prev24hStart := now - 48*3600 // 48 hours ago
		last1hStart := now - 3600     // 1 hour ago
		prev1hStart := now - 2*3600   // 2 hours ago

		// Get 24-hour mail statistics
		lastSent, lastReceived, err := getMailStats(last24hStart, now)
		if err == nil {
			data.MailStats.Last24hSent = lastSent
			data.MailStats.Last24hReceived = lastReceived
			data.MailStats.Last24hTotal = lastSent + lastReceived

			prevSent, prevReceived, err := getMailStats(prev24hStart, last24hStart)
			if err == nil {
				data.MailStats.Prev24hSent = prevSent
				data.MailStats.Prev24hReceived = prevReceived
				data.MailStats.Prev24hTotal = prevSent + prevReceived

				// Calculate 24h threshold and status
				data.MailStats.Threshold24h = MailHealthConfig.Pmg.Email_monitoring.Threshold_factor.Daily
				prevTotalForComparison := data.MailStats.Prev24hTotal
				if prevTotalForComparison == 0 {
					prevTotalForComparison = 1
				}
				thresholdValue := float64(prevTotalForComparison) * data.MailStats.Threshold24h
				data.MailStats.IsNormal24h = float64(data.MailStats.Last24hTotal) < thresholdValue
			}
		}

		// Get 1-hour mail statistics
		last1hSent, last1hReceived, err := getMailStats(last1hStart, now)
		if err == nil {
			data.MailStats.Last1hSent = last1hSent
			data.MailStats.Last1hReceived = last1hReceived
			data.MailStats.Last1hTotal = last1hSent + last1hReceived

			prev1hSent, prev1hReceived, err := getMailStats(prev1hStart, last1hStart)
			if err == nil {
				data.MailStats.Prev1hSent = prev1hSent
				data.MailStats.Prev1hReceived = prev1hReceived
				data.MailStats.Prev1hTotal = prev1hSent + prev1hReceived

				// Calculate 1h threshold and status
				data.MailStats.Threshold1h = MailHealthConfig.Pmg.Email_monitoring.Threshold_factor.Hourly
				prev1hTotalForComparison := data.MailStats.Prev1hTotal
				if prev1hTotalForComparison == 0 {
					prev1hTotalForComparison = 1
				}
				threshold1hValue := float64(prev1hTotalForComparison) * data.MailStats.Threshold1h
				data.MailStats.IsNormal1h = float64(data.MailStats.Last1hTotal) < threshold1hValue
			}
		}
	}

	// Get version status from Proxmox version check (will need to be modified in the future)
	// For now we'll just set some defaults
	data.VersionStatus.CurrentVersion = "unknown"
	data.VersionStatus.LatestVersion = "unknown"
	data.VersionStatus.IsUpToDate = true

	// Determine overall health
	for _, serviceStatus := range data.Services {
		if !serviceStatus {
			data.IsHealthy = false
			break
		}
	}

	if !data.PostgresRunning {
		data.IsHealthy = false
	}

	if !data.QueueStatus.IsHealthy {
		data.IsHealthy = false
	}

	// Set overall status
	if data.IsHealthy {
		data.Status = "Healthy"
	} else {
		data.Status = "Unhealthy"
	}

	return data
}

func Main(cmd *cobra.Command, args []string) {
	common.ScriptName = "pmgHealth"
	common.TmpDir = common.TmpDir + "pmgHealth"
	common.Init()
	common.ConfInit("mail", &MailHealthConfig)
	if MailHealthConfig.Pmg.Email_monitoring.Threshold_factor.Daily == 0 {
		MailHealthConfig.Pmg.Email_monitoring.Threshold_factor.Daily = 2.0
	}

	if MailHealthConfig.Pmg.Email_monitoring.Threshold_factor.Hourly == 0 {
		MailHealthConfig.Pmg.Email_monitoring.Threshold_factor.Hourly = 3.0
	}
	client.WrapperGetServiceStatus("pmgHealth")

	// Collect all health data with skipOutput=true since we'll use our UI rendering
	healthData := CheckPmgHealth(true)

	// Perform 24-hour mail statistics check
	if err := statsCheck24h(); err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "stats_check_24h").
			Msg("Error during 24-hour statistics check")
	}

	// Perform 1-hour mail statistics check
	if err := statsCheck1h(); err != nil {
		log.Error().
			Err(err).
			Str("component", "pmgHealth").
			Str("action", "stats_check_1h").
			Msg("Error during 1-hour statistics check")
	}

	// Check Proxmox Mail Gateway version directly to avoid console output
	if _, err := exec.LookPath("pmgversion"); err == nil {
		// Get the version of Proxmox Mail Gateway
		out, err := exec.Command("pmgversion").Output()
		if err == nil {
			// Parse the version (e.g., "pmg/6.4-13/1c2b3f0e")
			versionString := strings.TrimSpace(strings.Split(string(out), "/")[1])
			healthData.VersionStatus.CurrentVersion = versionString

			// Since we're not using the version check functions, just mark as updated
			// to simplify this part
			healthData.VersionStatus.LatestVersion = versionString
			healthData.VersionStatus.IsUpToDate = true
		}
	}

	// Create a title for the box
	title := "Proxmox Mail Gateway Health Status"

	// Generate content using our UI renderer
	content := healthData.RenderCompact()

	// Display the rendered box
	renderedBox := common.DisplayBox(title, content)
	fmt.Println(renderedBox)

	// Store the health data for future API access
	// Will be implemented later when API functionality is added
	// api.SetServiceHealthData("pmgHealth", healthData)
}
