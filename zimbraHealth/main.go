//go:build linux && plugin

package zimbraHealth

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	"github.com/monobilisim/monokit/common/healthdb"
	mail "github.com/monobilisim/monokit/common/mail"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	ver "github.com/monobilisim/monokit/common/versionCheck"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/gomail.v2"
)

// DetectZimbra checks for the presence of Zimbra installation directories.
func DetectZimbra() bool {
	// Check for standard Zimbra path
	if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
		log.Debug().Msg("Zimbra detected at /opt/zimbra.")
		return true
	}

	log.Debug().Str("path", "/opt/zimbra").Msg("Zimbra not detected")
	return false
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "zimbraHealth", // Name used in config/daemon loop
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectZimbra, // Use the new DetectZimbra function
	})
}

var MailHealthConfig mail.MailHealth
var zimbraPath string // Determined in collectHealthData
var restartCounter int
var lastRestart time.Time // Track last restart attempt time
var templateFile string   // Determined in collectHealthData
var CacheFilePath = "/tmp/mono/zimbraHealth/cache.json"

// Main entry point for zimbraHealth
func Main(cmd *cobra.Command, args []string) {
	// version := "2.3.0" // Removed unused variable
	common.ScriptName = "zimbraHealth"
	common.TmpDir = common.TmpDir + "zimbraHealth"
	common.Init()
	common.ConfInit("mail", &MailHealthConfig)
	client.WrapperGetServiceStatus("zimbraHealth") // Keep this for service status reporting

	// log.Debug().Msg("Starting Zimbra Health Check - v" + version) // Removed LogInfo

	if common.ProcGrep("install.sh", true) {
		// log.Debug().Msg("Installation is running. Exiting.") // Removed LogInfo
		fmt.Println("Installation is running. Exiting.") // Keep user-facing message
		return
	}

	var healthData *ZimbraHealthData

	// Check if we should run a full check or use cached data
	if shouldRunFullCheck() {
		log.Debug().Msg("Running full health check")
		// Collect fresh health data
		healthData = collectHealthData()

		// Save to cache
		if err := saveCachedData(healthData); err != nil {
			log.Error().Err(err).Msg("Failed to save data to cache")
		}
	} else {
		log.Debug().Msg("Loading data from cache")
		// Load from cache
		var err error
		healthData, err = loadCachedData()
		if err != nil {
			log.Error().Err(err).Msg("Failed to load cached data, running full check")
			// Fallback to full check
			healthData = collectHealthData()
			if saveErr := saveCachedData(healthData); saveErr != nil {
				log.Error().Err(saveErr).Msg("Failed to save fallback data to cache")
			}
		} else {
			// Even when using cached data, always perform a live service check
			// and attempt restarts if needed, then overlay into the cached view.
			if zimbraPath == "" {
				if _, derr := os.Stat("/opt/zimbra"); !os.IsNotExist(derr) {
					zimbraPath = "/opt/zimbra"
				} else {
					log.Error().Str("path", "/opt/zimbra").Msg("Zimbra not detected for live service refresh")
				}
			}

			if zimbraPath != "" {
				currentServices := CheckZimbraServices()
				if len(currentServices) > 0 {
					healthData.Services = currentServices
					// Optionally refresh basic system fields for UI clarity
					if healthData.System.ProductPath == "" {
						healthData.System.ProductPath = zimbraPath
					}
					healthData.System.LastChecked = time.Now().Format("2006-01-02 15:04:05")
				}
			}
		}
	}

	// Display as a nice box UI
	displayBoxUI(healthData)

	// Run background/periodic tasks separately (only on full checks)
	if !healthData.CacheInfo.FromCache {
		runPeriodicTasks(healthData) // Pass healthData if needed for context
	}
}

// collectHealthData gathers all the health information
func collectHealthData() *ZimbraHealthData {
	healthData := NewZimbraHealthData()
	healthData.System.LastChecked = time.Now().Format("2006-01-02 15:04:05")

	// Determine Zimbra path and set basic system info
	if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
		zimbraPath = "/opt/zimbra"
		healthData.System.ProductPath = zimbraPath
	} else {
		log.Error().Str("path", "/opt/zimbra").Msg("Zimbra not detected")
		// Return partially filled data or handle error appropriately
		return healthData // Or perhaps os.Exit(1) if unusable
	}
	templateFile = zimbraPath + "/conf/nginx/templates/nginx.conf.web.https.default.template"
	hostname, _ := os.Hostname()
	healthData.System.Hostname = hostname

	// --- Collect data from checks ---
	healthData.IPAccess = CheckIpAccess()
	healthData.Services = CheckZimbraServices()

	// Version Check
	versionStr, err := ver.ZimbraCheck()
	if err != nil {
		healthData.Version.CheckStatus = false
		healthData.Version.Message = err.Error()
	} else {
		healthData.Version.CheckStatus = true
		healthData.Version.InstalledVersion = versionStr
		// Note: LatestVersion and UpdateAvailable are not populated by ver.ZimbraCheck yet
	}

	// Z-Push Check (only if URL configured)
	if MailHealthConfig.Zimbra.Z_Url != "" {
		healthData.ZPush = CheckZPush()
	}

	// Queued Messages Check
	healthData.QueuedMessages = CheckQueuedMessages()

	// SSL Check (run regardless of time for UI, but actual issue creation might depend on time)
	healthData.SSLCert = CheckSSL()

	// Hosts File Check (always run for UI display, but actual monitoring depends on scheduling)
	healthData.HostsFile = CheckHostsFile()

	// Login Test Check (only if enabled and credentials configured)
	healthData.LoginTest = CheckLoginTest()

	// Email Send Test Check (only if enabled and configured)
	healthData.EmailSendTest = CheckEmailSendTest()

	// CBPolicyd Check (service and database connectivity)
	healthData.CBPolicyd = CheckCBPolicyd()

	// Webhook Tail Info (config only for now)
	healthData.WebhookTail.Logfile = MailHealthConfig.Zimbra.Webhook_tail.Logfile
	healthData.WebhookTail.QuotaLimit = MailHealthConfig.Zimbra.Webhook_tail.Quota_limit

	return healthData
}

// displayBoxUI displays the health data in a nice box UI
func displayBoxUI(healthData *ZimbraHealthData) {
	title := fmt.Sprintf("monokit zimbraHealth @ %s", healthData.System.Hostname)
	content := healthData.RenderAll() // Use the RenderAll method from ui.go

	renderedBox := common.DisplayBox(title, content)
	fmt.Println(renderedBox)
}

// runPeriodicTasks handles tasks not directly part of the immediate health status display
func runPeriodicTasks(healthData *ZimbraHealthData) {
	// Webhook Tailing (runs every time if configured)
	if !common.IsEmptyOrWhitespaceStr(healthData.WebhookTail.Logfile) && healthData.WebhookTail.QuotaLimit != 0 {
		// common.SplitSection("Webhook Tail") // Removed SplitSection, TailWebhook has its own logging if needed
		TailWebhook(healthData.WebhookTail.Logfile, healthData.WebhookTail.QuotaLimit)
	}

	// Tasks to run only at specific times (e.g., 01:00)
	//date := time.Now().Format("15:04") // Use 15:04 for HH:MM format
	// Get env variable ZIMBRA_HEALTH_TEST_ZMFIXPERMS
	//if date == "01:00" || (os.Getenv("ZIMBRA_HEALTH_TEST_ZMFIXPERMS") == "true" || os.Getenv("ZIMBRA_HEALTH_TEST_ZMFIXPERMS") == "1") {
	// log.Debug().Msg("Running scheduled 01:00 tasks...") // Removed LogInfo
	// common.SplitSection("Running zmfixperms") // Removed SplitSection
	//Zmfixperms() // Zmfixperms has its own logging
	// Note: SSL check data is already collected in collectHealthData for UI display.
	// The Redmine issue creation logic within CheckSSL might still be relevant here
	// if it should only happen at 01:00. Consider refactoring CheckSSL further if needed.
	//	}

	// Hosts file monitoring - run every 12 hours (at 12:00 AM and 12:00 PM)
	currentTime := time.Now().Format("15:04") // Use 15:04 for HH:MM format
	if currentTime == "00:00" || currentTime == "12:00" || os.Getenv("ZIMBRA_HEALTH_TEST_HOSTS_CHECK") == "true" {
		log.Debug().Str("time", currentTime).Msg("Running scheduled hosts file check...")
		// The actual check is already performed in collectHealthData for UI display
		// This scheduled run ensures alarms are sent at the right time
		hostsInfo := CheckHostsFile()
		if hostsInfo.HasChanges {
			log.Warn().Str("backup_path", hostsInfo.BackupPath).Msg("Scheduled hosts file check detected changes")
		} else {
			log.Debug().Str("backup_path", hostsInfo.BackupPath).Msg("Scheduled hosts file check completed - no changes")
		}
	}
}

// escapeJSON remains unchanged
func escapeJSON(input string) string {
	output := bytes.Buffer{}
	for _, r := range input {
		switch r {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		default:
			output.WriteRune(r)
		}
	}
	return output.String() // Return the result
} // <-- Correct end of escapeJSON

// TailWebhook remains largely unchanged, but logging might be preferred over fmt.Println
func TailWebhook(filePath string, quotaLimit int) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		log.Error().Err(err).Str("file_path", filePath).Msg("Failed to open webhook log file")
		// Consider returning error or handling differently
		return // Exit if file cannot be opened
	}
	defer file.Close()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Use regex to get the ID from the log line
		// `- (\d+)\]` matches the ID in the log line
		re := regexp.MustCompile(`- (\d+)\]`)
		matches := re.FindStringSubmatch(line)
		if len(matches) < 2 {
			log.Warn().Str("line", line).Msg("Could not match ID in webhook log line") // Changed to Warn
			continue                                                                   // Skip this line if ID not found
		}
		id := matches[1]

		// Add another regex
		// `quota=([\d.]+)\/([\d.]+) \(([\d.]+)%\)` matches the quota in the log line
		re = regexp.MustCompile(`quota=([\d.]+)\/([\d.]+) \(([\d.]+)%\)`)
		matches = re.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue // Skip if quota info not found
		}
		var percentage float64
		percentage, err = strconv.ParseFloat(matches[3], 64) // Use 64-bit float and check error
		if err != nil {
			log.Warn().Str("line", line).Str("percentage", matches[3]).Err(err).Msg("Could not parse quota percentage")
			continue
		}
		quotaLimitFloat := float64(quotaLimit) // Convert limit once

		if percentage < quotaLimitFloat {
			continue
		}

		// Check if the file exists (marker for already alarmed)
		markerFile := common.TmpDir + "/webhook_tail_" + id
		if _, err := os.Stat(markerFile); os.IsNotExist(err) {
			// Create the marker file
			markerHandle, createErr := os.Create(markerFile)
			if createErr != nil {
				log.Error().Str("marker_file", markerFile).Err(createErr).Msg("Failed to create webhook marker file")
				// Continue without marker, might re-alarm
			} else {
				markerHandle.Close() // Close file handle immediately after creation
			}

			// Send the alarm
			log.Warn().Str("line", line).Msg("Webhook quota limit exceeded") // Log the event as Warn
			common.Alarm("[zimbraHealth - "+common.Config.Identifier+"] Quota limit is above "+strconv.Itoa(quotaLimit)+"% "+escapeJSON(line), MailHealthConfig.Zimbra.Webhook_tail.Stream, MailHealthConfig.Zimbra.Webhook_tail.Topic, true)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error().Err(err).Str("file_path", filePath).Msg("Failed to read webhook file content")
	}
} // <-- Correct end of TailWebhook

// Zmfixperms remains unchanged, but remove internal LogInfo
func Zmfixperms() {
	// log.Debug().Msg("Running zmfixperms...") // Removed LogInfo
	out, err := ExecZimbraCommand("libexec/zmfixperms", true, true)

	if err != nil {
		common.Alarm("["+common.Config.Identifier+"] Zmfixperms failed: \n```spoiler Error\n"+err.Error()+"\n```", MailHealthConfig.Zimbra.Zmfixperms.Stream, MailHealthConfig.Zimbra.Zmfixperms.Topic, true)
	} else {
		_, _ = ExecZimbraCommand("zmcontrol restart", false, false) // Restart Zimbra services after zmfixperms
		secondOut, _ := ExecZimbraCommand("zmcontrol status", false, false)
		common.Alarm("["+common.Config.Identifier+"] Zmfixperms completed successfully: \n```spoiler Zmfixperms Output\n"+out+"\n``` ```spoiler zmcontrol status output\n"+secondOut+"\n```", MailHealthConfig.Zimbra.Zmfixperms.Stream, MailHealthConfig.Zimbra.Zmfixperms.Topic, true)
	}
} // <-- Correct end of Zmfixperms

// CheckIpAccess refactored to return IPAccessInfo
func CheckIpAccess() IPAccessInfo {
	info := IPAccessInfo{CheckStatus: false} // Default to check failed
	var productName string
	var certFile string
	var keyFile string
	var message string = "Hello World!" // Keep the check message
	var regexPattern string
	var proxyBlock string
	var output string

	// zimbraPath and templateFile should be available globally or passed
	if zimbraPath == "" {
		info.Message = "Zimbra path not determined."
		log.Error().Str("zimbra_path", zimbraPath).Msg("Zimbra path not determined")
		return info
	}

	productName = "zimbra"

	certFile = zimbraPath + "/ssl/" + productName + "/server/server.crt"
	keyFile = zimbraPath + "/ssl/" + productName + "/server/server.key"

	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		info.Message = "Nginx template file not found: " + templateFile
		log.Error().Str("template_file", templateFile).Msg("Nginx template file not found")
		return info
	}

	// Determine IP Address
	if _, err := os.Stat(zimbraPath + "/conf/nginx/external_ip.txt"); !os.IsNotExist(err) {
		fileContent, err := os.ReadFile(zimbraPath + "/conf/nginx/external_ip.txt")
		if err != nil {
			log.Warn().Str("error", err.Error()).Msg("Error reading external_ip.txt. Falling back to ifconfig.co")
			// Fallback below
		} else {
			info.IPAddress = strings.TrimSpace(string(fileContent))
		}
	}
	// Fallback or primary method: Get IP from ifconfig.co
	if info.IPAddress == "" {
		resp, err := http.Get("https://ifconfig.co")
		if err != nil {
			info.Message = "Error getting external IP from ifconfig.co: " + err.Error()
			log.Error().Str("message", info.Message).Msg("Error getting external IP from ifconfig.co")
			return info
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			info.Message = "Error reading external IP response: " + err.Error()
			log.Error().Str("message", info.Message).Msg("Error reading external IP response")
			return info
		}
		info.IPAddress = strings.TrimSpace(string(respBody))
	}

	// Validate IP format
	ipRegex := `\b[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\b`
	re := regexp.MustCompile(ipRegex)
	matches := re.FindAllString(info.IPAddress, -1)
	if len(matches) == 0 {
		info.Message = "External IP address format is invalid: " + info.IPAddress
		log.Error().Str("ip_address", info.IPAddress).Msg("External IP address format is invalid")
		return info
	}
	info.IPAddress = matches[0] // Use the first valid IP found

	// Check/Add Nginx Block
	regexPattern = fmt.Sprintf(
		`(?m)\n?(server\s+?{\n?\s+listen\s+443\s+ssl\s+http2;\n?\s+server_name\n?\s+%s;\n?\s+ssl_certificate\s+%s;\n?\s+ssl_certificate_key\s+%s;\n?\s+location\s+\/\s+{\n?\s+return\s+200\s+'%s';\n?\s+}\n?})`,
		info.IPAddress, certFile, keyFile, message)

	proxyBlock = fmt.Sprintf(`server {
listen                  443 ssl http2;
server_name             %s;
ssl_certificate         %s;
ssl_certificate_key     %s;
location / {
    return 200 '%s';
}
}`, info.IPAddress, certFile, keyFile, message)

	templateContent, err := os.ReadFile(templateFile)
	if err != nil {
		info.Message = "Error reading template file: " + err.Error()
		log.Error().Str("template_file", templateFile).Err(err).Msg("Error reading template file")
		return info
	}

	re = regexp.MustCompile(regexPattern)
	matches = re.FindAllString(string(templateContent), -1)
	if len(matches) > 0 {
		output = strings.ReplaceAll(matches[0], "", "\n") // Keep this logic? Seems unused.
	}

	if output == "" { // Block not found
		log.Warn().Str("template_file", templateFile).Msg("Adding proxy control block in template file") // Changed to Warn as it's modifying config
		fileHandle, err := os.OpenFile(templateFile, os.O_APPEND|os.O_WRONLY, 0644)                      // Don't create if not exists
		if err != nil {
			info.Message = fmt.Sprintf("Error opening template file for append: %v", err)
			log.Error().Str("template_file", templateFile).Err(err).Msg("Error opening template file for append")
			return info
		}
		defer fileHandle.Close()
		if _, err := fileHandle.WriteString("\n" + proxyBlock + "\n"); err != nil {
			info.Message = fmt.Sprintf("Error writing proxy block to template file: %v", err)
			log.Error().Str("template_file", templateFile).Err(err).Msg("Error writing proxy block to template file")
			return info
		}
		log.Warn().Msg("Proxy control block added. Restarting proxy service") // Changed to Warn
		// Consider if restart should happen here or be handled separately
		_, err = ExecZimbraCommand("zmproxyctl restart", false, false)
		if err != nil {
			log.Warn().Str("error", err.Error()).Msg("Failed to restart proxy after adding IP block")
			// Continue check anyway
		}
	}

	// Perform the HTTP check
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Keep insecure for IP check
		},
	}
	req, err := http.NewRequest("GET", "https://"+info.IPAddress, nil)
	if err != nil {
		info.Message = "Failed to create HTTP request: " + err.Error()
		log.Error().Str("ip_address", info.IPAddress).Err(err).Msg("Failed to create HTTP request")
		common.AlarmCheckDown("accesswithip", "Check failed: "+info.Message, false, "", "")
		return info
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		info.Message = "HTTP request failed: " + err.Error()
		log.Error().Str("ip_address", info.IPAddress).Err(err).Msg("HTTP request failed")
		common.AlarmCheckDown("accesswithip", "Cannot access IP "+info.IPAddress+": "+err.Error(), false, "", "")
		return info
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		info.Message = "Failed to read response body: " + err.Error()
		log.Error().Str("ip_address", info.IPAddress).Err(err).Msg("Failed to read response body")
		common.AlarmCheckDown("accesswithip", "Check failed reading response from "+info.IPAddress+": "+err.Error(), false, "", "")
		return info
	}

	bodyStr := string(body)
	info.CheckStatus = true // Check was performed

	if !strings.Contains(bodyStr, message) {
		// This means the IP access IS possible (didn't get the specific message)
		info.Accessible = false // For data structure: false means accessible
		info.Message = "Direct access via IP is possible."
		log.Warn().Str("ip_address", info.IPAddress).Msg("Direct access via IP is possible")
		common.AlarmCheckDown("accesswithip", "Can access Zimbra through plain IP: "+info.IPAddress, false, "", "")
	} else {
		// This means the IP access IS blocked (got the specific message)
		info.Accessible = true // For data structure: true means blocked/not accessible
		info.Message = "Direct access via IP is blocked."
		// log.Debug().Str("message", info.Message).Msg("Direct access via IP is blocked") // Removed LogInfo
		common.AlarmCheckUp("accesswithip", "Cannot access Zimbra through plain IP: "+info.IPAddress, false)
	}

	return info
}

// RestartZimbraService refactored to avoid recursion and direct call to CheckZimbraServices
func RestartZimbraService(service string) bool {
	// Enforce per-service restart interval and limit
	restartInterval := MailHealthConfig.Zimbra.Restart_Interval
	if restartInterval <= 0 {
		restartInterval = 3 // minutes
	}
	interval := time.Duration(restartInterval) * time.Minute

	// Check last restart time per service
	if last := getServiceLastRestartAt(service); !last.IsZero() {
		if time.Since(last) < interval {
			log.Warn().Str("service", service).Dur("remaining", interval-time.Since(last)).Msg("Skipping restart due to interval guard")
			return false
		}
	}

	// Check per-service restart attempt count
	attempts := getServiceRestartCount(service)
	if attempts >= MailHealthConfig.Zimbra.Restart_Limit {
		log.Warn().Str("service", service).Int("attempts", attempts).Int("limit", MailHealthConfig.Zimbra.Restart_Limit).Msg("Restart limit reached")
		common.AlarmCheckDown("service_restart_limit_"+service, "Restart limit reached for "+service, false, "", "")
		return false
	}

	// Clear restart limit alarm since we're within limits
	common.AlarmCheckUp("service_restart_limit_"+service, "Restart limit not exceeded for "+service+" ("+strconv.Itoa(attempts)+"/"+strconv.Itoa(MailHealthConfig.Zimbra.Restart_Limit)+")", false)

	log.Warn().Str("service", service).Msg("Attempting to restart Zimbra services")

	// If the affected service is 'stats', stop zmstat first
	if strings.EqualFold(strings.TrimSpace(service), "stats") {
		log.Warn().Msg("Detected 'stats' service; running 'zmstatctl stop' before restart")
		if out, err := ExecZimbraCommand("zmstatctl stop", false, false); err != nil {
			log.Warn().Err(err).Str("output", out).Msg("zmstatctl stop failed; continuing with restart")
		}
	}

	output, err := ExecZimbraCommand("zmcontrol start", false, false)
	log.Debug().Str("output", output).Msg("zmcontrol start output")
	if err != nil {
		log.Error().Err(err).Str("service", service).Msg("Failed to start Zimbra services")
		common.AlarmCheckDown("service_restart_failed_"+service, "Failed to execute zmcontrol start for "+service+": "+err.Error(), false, "", "")
		return false
	}

	// Update per-service tracking after executing restart command
	setServiceRestartCount(service, attempts+1)
	setServiceLastRestartAt(service, time.Now())
	common.AlarmCheckUp("service_restart_failed_"+service, "Zimbra restart command executed successfully for "+service, false)
	return true
}

// --- Per-service restart attempt tracking ---
func getServiceRestartCount(service string) int {
	jsonStr, _, _, found, err := healthdb.GetJSON("zimbraHealth", "service_restart_"+service)
	if err != nil || !found || jsonStr == "" {
		return 0
	}
	var v struct {
		Count  int    `json:"count"`
		LastAt string `json:"last_at"`
	}
	if json.Unmarshal([]byte(jsonStr), &v) != nil {
		return 0
	}
	return v.Count
}

func setServiceRestartCount(service string, count int) {
	// read existing
	jsonStr, _, _, _, _ := healthdb.GetJSON("zimbraHealth", "service_restart_"+service)
	var v struct {
		Count  int    `json:"count"`
		LastAt string `json:"last_at"`
	}
	_ = json.Unmarshal([]byte(jsonStr), &v)
	v.Count = count
	b, _ := json.Marshal(v)
	_ = healthdb.PutJSON("zimbraHealth", "service_restart_"+service, string(b), nil, time.Now())
}

func getServiceLastRestartAt(service string) time.Time {
	jsonStr, _, _, found, err := healthdb.GetJSON("zimbraHealth", "service_restart_"+service)
	if err != nil || !found || jsonStr == "" {
		return time.Time{}
	}
	var v struct {
		Count  int    `json:"count"`
		LastAt string `json:"last_at"`
	}
	if json.Unmarshal([]byte(jsonStr), &v) != nil || strings.TrimSpace(v.LastAt) == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(v.LastAt))
	if err != nil {
		return time.Time{}
	}
	return t
}

func setServiceLastRestartAt(service string, ts time.Time) {
	jsonStr, _, _, _, _ := healthdb.GetJSON("zimbraHealth", "service_restart_"+service)
	var v struct {
		Count  int    `json:"count"`
		LastAt string `json:"last_at"`
	}
	_ = json.Unmarshal([]byte(jsonStr), &v)
	v.LastAt = ts.Format(time.RFC3339)
	b, _ := json.Marshal(v)
	_ = healthdb.PutJSON("zimbraHealth", "service_restart_"+service, string(b), nil, time.Now())
}

func resetServiceRestartTracking(service string) {
	v := struct {
		Count  int    `json:"count"`
		LastAt string `json:"last_at"`
	}{Count: 0, LastAt: ""}
	b, _ := json.Marshal(v)
	_ = healthdb.PutJSON("zimbraHealth", "service_restart_"+service, string(b), nil, time.Now())
}

// CheckZimbraServices refactored to return []ServiceInfo with recovery tracking
func CheckZimbraServices() []ServiceInfo {
	// Helper to parse zmcontrol status output
	parseStatus := func(statusOutput string) ([]ServiceInfo, map[string]bool) {
		var services []ServiceInfo
		statusMap := make(map[string]bool)
		lines := strings.Split(statusOutput, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "Host") {
				continue
			}
			svc := strings.Join(strings.Fields(line), " ")
			var serviceName string
			var isRunning bool
			if strings.Contains(svc, "Running") {
				isRunning = true
				serviceName = strings.TrimSpace(strings.Split(svc, "Running")[0])
			} else if strings.Contains(svc, "Stopped") {
				isRunning = false
				serviceName = strings.TrimSpace(strings.Split(svc, "Stopped")[0])
			} else if strings.Contains(svc, "is not running") {
				isRunning = false
				serviceName = strings.TrimSpace(strings.Split(svc, "is not running")[0])
			} else {
				log.Warn().Str("line", line).Msg("Could not parse service status line")
				continue
			}
			serviceName = strings.TrimPrefix(serviceName, "service ")
			serviceName = strings.TrimPrefix(serviceName, "carbonio-")
			services = append(services, ServiceInfo{Name: serviceName, Running: isRunning})
			statusMap[serviceName] = isRunning
		}
		return services, statusMap
	}

	initialOutput, err := ExecZimbraCommand("zmcontrol status", false, false)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get zmcontrol status")
		return nil
	}

	currentServices, currentStatus := parseStatus(initialOutput)
	restartAttempted := false

	// Attempt restarts for services that are down
	for _, svc := range currentServices {
		service := strings.TrimSpace(svc.Name)
		switch service {
		case "zmswatchctl", "zmswatch":
			output, err := ExecZimbraCommand("zmzmswatchctl stop", false, false)
			log.Debug().Str("output", output).Msg("zmzmswatchctl stop output")
			if err == nil {
				output, err = ExecZimbraCommand("zmzmswatchctl start", false, false)
				log.Debug().Str("output", output).Msg("zmzmswatchctl start output")
			}
			continue
		case "zmlogswatchctl", "zmlogswatch":
			output, err := ExecZimbraCommand("zmlogswatchctl stop", false, false)
			log.Debug().Str("output", output).Msg("zmlogswatchctl stop output")
			if err == nil {
				output, err = ExecZimbraCommand("zmlogswatchctl start", false, false)
				log.Debug().Str("output", output).Msg("zmlogswatchctl start output")
			}
			continue
		case "zmstatctl", "zmstat", "stats":
			output, err := ExecZimbraCommand("zmstatctl stop", false, false)
			log.Debug().Str("output", output).Msg("zmstatctl stop output")
			if err == nil {
				output, err = ExecZimbraCommand("zmstatctl start", false, false)
				log.Debug().Str("output", output).Msg("zmstatctl start output")
			}
			continue
		}
		if !svc.Running {
			log.Warn().Str("service", svc.Name).Msg("Service is not running")
			common.WriteToFile(common.TmpDir+"/"+"zmcontrol_status_"+time.Now().Format("2006-01-02_15.04.05")+".log", initialOutput)
			common.AlarmCheckDown("service_"+svc.Name, svc.Name+" is not running ````spoiler zmcontrol status\n"+initialOutput+"\n```", false, "", "")
			if MailHealthConfig.Zimbra.Restart {
				if RestartZimbraService(svc.Name) {
					restartAttempted = true
				}
			}
		}
	}

	// If we attempted a restart, re-check status once to reflect success immediately
	if restartAttempted {
		// Small grace period for services to come up
		time.Sleep(5 * time.Second)
		refreshedOutput, err := ExecZimbraCommand("zmcontrol status", false, false)
		if err == nil {
			currentServices, currentStatus = parseStatus(refreshedOutput)
		} else {
			log.Warn().Err(err).Msg("Failed to refresh zmcontrol status after restart attempt")
		}

		// Reset per-service counters for services that recovered successfully
		for _, svc := range currentServices {
			if svc.Running {
				resetServiceRestartTracking(svc.Name)
			}
		}
	}

	// Track state changes and print summary based on the final observed status
	allServiceStates := processServiceStateChanges(common.TmpDir, currentStatus)
	displayServiceSummary(allServiceStates)
	return currentServices
}

// --- Service State Persistence & Summary ---
// --- Service State Persistence & Summary ---
func loadServiceState(tmpDir, name string) (*ServiceState, error) {
	path := filepath.Join(tmpDir, "service_"+name+".state")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s ServiceState
	if json.Unmarshal(b, &s) != nil {
		return nil, err
	}
	return &s, nil
}

func saveServiceState(tmpDir string, s *ServiceState) error {
	path := filepath.Join(tmpDir, "service_"+s.Name+".state")
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// Collect state for all files
func loadAllServiceStates(tmpDir string) (map[string]*ServiceState, error) {
	states := make(map[string]*ServiceState)
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return states, nil // treat as empty if not exists
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".state") {
			continue
		}
		name := strings.TrimPrefix(strings.TrimSuffix(file.Name(), ".state"), "service_")
		if s, err := loadServiceState(tmpDir, name); err == nil {
			states[name] = s
		}
	}
	return states, nil
}

// processServiceStateChanges tracks service state transitions and emits recovery alarms
func processServiceStateChanges(tmpDir string, currentStatus map[string]bool) []*ServiceState {
	now := time.Now().Format("2006-01-02 15:04:05")

	// Load all existing service states
	existingStates, _ := loadAllServiceStates(tmpDir)

	var allStates []*ServiceState

	// Process current services
	for serviceName, isRunning := range currentStatus {
		var state *ServiceState
		if existing, exists := existingStates[serviceName]; exists {
			state = existing
		} else {
			state = &ServiceState{
				Name:   serviceName,
				Status: "Unknown",
			}
		}

		previousStatus := state.Status

		if isRunning {
			state.Status = "Running"
			// Check for recovery (was not running, now running)
			if previousStatus == "Stopped" || previousStatus == "Unknown" {
				state.RecoveredAt = now
				state.RestartAttempts = 0 // Reset on recovery

				// Emit recovery alarm only if down alarm file exists (avoids duplicates)
				alarmFile := filepath.Join(tmpDir, "service_"+serviceName+".log")
				if _, err := os.Stat(alarmFile); err == nil {
					common.AlarmCheckUp("service_"+serviceName, serviceName+" is now running", false)
				}
			}
		} else {
			state.Status = "Stopped"
			// Check for new failure (was running, now stopped)
			if previousStatus == "Running" || previousStatus == "Unknown" {
				state.LastFailure = now
				state.RestartAttempts = 1
				state.RecoveredAt = "" // Clear recovery time
			} else if previousStatus == "Stopped" {
				// Increment restart attempts for continued failure
				state.RestartAttempts++
			}
		}

		// Save updated state
		saveServiceState(tmpDir, state)
		allStates = append(allStates, state)

		// Remove from existing states map (for tracking disappeared services)
		delete(existingStates, serviceName)
	}

	// Handle disappeared services (exist in state files but not in current status)
	for serviceName, state := range existingStates {
		if state.Status != "Disappeared" {
			state.Status = "Disappeared"
			state.RecoveredAt = now

			// Emit recovery alarm for disappeared services (they're no longer failing)
			alarmFile := filepath.Join(tmpDir, "service_"+serviceName+".log")
			if _, err := os.Stat(alarmFile); err == nil {
				common.AlarmCheckUp("service_"+serviceName, serviceName+" is no longer reported by zmcontrol status", false)
			}

			saveServiceState(tmpDir, state)
		}
		allStates = append(allStates, state)
	}

	return allStates
}

// displayServiceSummary prints a summary table of service states
func displayServiceSummary(states []*ServiceState) {
	if len(states) == 0 {
		return
	}

	// Filter to only show services that have had failures
	var relevantStates []*ServiceState
	for _, state := range states {
		if state.LastFailure != "" || state.RestartAttempts > 0 {
			relevantStates = append(relevantStates, state)
		}
	}

	if len(relevantStates) == 0 {
		log.Debug().Msg("No service failures recorded")
		return
	}

	for _, state := range relevantStates {
		lastFailure := "–"
		if state.LastFailure != "" {
			lastFailure = state.LastFailure
		}

		recoveryTime := "–"
		if state.RecoveredAt != "" {
			recoveryTime = state.RecoveredAt
		}

		restartInfo := fmt.Sprintf("%d", state.RestartAttempts)
		if state.RestartAttempts >= 2 {
			restartInfo += "/2 (limit)"
		}

		statusIcon := ""
		switch state.Status {
		case "Running":
			statusIcon = "Running"
		case "Stopped":
			statusIcon = "Stopped"
		case "Disappeared":
			statusIcon = "Gone"
		default:
			statusIcon = state.Status
		}

		log.Debug().Str("name", state.Name).Str("last_failure", lastFailure).Str("restart_info", restartInfo).Str("recovery_time", recoveryTime).Str("status_icon", statusIcon).Msg("Service state")
	}
}

// changeImmutable remains unchanged
func changeImmutable(filePath string, add bool) {
}

// modifyFile remains unchanged (called by CheckZPush if needed)
func modifyFile(templateFile string) {
	// Read the file content
	content, err := os.ReadFile(templateFile)
	if err != nil {
		log.Error().Err(err).Str("template_file", templateFile).Msg("Failed to read template file")
	}

	text := string(content)

	if strings.Contains(text, "nginx-php-fpm.conf") {
		return
	}

	// Define regex patterns and replacements
	blockRegex := regexp.MustCompile(`(?s)(Microsoft-Server-ActiveSync.*?# For audit)`)
	modifiedBlock := blockRegex.ReplaceAllStringFunc(text, func(match string) string {
		match = regexp.MustCompile(`proxy_pass`).ReplaceAllString(match, "### proxy_pass")
		match = regexp.MustCompile(`proxy_read_timeout`).ReplaceAllString(match, "### proxy_read_timeout")
		match = regexp.MustCompile(`proxy_buffering`).ReplaceAllString(match, "### proxy_buffering")
		return regexp.MustCompile(`# For audit`).ReplaceAllString(match, `# Z-PUSH start
        include /etc/nginx-php-fpm.conf;
        # Z-PUSH end

        # For audit`)
	})

	// Write the modified content back to the file
	if err := os.WriteFile(templateFile, []byte(modifiedBlock), 0644); err != nil {
		log.Error().Err(err).Str("template_file", templateFile).Msg("Failed to write modified content to template file")
	}

	log.Warn().Str("template_file", templateFile).Msg("Added Z-Push block to template file, restarting zimbra proxy service") // Changed to Warn
	_, err = ExecZimbraCommand("zmproxyctl restart", false, false)
	if err != nil {
		log.Error().Err(err).Msg("Failed to restart Zimbra proxy service")
	}
}

// ExecZimbraCommand remains unchanged
func ExecZimbraCommand(command string, fullPath bool, runAsRoot bool) (string, error) {
	zimbraUser := "zimbra"

	// Check if zimbra user exists
	cmd := exec.Command("id", "zimbra")
	_ = cmd.Run()

	if runAsRoot {
		zimbraUser = "root"
	}

	cmd = nil

	// Execute command
	if fullPath {
		log.Debug().Str("command", command).Msg("Executing Zimbra command with full path")
		cmd = exec.Command("/bin/su", zimbraUser, "-c", zimbraPath+"/"+command)
	} else {
		log.Debug().Str("command", command).Msg("Executing Zimbra command without full path")
		cmd = exec.Command("/bin/su", zimbraUser, "-c", zimbraPath+"/bin/"+command)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	cmd.Run()

	log.Debug().Str("command", command).Str("output", out.String()).Msg("Command executed")
	if cmd.ProcessState.ExitCode() != 0 {
		return out.String(), fmt.Errorf("Command failed: " + command + " with stdout: " + out.String())
	}

	return out.String(), nil
}

// CheckZPush refactored to return ZPushInfo
func CheckZPush() ZPushInfo {
	info := ZPushInfo{
		URL:         MailHealthConfig.Zimbra.Z_Url,
		CheckStatus: false, // Default to check failed
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		// Add transport with InsecureSkipVerify if needed, depending on Z-Push setup
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("GET", info.URL, nil)
	if err != nil {
		info.Message = "Error creating Z-Push request: " + err.Error()
		log.Error().Err(err).Msg("Error creating Z-Push request")
		common.AlarmCheckDown("zpush", "Check failed: "+info.Message, false, "", "")
		return info
	}

	resp, err := client.Do(req)
	if err != nil {
		info.Message = "Error performing Z-Push request: " + err.Error()
		log.Error().Err(err).Msg("Error performing Z-Push request")
		common.AlarmCheckDown("zpush", "Z-Push request failed: "+err.Error(), false, "", "")
		return info
	}
	defer resp.Body.Close()

	info.CheckStatus = true // Request was successful

	// Check headers
	for key, values := range resp.Header {
		for _, value := range values {
			if strings.Contains(strings.ToLower(key), "zpush") || strings.Contains(strings.ToLower(value), "zpush") {
				info.HeaderFound = true
				break
			}
		}
		if info.HeaderFound {
			break
		}
	}

	// Check Nginx config file existence
	_, err = os.Stat("/etc/nginx-php-fpm.conf")
	info.NginxConfig = err == nil // True if file exists

	// Handle alarms and potential file modification
	if info.HeaderFound {
		// log.Debug().Msg("Z-Push headers detected.") // Removed LogInfo
		common.AlarmCheckUp("zpush", "Z-Push is responding correctly", false)
	} else {
		info.Message = "Z-Push headers not found in response."
		log.Warn().Msg("Z-Push headers not found in response") // Changed to Warn
		common.AlarmCheckDown("zpush", "Z-Push headers not found", false, "", "")
	}

	if !info.NginxConfig {
		log.Warn().Str("template_file", templateFile).Msg("Z-Push Nginx config file not found")
		// Alarm? Or just log? Depends on requirements.
	} else {
		// log.Debug().Msg("Z-Push Nginx config file found.") // Removed LogInfo
		// Check if modification is needed (this logic might need refinement)
		contentBytes, _ := os.ReadFile(templateFile) // Read template again
		if !strings.Contains(string(contentBytes), "nginx-php-fpm.conf") {
			log.Warn().Str("template_file", templateFile).Msg("Nginx template needs Z-Push include. Modifying") // Changed to Warn
			modifyFile(templateFile)                                                                            // This function handles its own logging/errors
		}
	}

	return info
}

// CheckQueuedMessages refactored to return QueuedMessagesInfo
func CheckQueuedMessages() QueuedMessagesInfo {
	info := QueuedMessagesInfo{
		Limit:       MailHealthConfig.Zimbra.Queue_Limit,
		CheckStatus: false, // Default to check failed
	}

	// zimbraPath should be set globally
	if zimbraPath == "" {
		info.Message = "Zimbra path not determined."
		log.Error().Msg(info.Message)
		return info
	}

	cmd := exec.Command(zimbraPath + "/common/sbin/mailq")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out // Capture stderr as well

	err := cmd.Run()
	if err != nil {
		// Check if the error is just "Mail queue is empty" which is not a real error
		if strings.Contains(out.String(), "Mail queue is empty") {
			info.Count = 0
			info.CheckStatus = true
			// log.Debug().Msg("Mail queue is empty.") // Removed LogInfo
		} else {
			info.Message = "Error running mailq: " + err.Error() + " - Output: " + out.String()
			log.Error().Msg(info.Message)
			common.AlarmCheckDown("mailq_error", "Failed to run mailq: "+err.Error(), false, "", "")
			return info
		}
	} else {
		// Regex to match lines starting with A-F or 0-9 (queue IDs)
		re := regexp.MustCompile(`^[A-F0-9]`)
		scanner := bufio.NewScanner(&out)
		count := 0
		for scanner.Scan() {
			line := scanner.Text()
			if re.MatchString(line) {
				count++
			}
		}
		if err := scanner.Err(); err != nil {
			info.Message = "Error reading mailq output: " + err.Error()
			log.Error().Msg(info.Message)
			common.AlarmCheckDown("mailq_error", "Failed to read mailq output: "+err.Error(), false, "", "")
			return info
		}
		info.Count = count
		info.CheckStatus = true
	}

	// Process results if check was successful
	if info.CheckStatus {
		info.Exceeded = info.Count > info.Limit
		// log.Debug().Msg(fmt.Sprintf("Queued messages: %d (Limit: %d)", info.Count, info.Limit)) // Removed LogInfo

		if info.Exceeded {
			log.Warn().Msg("Mail queue limit exceeded.")
			common.AlarmCheckDown("mailq", fmt.Sprintf("Mail queue is over the limit (%d/%d)", info.Count, info.Limit), false, "", "")
		} else {
			common.AlarmCheckUp("mailq", fmt.Sprintf("Mail queue is under the limit (%d/%d)", info.Count, info.Limit), false)
		}
	}

	return info
}

// CheckSSL refactored to return SSLCertInfo
func CheckSSL() SSLCertInfo {
	info := SSLCertInfo{CheckStatus: false} // Default to check failed
	const expiryThresholdDays = 10          // Define threshold

	// Get Zimbra hostname
	zmHostname, err := ExecZimbraCommand("zmhostname", false, false)
	if err != nil {
		info.Message = "Error getting zimbra hostname: " + err.Error()
		log.Error().Msg(info.Message)
		return info
	}
	zmHostname = strings.TrimSpace(zmHostname)

	// Get service hostname for the mail host
	provOutput, err := ExecZimbraCommand("zmprov gs "+zmHostname+" zimbraServiceHostname", false, false)
	if err != nil {
		info.Message = "Error getting zimbraServiceHostname: " + err.Error()
		log.Error().Msg(info.Message)
		return info
	}

	// Parse zimbraServiceHostname
	re := regexp.MustCompile(`zimbraServiceHostname:\s*(.*)`)
	matches := re.FindStringSubmatch(provOutput)
	if len(matches) < 2 {
		info.Message = "Could not parse zimbraServiceHostname from zmprov output: " + provOutput
		log.Error().Msg(info.Message)
		// Fallback to zmHostname itself?
		info.MailHost = zmHostname
		log.Warn().Msg("Falling back to using zmhostname for SSL check.")
		// return info // Decide if fallback is acceptable or should fail
	} else {
		info.MailHost = strings.TrimSpace(matches[1])
	}

	if info.MailHost == "" {
		info.Message = "Mail host for SSL check could not be determined."
		log.Error().Msg(info.Message)
		return info
	}

	log.Debug().Msg("Checking SSL certificate for host: " + info.MailHost)

	// Dial to get certificate
	conn, err := tls.Dial("tcp", info.MailHost+":443", &tls.Config{
		InsecureSkipVerify: true,          // Keep insecure for self-signed/internal CAs often used
		ServerName:         info.MailHost, // Important for SNI
	})
	if err != nil {
		info.Message = "Error connecting to " + info.MailHost + ":443 for SSL check: " + err.Error()
		log.Error().Msg(info.Message)
		common.AlarmCheckDown("sslcert_conn", "Failed to connect for SSL check: "+err.Error(), false, "", "")
		return info
	}
	defer conn.Close()

	// Check peer certificates
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		info.Message = "No SSL certificates found for " + info.MailHost
		log.Error().Msg(info.Message)
		common.AlarmCheckDown("sslcert_nocert", info.Message, false, "", "")
		return info
	}

	cert := certs[0]
	info.CheckStatus = true // Check performed successfully

	// Calculate days until expiry
	info.DaysUntilExpiry = int(time.Until(cert.NotAfter).Hours() / 24)
	info.ExpiringSoon = info.DaysUntilExpiry < expiryThresholdDays

	// log.Debug().Msg(fmt.Sprintf("SSL Certificate for %s expires in %d days.", info.MailHost, info.DaysUntilExpiry)) // Removed LogInfo

	// Handle alarms and Redmine issues
	alarmMsg := fmt.Sprintf("SSL Certificate for %s expires in %d days", info.MailHost, info.DaysUntilExpiry)
	if info.ExpiringSoon {
		log.Warn().Msg("SSL Certificate is expiring soon.")
		common.AlarmCheckDown("sslcert", alarmMsg, false, "", "")
		// Only create/update Redmine issue if expiring soon
		viewDeployedCert, err := ExecZimbraCommand("zmcertmgr viewdeployedcrt", false, false)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get deployed cert details for Redmine issue")
			viewDeployedCert = "Could not retrieve certificate details."
		}
		issueBody := fmt.Sprintf("Certificate for %s expires on %s.\n\n```\n%s\n```", info.MailHost, cert.NotAfter.Format("2006-01-02"), viewDeployedCert)
		issues.CheckDown("sslcert", common.Config.Identifier+" sunucusunun SSL sertifikası bitimine "+fmt.Sprintf("%d gün kaldı", info.DaysUntilExpiry), issueBody, false, 0)
	} else {
		common.AlarmCheckUp("sslcert", alarmMsg, false)
		// Close existing Redmine issue if it's no longer expiring soon
		issues.CheckUp("sslcert", "SSL sertifikası artık "+fmt.Sprintf("%d gün sonra sona erecek şekilde güncellendi", info.DaysUntilExpiry))
	}

	return info
}

// CheckHostsFile monitors /etc/hosts file for changes
func CheckHostsFile() HostsFileInfo {
	info := HostsFileInfo{CheckStatus: false} // Default to check failed
	info.LastChecked = time.Now().Format("2006-01-02 15:04:05")

	hostsFilePath := "/etc/hosts"
	backupFileName := "hosts_backup"
	info.BackupPath = filepath.Join(common.TmpDir, backupFileName)

	// Check if /etc/hosts exists
	if _, err := os.Stat(hostsFilePath); os.IsNotExist(err) {
		info.Message = "/etc/hosts file does not exist"
		log.Error().Str("hosts_file", hostsFilePath).Msg("/etc/hosts file does not exist")
		return info
	}

	// Check if backup exists, create if it doesn't
	if _, err := os.Stat(info.BackupPath); os.IsNotExist(err) {
		log.Debug().Str("backup_path", info.BackupPath).Msg("Creating initial backup of /etc/hosts")

		// Read current hosts file
		hostsContent, err := os.ReadFile(hostsFilePath)
		if err != nil {
			info.Message = "Failed to read /etc/hosts: " + err.Error()
			log.Error().Err(err).Str("hosts_file", hostsFilePath).Msg("Failed to read /etc/hosts")
			return info
		}

		// Create backup
		err = os.WriteFile(info.BackupPath, hostsContent, 0644)
		if err != nil {
			info.Message = "Failed to create backup: " + err.Error()
			log.Error().Err(err).Str("backup_path", info.BackupPath).Msg("Failed to create hosts backup")
			return info
		}

		info.BackupExists = true
		info.HasChanges = false
		info.CheckStatus = true
		info.Message = "Initial backup created successfully"
		log.Debug().Str("backup_path", info.BackupPath).Msg("Initial hosts backup created")
		return info
	}

	info.BackupExists = true

	// Read current hosts file
	currentContent, err := os.ReadFile(hostsFilePath)
	if err != nil {
		info.Message = "Failed to read current /etc/hosts: " + err.Error()
		log.Error().Err(err).Str("hosts_file", hostsFilePath).Msg("Failed to read current /etc/hosts")
		return info
	}

	// Read backup file
	backupContent, err := os.ReadFile(info.BackupPath)
	if err != nil {
		info.Message = "Failed to read backup file: " + err.Error()
		log.Error().Err(err).Str("backup_path", info.BackupPath).Msg("Failed to read hosts backup")
		return info
	}

	// Compare files
	info.CheckStatus = true
	if !bytes.Equal(currentContent, backupContent) {
		info.HasChanges = true
		info.Message = "Changes detected in /etc/hosts file"
		log.Warn().Str("hosts_file", hostsFilePath).Msg("Changes detected in /etc/hosts file")

		// Send alarm for changes detected
		common.AlarmCheckDown("hosts_file_changed", "/etc/hosts file has been modified since last backup", true, "", "")

		// Update backup with current content for next comparison
		err = os.WriteFile(info.BackupPath, currentContent, 0644)
		if err != nil {
			log.Error().Err(err).Str("backup_path", info.BackupPath).Msg("Failed to update hosts backup after change detection")
		} else {
			log.Debug().Str("backup_path", info.BackupPath).Msg("Updated hosts backup with current content")
		}
	} else {
		info.HasChanges = false
		info.Message = "No changes detected"
		// Send success alarm for no changes
		common.AlarmCheckUp("hosts_file_changed", "/etc/hosts file is unchanged since last backup", false)
	}

	return info
}

// CheckLoginTest performs a login test using Zimbra's zmmailbox command
func CheckLoginTest() LoginTestInfo {
	info := LoginTestInfo{
		Enabled:     MailHealthConfig.Zimbra.Login_test.Enabled,
		Username:    MailHealthConfig.Zimbra.Login_test.Username,
		CheckStatus: false, // Default to check failed
	}

	// Skip entirely if not enabled
	if !info.Enabled {
		info.Message = "Login test is disabled in configuration"
		log.Debug().Str("message", info.Message).Msg("Skipping login test")
		return info
	}

	// Check if zimbraPath is available (needed for provisioning/existence checks)
	if zimbraPath == "" {
		info.Message = "Zimbra path not determined"
		log.Error().Msg(info.Message)
		return info
	}

	// Determine if credentials are manually provided via config (both username and password present)
	manualFromConfig := MailHealthConfig.Zimbra.Login_test.Username != "" && MailHealthConfig.Zimbra.Login_test.Password != ""

	// Resolve credentials: prefer manual config; otherwise use DB or auto-provision
	if !manualFromConfig {
		// If credentials are missing in config, try to load/create them automatically
		if uname, pwd, ok := loadLoginTestCredsFromDB(); ok {
			info.Username = uname
			MailHealthConfig.Zimbra.Login_test.Username = uname
			MailHealthConfig.Zimbra.Login_test.Password = pwd
		} else {
			// Attempt to ensure a test account exists and store creds
			uname, pwd, err := ensureLoginTestAccount()
			if err != nil {
				info.Message = "Login test credentials not configured and could not auto-provision: " + err.Error()
				log.Debug().Str("message", info.Message).Msg("Skipping login test")
				return info
			}
			info.Username = uname
			MailHealthConfig.Zimbra.Login_test.Username = uname
			MailHealthConfig.Zimbra.Login_test.Password = pwd
		}
	}

	// Before attempting login, verify that the account exists
	if !accountExists(info.Username) {
		if manualFromConfig {
			// Respect manual configuration: do not auto-create
			info.Message = "Configured login test account does not exist: " + info.Username
			log.Error().Str("username", info.Username).Msg("Login test account missing and is manually configured")
			common.AlarmCheckDown("zimbra_login_test", "Configured login test account does not exist: "+info.Username, false, "", "")
			return info
		}

		// Auto-managed flow: (re)provision account and overwrite DB
		uname, pwd, err := ensureLoginTestAccountWithPreferredAddress(info.Username)
		if err != nil {
			info.Message = "Failed to provision login test account: " + err.Error()
			log.Error().Err(err).Str("username", info.Username).Msg("Failed to auto-provision login test account")
			common.AlarmCheckDown("zimbra_login_test", "Failed to auto-provision login test account: "+err.Error(), false, "", "")
			return info
		}
		info.Username = uname
		MailHealthConfig.Zimbra.Login_test.Username = uname
		MailHealthConfig.Zimbra.Login_test.Password = pwd
		log.Debug().Str("username", info.Username).Msg("Auto-provisioned login test account")
	}

	log.Debug().Str("username", info.Username).Msg("Starting Zimbra login test")

	// For regular user accounts, use -m (mailbox) flag with -p (password)
	// We'll use 'gms' (get mailbox size) which is a basic read operation
	// that any user can perform on their own mailbox and also tests authentication
	authTestCmd := fmt.Sprintf("zmmailbox -m %s -p %s gms",
		info.Username, MailHealthConfig.Zimbra.Login_test.Password)

	authOutput, err := ExecZimbraCommand(authTestCmd, false, false)
	if err != nil {
		info.Message = "Login authentication failed: " + err.Error()
		log.Error().Str("username", info.Username).Err(err).Msg("Zimbra login test failed")
		common.AlarmCheckDown("zimbra_login_test", "Zimbra login test failed for "+info.Username+": "+err.Error(), false, "", "")
		return info
	}

	// If we get here, authentication was successful and we got mailbox size
	info.LoginSuccessful = true
	info.CheckStatus = true

	// Parse mailbox size output and check for errors
	// Check if there are any ERROR messages in the output
	if strings.Contains(authOutput, "ERROR:") {
		// Extract the error message
		lines := strings.Split(authOutput, "\n")
		var errorMsg string
		for _, line := range lines {
			if strings.Contains(line, "ERROR:") {
				errorMsg = strings.TrimSpace(line)
				break
			}
		}
		info.Message = "Login successful, but mailbox access issue: " + errorMsg
		log.Warn().Str("username", info.Username).Str("error", errorMsg).Msg("Zimbra login successful but with mailbox access warning")
	} else if strings.Contains(authOutput, "size") || strings.Contains(authOutput, "bytes") || authOutput != "" {
		// Clean up the output by removing timestamp and keeping only the size info
		cleanOutput := strings.TrimSpace(authOutput)
		lines := strings.Split(cleanOutput, "\n")
		var sizeInfo string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "KB") || strings.Contains(line, "MB") || strings.Contains(line, "GB") || strings.Contains(line, "bytes") {
				sizeInfo = line
				break
			}
		}
		if sizeInfo != "" {
			info.Message = "Login successful, mailbox accessible"
			info.LastMailSubject = "Mailbox size retrieved"
			info.LastMailDate = sizeInfo
		} else {
			info.Message = "Login successful, mailbox accessible"
			info.LastMailSubject = "Mailbox size retrieved"
			info.LastMailDate = "Size information available"
		}
		log.Debug().Str("username", info.Username).Str("mailbox_size", sizeInfo).Msg("Zimbra login test successful - retrieved mailbox size")
	} else {
		info.Message = "Login successful, mailbox accessible"
		log.Debug().Str("username", info.Username).Msg("Zimbra login test successful")
	}

	// Send success alarm
	common.AlarmCheckUp("zimbra_login_test", "Zimbra login test successful for "+info.Username, false)

	return info
}

// CheckEmailSendTest performs an email send test using the configured SMTP settings
func CheckEmailSendTest() EmailSendTestInfo {
	info := EmailSendTestInfo{
		Enabled:            MailHealthConfig.Zimbra.Email_send_test.Enabled,
		FromEmail:          MailHealthConfig.Zimbra.Email_send_test.From_email,
		ToEmail:            MailHealthConfig.Zimbra.Email_send_test.To_email,
		SMTPServer:         MailHealthConfig.Zimbra.Email_send_test.Smtp_server,
		SMTPPort:           MailHealthConfig.Zimbra.Email_send_test.Smtp_port,
		UseTLS:             MailHealthConfig.Zimbra.Email_send_test.Use_tls,
		Subject:            MailHealthConfig.Zimbra.Email_send_test.Subject,
		CheckStatus:        false, // Default to check failed
		CheckReceived:      MailHealthConfig.Zimbra.Email_send_test.Check_received,
		IMAPServer:         MailHealthConfig.Zimbra.Email_send_test.Imap_server,
		IMAPPort:           MailHealthConfig.Zimbra.Email_send_test.Imap_port,
		IMAPUseTLS:         MailHealthConfig.Zimbra.Email_send_test.Imap_use_tls,
		ToEmailUsername:    MailHealthConfig.Zimbra.Email_send_test.To_email_username,
		ToEmailPassword:    MailHealthConfig.Zimbra.Email_send_test.To_email_password,
		CheckRetries:       MailHealthConfig.Zimbra.Email_send_test.Check_retries,
		CheckRetryInterval: MailHealthConfig.Zimbra.Email_send_test.Check_retry_interval,
	}

	// Check for environment variable override
	forceEmailTest := os.Getenv("MONOKIT_ZIMBRA_HEALTH_MAIL_SEND_TEST") == "1"

	// Skip if not enabled or not properly configured (unless forced by env var)
	if !info.Enabled && !forceEmailTest {
		info.Message = "Email send test is disabled in configuration"
		log.Debug().Str("message", info.Message).Msg("Skipping email send test")
		return info
	}

	// If forced by environment variable, log it
	if forceEmailTest && !info.Enabled {
		log.Debug().Msg("Email send test forced by MONOKIT_ZIMBRA_HEALTH_MAIL_SEND_TEST environment variable")
		info.Enabled = true     // Set enabled to true for the test
		info.ForcedByEnv = true // Mark as forced by environment variable
	}

	if (info.ToEmail == "" || info.SMTPServer == "") && !forceEmailTest {
		info.Message = "Email send test not properly configured (missing to_email or smtp_server)"
		log.Debug().Str("message", info.Message).Msg("Skipping email send test")
		return info
	}

	// If forced by environment variable but missing basic config, provide a warning
	if forceEmailTest && (info.ToEmail == "" || info.SMTPServer == "") {
		log.Warn().Msg("Email send test forced by environment variable but basic configuration (to_email/smtp_server) missing - test will likely fail")
	}

	// Use login test credentials for SMTP authentication (unless forced by env var)
	if (!MailHealthConfig.Zimbra.Login_test.Enabled || MailHealthConfig.Zimbra.Login_test.Username == "" || MailHealthConfig.Zimbra.Login_test.Password == "") && !forceEmailTest {
		info.Message = "Email send test requires login test credentials to be configured for SMTP authentication"
		log.Debug().Str("message", info.Message).Msg("Skipping email send test")
		return info
	}

	// If forced by environment variable but no login credentials, provide a warning
	if forceEmailTest && (MailHealthConfig.Zimbra.Login_test.Username == "" || MailHealthConfig.Zimbra.Login_test.Password == "") {
		log.Warn().Msg("Email send test forced by environment variable but login credentials not configured - test may fail")
	}

	// Use login test username as from_email if not explicitly configured
	if info.FromEmail == "" {
		info.FromEmail = MailHealthConfig.Zimbra.Login_test.Username
		log.Debug().Str("from_email", info.FromEmail).Msg("Using login test username as from_email")
	}

	// Use to_email as to_email_username if not explicitly configured
	if info.ToEmailUsername == "" {
		info.ToEmailUsername = info.ToEmail
		log.Debug().Str("to_email_username", info.ToEmailUsername).Msg("Using to_email as to_email_username")
	}

	// Set default retry settings if not specified
	if info.CheckRetries == 0 {
		info.CheckRetries = 3 // Default to 3 retries
		log.Debug().Int("check_retries", info.CheckRetries).Msg("Using default check retries")
	}
	if info.CheckRetryInterval == 0 {
		info.CheckRetryInterval = 30 // Default to 30 seconds
		log.Debug().Int("check_retry_interval", info.CheckRetryInterval).Msg("Using default check retry interval")
	}

	// Set default port if not specified
	if info.SMTPPort == 0 {
		if info.UseTLS {
			info.SMTPPort = 587 // Default TLS port
		} else {
			info.SMTPPort = 25 // Default non-TLS port
		}
	}

	// Set default subject if not specified
	if info.Subject == "" {
		info.Subject = "Zimbra Health Check Test Email"
	}

	// Generate a unique test ID and add it to the subject for reliable matching
	testID := fmt.Sprintf("ZIMBRA-HEALTH-TEST-%d", time.Now().Unix())
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	uniqueSubject := fmt.Sprintf("%s - %s - %s", info.Subject, timestamp, testID)
	info.Subject = uniqueSubject
	info.TestID = testID

	log.Debug().
		Str("from_email", info.FromEmail).
		Str("to_email", info.ToEmail).
		Str("smtp_server", info.SMTPServer).
		Int("smtp_port", info.SMTPPort).
		Bool("use_tls", info.UseTLS).
		Msg("Starting Zimbra email send test")

	// Create the email message
	m := gomail.NewMessage()
	m.SetHeader("From", info.FromEmail)
	m.SetHeader("To", info.ToEmail)
	m.SetHeader("Subject", info.Subject)

	// Create email body with timestamp and test information
	hostname, _ := os.Hostname()
	body := fmt.Sprintf(`This is a test email sent by Zimbra Health Check.

Test Details:
- Test ID: %s
- Sent at: %s
- From server: %s
- SMTP Server: %s:%d
- TLS Enabled: %t

If you receive this email, the Zimbra email sending functionality is working correctly.

This is an automated test message - no action is required.`,
		info.TestID, timestamp, hostname, info.SMTPServer, info.SMTPPort, info.UseTLS)

	log.Debug().
		Str("test_id", info.TestID).
		Str("subject", info.Subject).
		Int("body_length", len(body)).
		Msg("Generated test email with unique ID")

	m.SetBody("text/plain", body)

	// Create SMTP dialer with authentication using login test credentials
	d := gomail.NewDialer(info.SMTPServer, info.SMTPPort, MailHealthConfig.Zimbra.Login_test.Username, MailHealthConfig.Zimbra.Login_test.Password)

	// Configure TLS settings
	if info.UseTLS {
		d.TLSConfig = &tls.Config{
			ServerName:         info.SMTPServer,
			InsecureSkipVerify: false, // Use proper TLS verification for email
		}
	} else {
		// For non-TLS connections, we might still want to use STARTTLS if available
		d.TLSConfig = &tls.Config{
			ServerName:         info.SMTPServer,
			InsecureSkipVerify: true, // More lenient for internal servers
		}
	}

	// Note: gomail.Dialer doesn't have a Timeout field, but it uses reasonable defaults

	// Attempt to send the email
	if err := d.DialAndSend(m); err != nil {
		info.Message = "Failed to send test email: " + err.Error()
		log.Error().
			Str("from_email", info.FromEmail).
			Str("to_email", info.ToEmail).
			Str("smtp_server", info.SMTPServer).
			Int("smtp_port", info.SMTPPort).
			Err(err).
			Msg("Zimbra email send test failed")
		common.AlarmCheckDown("zimbra_email_send_test", "Zimbra email send test failed: "+err.Error(), true, "", "")
		return info
	}

	// Email sent successfully
	info.SendSuccess = true
	info.CheckStatus = true
	info.SentAt = timestamp
	info.Message = "Test email sent successfully"

	log.Debug().
		Str("from_email", info.FromEmail).
		Str("to_email", info.ToEmail).
		Str("sent_at", info.SentAt).
		Msg("Zimbra email send test successful")

	// Send success alarm
	common.AlarmCheckUp("zimbra_email_send_test", "Zimbra email send test successful - email sent from "+info.FromEmail+" to "+info.ToEmail, false)

	// Check if email was received (if enabled)
	if info.CheckReceived {
		checkEmailReceived(&info)
	}

	return info
}

// ensureLoginTestAccount creates or updates a monokit test account when needed.
// Returns username and password ready for use.
func ensureLoginTestAccount() (string, string, error) {
	if zimbraPath == "" {
		return "", "", fmt.Errorf("Zimbra path not determined")
	}

	// Determine domain to use
	domain, err := choosePreferredDomain()
	if err != nil {
		return "", "", err
	}
	if domain == "" {
		return "", "", fmt.Errorf("no suitable domain found from zmprov gad")
	}

	address := fmt.Sprintf("monokit@%s", domain)
	password := generateRandomPassword(20)

	// Check if account exists
	_, err = ExecZimbraCommand("zmprov -l ga "+address, false, false)
	if err != nil {
		// Create account
		if _, err := ExecZimbraCommand("zmprov ca "+address+" "+password, false, false); err != nil {
			return "", "", fmt.Errorf("failed to create test account %s: %w", address, err)
		}
	} else {
		// Account exists -> set new password (only when provisioning creds)
		if _, err := ExecZimbraCommand("zmprov sp "+address+" "+password, false, false); err != nil {
			return "", "", fmt.Errorf("failed to set password for existing test account %s: %w", address, err)
		}
	}

	// Persist credentials
	saveLoginTestCredsToDB(address, password)
	return address, password, nil
}

// accountExists checks if a Zimbra account exists using zmprov
func accountExists(address string) bool {
	if strings.TrimSpace(address) == "" {
		return false
	}
	_, err := ExecZimbraCommand("zmprov -l ga "+address, false, false)
	return err == nil
}

// ensureLoginTestAccountWithPreferredAddress creates or updates the given address if possible.
// If the domain of preferredAddress does not exist, it falls back to ensureLoginTestAccount().
func ensureLoginTestAccountWithPreferredAddress(preferredAddress string) (string, string, error) {
	if zimbraPath == "" {
		return "", "", fmt.Errorf("Zimbra path not determined")
	}

	preferredAddress = strings.TrimSpace(preferredAddress)
	if preferredAddress == "" || !strings.Contains(preferredAddress, "@") {
		// No usable preferred address; choose automatically
		return ensureLoginTestAccount()
	}

	// Verify domain exists; if not, fallback to automatic selection
	parts := strings.Split(preferredAddress, "@")
	domain := strings.TrimSpace(parts[len(parts)-1])
	gadOut, err := ExecZimbraCommand("zmprov gad", false, false)
	if err != nil {
		return "", "", fmt.Errorf("failed to list domains (gad): %w", err)
	}
	domainExists := false
	for _, line := range strings.Split(gadOut, "\n") {
		if strings.TrimSpace(line) == domain {
			domainExists = true
			break
		}
	}
	var address string
	if domainExists {
		address = preferredAddress
	} else {
		// Fall back to automatic domain choice
		d, derr := choosePreferredDomain()
		if derr != nil {
			return "", "", derr
		}
		if d == "" {
			return "", "", fmt.Errorf("no suitable domain found from zmprov gad")
		}
		address = fmt.Sprintf("monokit@%s", d)
	}

	password := generateRandomPassword(20)

	// Create or set password
	if !accountExists(address) {
		if _, err := ExecZimbraCommand("zmprov ca "+address+" "+password, false, false); err != nil {
			return "", "", fmt.Errorf("failed to create test account %s: %w", address, err)
		}
	} else {
		if _, err := ExecZimbraCommand("zmprov sp "+address+" "+password, false, false); err != nil {
			return "", "", fmt.Errorf("failed to set password for existing test account %s: %w", address, err)
		}
	}

	saveLoginTestCredsToDB(address, password)
	return address, password, nil
}

// choosePreferredDomain selects a domain from `zmprov gad` that:
// - is not equal to the hostname
// - avoids common host-style subdomains (mail., mx., smtp., etc.) when possible
// - if possible, matches the most common domain used by existing accounts
func choosePreferredDomain() (string, error) {
	gadOut, err := ExecZimbraCommand("zmprov gad", false, false)
	if err != nil {
		return "", fmt.Errorf("failed to list domains (gad): %w", err)
	}

	var domains []string
	for _, line := range strings.Split(gadOut, "\n") {
		d := strings.TrimSpace(line)
		if d != "" {
			domains = append(domains, d)
		}
	}
	if len(domains) == 0 {
		return "", nil
	}

	// Exclude exact hostname and common host-style subdomains
	host, _ := os.Hostname()
	host = strings.TrimSpace(host)

	domainSet := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		domainSet[d] = struct{}{}
	}

	filtered := make([]string, 0, len(domains))
	commonHostPrefixes := map[string]struct{}{"mail": {}, "mx": {}, "smtp": {}, "imap": {}, "pop": {}, "pop3": {}, "imap4": {}, "webmail": {}, "server": {}, "srv": {}, "mta": {}, "carbonio": {}, "zimbra": {}}
	for _, d := range domains {
		if d == host {
			continue
		}
		labels := strings.Split(d, ".")
		if len(labels) >= 3 {
			if _, bad := commonHostPrefixes[strings.ToLower(labels[0])]; bad {
				// Skip typical host-based subdomains
				continue
			}
			// If parent domain exists in list, prefer parent; skip this subdomain
			parent := strings.Join(labels[1:], ".")
			if _, ok := domainSet[parent]; ok {
				continue
			}
		}
		filtered = append(filtered, d)
	}
	if len(filtered) == 0 {
		filtered = domains
	}

	// Prefer the domain most used by existing accounts
	var mostCommon string
	counts := map[string]int{}
	if gaaOut, err := ExecZimbraCommand("zmprov -l gaa", false, false); err == nil {
		lines := strings.Split(gaaOut, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || !strings.Contains(line, "@") {
				continue
			}
			parts := strings.Split(line, "@")
			dom := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
			counts[dom]++
		}
		maxCount := -1
		for _, d := range filtered {
			if c := counts[strings.ToLower(d)]; c > maxCount {
				maxCount = c
				mostCommon = d
			}
		}
	}
	if mostCommon != "" {
		return mostCommon, nil
	}
	return filtered[0], nil
}

// generateRandomPassword creates a random password with given length.
// To avoid shell quoting issues in zmprov, restrict to alphanumeric characters.
func generateRandomPassword(length int) string {
	if length <= 0 {
		length = 16
	}
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var b strings.Builder
	b.Grow(length)
	for i := 0; i < length; i++ {
		nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b.WriteByte(charset[nBig.Int64()])
	}
	return b.String()
}

// loadLoginTestCredsFromDB returns stored creds if present
func loadLoginTestCredsFromDB() (string, string, bool) {
	jsonStr, _, _, found, err := healthdb.GetJSON("zimbraHealth", "login_test_account")
	if err != nil || !found || jsonStr == "" {
		return "", "", false
	}
	var v struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if json.Unmarshal([]byte(jsonStr), &v) != nil {
		return "", "", false
	}
	if v.Username == "" || v.Password == "" {
		return "", "", false
	}
	return v.Username, v.Password, true
}

// saveLoginTestCredsToDB persists creds
func saveLoginTestCredsToDB(username, password string) {
	payload := struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		UpdatedAt string `json:"updated_at"`
	}{Username: username, Password: password, UpdatedAt: time.Now().Format("2006-01-02 15:04:05")}
	b, _ := json.Marshal(payload)
	_ = healthdb.PutJSON("zimbraHealth", "login_test_account", string(b), nil, time.Now())
}

// checkEmailReceived checks if the sent email was received in the recipient's mailbox
func checkEmailReceived(info *EmailSendTestInfo) {
	// Validate IMAP configuration
	if info.IMAPServer == "" || info.ToEmailUsername == "" || info.ToEmailPassword == "" {
		info.CheckMessage = "IMAP configuration incomplete (missing server, username, or password)"
		log.Debug().Str("message", info.CheckMessage).Msg("Skipping email receive check")
		return
	}

	// Check for email with retry logic instead of fixed wait
	log.Debug().
		Int("check_retries", info.CheckRetries).
		Int("check_retry_interval", info.CheckRetryInterval).
		Msg("Starting email check with retry logic")

	// Set default IMAP port if not specified
	if info.IMAPPort == 0 {
		if info.IMAPUseTLS {
			info.IMAPPort = 993 // Default IMAPS port
		} else {
			info.IMAPPort = 143 // Default IMAP port
		}
	}

	log.Debug().
		Str("imap_server", info.IMAPServer).
		Int("imap_port", info.IMAPPort).
		Bool("imap_use_tls", info.IMAPUseTLS).
		Str("username", info.ToEmailUsername).
		Msg("Starting IMAP email receive check")

	// Try to find the email with retry logic
	for attempt := 1; attempt <= info.CheckRetries; attempt++ {
		log.Debug().
			Int("attempt", attempt).
			Int("max_attempts", info.CheckRetries).
			Msg("Attempting to check for email")

		if checkEmailExists(info, attempt) {
			// Email found!
			info.ReceiveSuccess = true
			info.ReceivedAt = time.Now().Format("2006-01-02 15:04:05")
			info.CheckMessage = fmt.Sprintf("Email successfully received on attempt %d - found email with test ID '%s'", attempt, info.TestID)

			log.Debug().
				Int("attempt", attempt).
				Str("test_id", info.TestID).
				Msg("Email found successfully")
			break
		}

		// If this wasn't the last attempt, wait before retrying
		if attempt < info.CheckRetries {
			waitDuration := time.Duration(info.CheckRetryInterval) * time.Second
			log.Debug().
				Int("attempt", attempt).
				Int("next_attempt_in_seconds", info.CheckRetryInterval).
				Msg("Email not found, waiting before next attempt")
			time.Sleep(waitDuration)
		} else {
			// Final attempt failed
			info.CheckMessage = fmt.Sprintf("Email not found after %d attempts with %d second intervals", info.CheckRetries, info.CheckRetryInterval)
			log.Debug().
				Int("total_attempts", info.CheckRetries).
				Str("test_id", info.TestID).
				Msg("Email not found after all retry attempts")
		}
	}

	if info.ReceiveSuccess {
		log.Debug().
			Str("test_id", info.TestID).
			Str("received_at", info.ReceivedAt).
			Msg("Email receive check successful")
	}

	// Update alarm status based on email receive result
	if info.ReceiveSuccess {
		common.AlarmCheckUp("zimbra_email_receive_test", "Zimbra email receive test successful - email found in "+info.ToEmailUsername+" mailbox", false)
	} else {
		common.AlarmCheckDown("zimbra_email_receive_test", "Zimbra email receive test failed: "+info.CheckMessage, false, "", "")
	}
}

// checkEmailExists connects to IMAP and checks if the email with the test ID exists
func checkEmailExists(info *EmailSendTestInfo, attempt int) bool {
	// Connect to IMAP server
	var c *imapclient.Client
	var err error

	if info.IMAPUseTLS {
		// Connect with TLS
		c, err = imapclient.DialTLS(fmt.Sprintf("%s:%d", info.IMAPServer, info.IMAPPort), &tls.Config{
			ServerName:         info.IMAPServer,
			InsecureSkipVerify: false, // Use proper TLS verification
		})
	} else {
		// Connect without TLS
		c, err = imapclient.Dial(fmt.Sprintf("%s:%d", info.IMAPServer, info.IMAPPort))
	}

	if err != nil {
		log.Error().
			Int("attempt", attempt).
			Str("imap_server", info.IMAPServer).
			Int("imap_port", info.IMAPPort).
			Err(err).
			Msg("Failed to connect to IMAP server")
		return false
	}
	defer c.Close()

	// Login
	if err := c.Login(info.ToEmailUsername, info.ToEmailPassword); err != nil {
		log.Error().
			Int("attempt", attempt).
			Str("username", info.ToEmailUsername).
			Err(err).
			Msg("Failed to login to IMAP server")
		return false
	}

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Error().
			Int("attempt", attempt).
			Err(err).
			Msg("Failed to select INBOX")
		return false
	}

	// Search for emails with the exact subject (which contains our unique test ID)
	since := time.Now().Add(-10 * time.Minute)
	criteria := imap.NewSearchCriteria()
	criteria.Since = since
	criteria.Header.Set("Subject", info.Subject)

	log.Debug().
		Int("attempt", attempt).
		Str("subject", info.Subject).
		Str("test_id", info.TestID).
		Time("since", since).
		Msg("Searching for emails with exact subject match")

	uids, err := c.UidSearch(criteria)
	if err != nil {
		log.Error().
			Int("attempt", attempt).
			Err(err).
			Msg("Failed to search for emails")
		return false
	}

	log.Debug().
		Int("attempt", attempt).
		Int("matching_uids", len(uids)).
		Uint32("total_messages", mbox.Messages).
		Msg("Email search completed")

	if len(uids) > 0 {
		log.Debug().
			Int("attempt", attempt).
			Int("matching_emails", len(uids)).
			Str("test_id", info.TestID).
			Msg("Email found with matching subject")
		return true
	}

	log.Debug().
		Int("attempt", attempt).
		Str("subject", info.Subject).
		Str("test_id", info.TestID).
		Uint32("total_messages", mbox.Messages).
		Msg("No matching emails found")
	return false
}

// shouldRunFullCheck determines if a full health check should be performed
func shouldRunFullCheck() bool {
	// Set default cache interval if not configured
	cacheInterval := MailHealthConfig.Zimbra.Cache_interval
	if cacheInterval == 0 {
		cacheInterval = 12 // Default to 12 hours
	}

	// Load cache metadata from SQLite
	_, cachedAt, nextCheckAt, found, err := healthdb.GetJSON("zimbraHealth", "cache")
	if err != nil || !found {
		log.Debug().Msg("No zimbraHealth cache found in SQLite, running full check")
		return true
	}

	now := time.Now()
	if nextCheckAt != nil {
		if now.After(*nextCheckAt) {
			log.Debug().Time("next_check", *nextCheckAt).Msg("Cache expired, running full check")
			return true
		}
	} else {
		if now.Sub(cachedAt) >= time.Duration(cacheInterval)*time.Hour {
			log.Debug().Time("cached_at", cachedAt).Msg("Cache older than interval, running full check")
			return true
		}
	}

	log.Debug().Time("cached_at", cachedAt).Msg("Using cached data from SQLite")
	return false
}

// loadCachedData loads health data from SQLite cache
func loadCachedData() (*ZimbraHealthData, error) {
	jsonStr, _, _, found, err := healthdb.GetJSON("zimbraHealth", "cache")
	if err != nil {
		return nil, fmt.Errorf("failed to read cache from SQLite: %w", err)
	}
	if !found || jsonStr == "" {
		return nil, fmt.Errorf("cache not found")
	}
	var healthData ZimbraHealthData
	if err := json.Unmarshal([]byte(jsonStr), &healthData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}
	// Update cache info to reflect that this is from cache
	healthData.CacheInfo.FromCache = true
	log.Debug().Str("source", "sqlite").Str("last_full_check", healthData.CacheInfo.LastFullCheck).Msg("Loaded data from cache")
	return &healthData, nil
}

// saveCachedData saves health data to SQLite cache
func saveCachedData(healthData *ZimbraHealthData) error {
	// Update cache info
	now := time.Now()
	cacheInterval := MailHealthConfig.Zimbra.Cache_interval
	if cacheInterval == 0 {
		cacheInterval = 12
	}
	next := now.Add(time.Duration(cacheInterval) * time.Hour)

	healthData.CacheInfo = CacheInfo{
		Enabled:       true,
		CacheInterval: cacheInterval,
		LastFullCheck: now.Format("2006-01-02 15:04:05"),
		NextFullCheck: next.Format("2006-01-02 15:04:05"),
		FromCache:     false,
		CacheFile:     "sqlite://health.db#zimbraHealth/cache",
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(healthData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal health data: %w", err)
	}

	// Store in SQLite
	if err := healthdb.PutJSON("zimbraHealth", "cache", string(data), &next, now); err != nil {
		return fmt.Errorf("failed to store cache in SQLite: %w", err)
	}

	log.Debug().Str("source", "sqlite").Str("last_full_check", healthData.CacheInfo.LastFullCheck).Str("next_full_check", healthData.CacheInfo.NextFullCheck).Msg("Saved data to cache")
	return nil
}
