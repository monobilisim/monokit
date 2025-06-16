//go:build linux

package zimbraHealth

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	mail "github.com/monobilisim/monokit/common/mail"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	ver "github.com/monobilisim/monokit/common/versionCheck"
	"github.com/spf13/cobra"
)

// DetectZimbra checks for the presence of Zimbra installation directories.
func DetectZimbra() bool {
	// Check for standard Zimbra path
	if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
		common.LogDebug("Zimbra detected at /opt/zimbra.")
		return true
	}
	// Check for Carbonio/Zextras path
	if _, err := os.Stat("/opt/zextras"); !os.IsNotExist(err) {
		common.LogDebug("Zextras/Carbonio detected at /opt/zextras.")
		return true
	}
	common.LogDebug("Neither /opt/zimbra nor /opt/zextras found. Zimbra not detected.")
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
var templateFile string // Determined in collectHealthData

// Main entry point for zimbraHealth
func Main(cmd *cobra.Command, args []string) {
	// version := "2.3.0" // Removed unused variable
	common.ScriptName = "zimbraHealth"
	common.TmpDir = common.TmpDir + "zimbraHealth"
	common.Init()
	common.ConfInit("mail", &MailHealthConfig)
	api.WrapperGetServiceStatus("zimbraHealth") // Keep this for service status reporting

	// common.LogInfo("Starting Zimbra Health Check - v" + version) // Removed LogInfo

	if common.ProcGrep("install.sh", true) {
		// common.LogInfo("Installation is running. Exiting.") // Removed LogInfo
		fmt.Println("Installation is running. Exiting.") // Keep user-facing message
		return
	}

	// Collect health data
	healthData := collectHealthData()

	// Display as a nice box UI
	displayBoxUI(healthData)

	// Run background/periodic tasks separately
	runPeriodicTasks(healthData) // Pass healthData if needed for context
}

// collectHealthData gathers all the health information
func collectHealthData() *ZimbraHealthData {
	healthData := NewZimbraHealthData()
	healthData.System.LastChecked = time.Now().Format("2006-01-02 15:04:05")

	// Determine Zimbra/Zextras path and set basic system info
	if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
		zimbraPath = "/opt/zimbra"
		healthData.System.ProductPath = zimbraPath
	} else if _, err := os.Stat("/opt/zextras"); !os.IsNotExist(err) {
		zimbraPath = "/opt/zextras"
		healthData.System.ProductPath = zimbraPath
	} else {
		common.LogError("Neither /opt/zimbra nor /opt/zextras found. Cannot proceed.")
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
	date := time.Now().Format("15:04") // Use 15:04 for HH:MM format
	// Get env variable ZIMBRA_HEALTH_TEST_ZMFIXPERMS
	if date == "01:00" || (os.Getenv("ZIMBRA_HEALTH_TEST_ZMFIXPERMS") == "true" || os.Getenv("ZIMBRA_HEALTH_TEST_ZMFIXPERMS") == "1") {
		// common.LogInfo("Running scheduled 01:00 tasks...") // Removed LogInfo
		// common.SplitSection("Running zmfixperms") // Removed SplitSection
		Zmfixperms() // Zmfixperms has its own logging
		// Note: SSL check data is already collected in collectHealthData for UI display.
		// The Redmine issue creation logic within CheckSSL might still be relevant here
		// if it should only happen at 01:00. Consider refactoring CheckSSL further if needed.
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
		common.LogError("Error opening file: " + err.Error())
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
			common.LogWarn("Could not match ID in webhook log line: " + line) // Changed to Warn
			continue                                                          // Skip this line if ID not found
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
			common.LogWarn(fmt.Sprintf("Could not parse quota percentage '%s': %v", matches[3], err))
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
				common.LogError("Failed to create webhook marker file " + markerFile + ": " + createErr.Error())
				// Continue without marker, might re-alarm
			} else {
				markerHandle.Close() // Close file handle immediately after creation
			}

			// Send the alarm
			common.LogWarn("Webhook quota limit exceeded: " + line) // Log the event as Warn
			common.Alarm("[zimbraHealth - "+common.Config.Identifier+"] Quota limit is above "+strconv.Itoa(quotaLimit)+"% "+escapeJSON(line), MailHealthConfig.Zimbra.Webhook_tail.Stream, MailHealthConfig.Zimbra.Webhook_tail.Topic, true)
		}
	}

	if err := scanner.Err(); err != nil {
		common.LogError("Error reading webhook file: " + err.Error())
	}
} // <-- Correct end of TailWebhook

// Zmfixperms remains unchanged, but remove internal LogInfo
func Zmfixperms() {
	// common.LogInfo("Running zmfixperms...") // Removed LogInfo
	out, err := ExecZimbraCommand("libexec/zmfixperms", true, true)
	
	if err != nil {
		common.Alarm("["+common.Config.Identifier+"] Zmfixperms failed: \n```spoiler Error\n"+err.Error()+"\n```", MailHealthConfig.Zimbra.Zmfixperms.Stream, MailHealthConfig.Zimbra.Zmfixperms.Topic, true)
	} else {
		_, _ := ExecZimbraCommand("zmcontrol restart", false, false) // Restart Zimbra services after zmfixperms
		secondOut, _ := ExecZimbraCommand("zmcontrol status", false, false)
		common.Alarm("["+common.Config.Identifier+"] Zmfixperms completed successfully: \n```spoiler Zmfixperms Output\n"+out+"\n``` ```spoiler zmcontrol status output\n" + secondOut + "\n```", MailHealthConfig.Zimbra.Zmfixperms.Stream, MailHealthConfig.Zimbra.Zmfixperms.Topic, true)
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
		common.LogError(info.Message)
		return info
	}
	if strings.Contains(zimbraPath, "zextras") {
		productName = "carbonio"
	} else {
		productName = "zimbra"
	}

	certFile = zimbraPath + "/ssl/" + productName + "/server/server.crt"
	keyFile = zimbraPath + "/ssl/" + productName + "/server/server.key"

	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		info.Message = "Nginx template file not found: " + templateFile
		common.LogError(info.Message)
		return info
	}

	// Determine IP Address
	if _, err := os.Stat(zimbraPath + "/conf/nginx/external_ip.txt"); !os.IsNotExist(err) {
		fileContent, err := os.ReadFile(zimbraPath + "/conf/nginx/external_ip.txt")
		if err != nil {
			common.LogWarn("Error reading external_ip.txt: " + err.Error() + ". Falling back to ifconfig.co.")
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
			common.LogError(info.Message)
			return info
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			info.Message = "Error reading external IP response: " + err.Error()
			common.LogError(info.Message)
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
		common.LogError(info.Message)
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
		common.LogError(info.Message)
		return info
	}

	re = regexp.MustCompile(regexPattern)
	matches = re.FindAllString(string(templateContent), -1)
	if len(matches) > 0 {
		output = strings.ReplaceAll(matches[0], "", "\n") // Keep this logic? Seems unused.
	}

	if output == "" { // Block not found
		common.LogWarn("Adding proxy control block in " + templateFile + " file...") // Changed to Warn as it's modifying config
		fileHandle, err := os.OpenFile(templateFile, os.O_APPEND|os.O_WRONLY, 0644)  // Don't create if not exists
		if err != nil {
			info.Message = fmt.Sprintf("Error opening template file for append: %v", err)
			common.LogError(info.Message)
			return info
		}
		defer fileHandle.Close()
		if _, err := fileHandle.WriteString("\n" + proxyBlock + "\n"); err != nil {
			info.Message = fmt.Sprintf("Error writing proxy block to template file: %v", err)
			common.LogError(info.Message)
			return info
		}
		common.LogWarn("Proxy control block added. Restarting proxy service...") // Changed to Warn
		// Consider if restart should happen here or be handled separately
		_, err = ExecZimbraCommand("zmproxyctl restart", false, false)
		if err != nil {
			common.LogWarn("Failed to restart proxy after adding IP block: " + err.Error())
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
		common.LogError(info.Message)
		common.AlarmCheckDown("accesswithip", "Check failed: "+info.Message, false, "", "")
		return info
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		info.Message = "HTTP request failed: " + err.Error()
		common.LogError(info.Message)
		common.AlarmCheckDown("accesswithip", "Cannot access IP "+info.IPAddress+": "+err.Error(), false, "", "")
		return info
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		info.Message = "Failed to read response body: " + err.Error()
		common.LogError(info.Message)
		common.AlarmCheckDown("accesswithip", "Check failed reading response from "+info.IPAddress+": "+err.Error(), false, "", "")
		return info
	}

	bodyStr := string(body)
	info.CheckStatus = true // Check was performed

	if !strings.Contains(bodyStr, message) {
		// This means the IP access IS possible (didn't get the specific message)
		info.Accessible = false // For data structure: false means accessible
		info.Message = "Direct access via IP is possible."
		common.LogWarn(info.Message)
		common.AlarmCheckDown("accesswithip", "Can access Zimbra through plain IP: "+info.IPAddress, false, "", "")
	} else {
		// This means the IP access IS blocked (got the specific message)
		info.Accessible = true // For data structure: true means blocked/not accessible
		info.Message = "Direct access via IP is blocked."
		// common.LogInfo(info.Message) // Removed LogInfo
		common.AlarmCheckUp("accesswithip", "Cannot access Zimbra through plain IP: "+info.IPAddress, false)
	}

	return info
}

// RestartZimbraService refactored to avoid recursion and direct call to CheckZimbraServices
func RestartZimbraService(service string) bool {
	if restartCounter >= MailHealthConfig.Zimbra.Restart_Limit { // Use >= for clarity
		common.LogWarn("Restart limit reached for " + service + ". Not restarting.")
		common.AlarmCheckDown("service_restart_limit_"+service, "Restart limit reached for "+service, false, "", "")
		return false
	}

	common.LogWarn("Attempting to restart Zimbra services (triggered by failure in " + service + ")...") // Changed to Warn
	_, err := ExecZimbraCommand("zmcontrol start", false, false)                                         // Try starting all services

	if err != nil {
		common.LogError("Error attempting to start Zimbra services: " + err.Error())
		common.AlarmCheckDown("service_restart_failed_"+service, "Failed to execute zmcontrol start for "+service+": "+err.Error(), false, "", "")
		return false // Restart command failed
	}

	restartCounter++
	common.LogWarn(fmt.Sprintf("Zimbra services restart attempted (%d/%d). Re-check will occur in the next cycle.", restartCounter, MailHealthConfig.Zimbra.Restart_Limit)) // Changed to Warn
	// Do not call CheckZimbraServices recursively. Let the next run verify.
	return true // Restart command executed
}

// CheckZimbraServices refactored to return []ServiceInfo with recovery tracking
func CheckZimbraServices() []ServiceInfo {
	var currentServices []ServiceInfo
	currentStatus := make(map[string]bool)

	statusOutput, err := ExecZimbraCommand("zmcontrol status", false, false)
	if err != nil {
		common.LogError("Failed to get zmcontrol status: " + err.Error())
		return currentServices
	}

	// Process current status
	lines := strings.Split(statusOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Host") {
			continue // Skip empty lines and header
		}

		// Normalize spacing and check status
		svc := strings.Join(strings.Fields(line), " ")
		var serviceName string
		var isRunning bool

		if strings.Contains(svc, "Running") {
			isRunning = true
			serviceName = strings.TrimSpace(strings.Split(svc, "Running")[0])
		} else if strings.Contains(svc, "Stopped") { // Handle "Stopped" status
			isRunning = false
			serviceName = strings.TrimSpace(strings.Split(svc, "Stopped")[0])
		} else if strings.Contains(svc, "is not running") {
			isRunning = false
			serviceName = strings.TrimSpace(strings.Split(svc, "is not running")[0])
		} else {
			common.LogWarn("Could not parse service status line: " + line)
			continue
		}

		// Clean up potential Carbonio prefixes like "service carbonio-"
		serviceName = strings.TrimPrefix(serviceName, "service ")
		serviceName = strings.TrimPrefix(serviceName, "carbonio-")

		currentServices = append(currentServices, ServiceInfo{Name: serviceName, Running: isRunning})
		currentStatus[serviceName] = isRunning

		// Handle down services for restart logic (existing behavior)
		if !isRunning {
			common.LogWarn(serviceName + " is NOT running.")
			common.WriteToFile(common.TmpDir+"/"+"zmcontrol_status_"+time.Now().Format("2006-01-02_15.04.05")+".log", statusOutput)
			common.AlarmCheckDown("service_"+serviceName, serviceName+" is not running", false, "", "")
			if MailHealthConfig.Zimbra.Restart {
				RestartZimbraService(serviceName) // Attempt restart
			}
		}
	}

	// Process state changes and emit recovery alarms
	allServiceStates := processServiceStateChanges(common.TmpDir, currentStatus)

	// Display summary
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
		common.LogDebug("No service failures recorded.")
		return
	}

	common.LogDebug("Service Recovery Summary:")
	common.LogDebug("================================================================================")
	common.LogDebug(fmt.Sprintf("%-15s %-20s %-10s %-20s %-10s",
		"Service", "Last Failure", "Restarts", "Recovery Time", "Status"))
	common.LogDebug("================================================================================")

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

		common.LogDebug(fmt.Sprintf("%-15s %-20s %-10s %-20s %-10s",
			state.Name, lastFailure, restartInfo, recoveryTime, statusIcon))
	}
	common.LogDebug("================================================================================")
}

// changeImmutable remains unchanged
func changeImmutable(filePath string, add bool) {
}

// modifyFile remains unchanged (called by CheckZPush if needed)
func modifyFile(templateFile string) {
	// Read the file content
	content, err := ioutil.ReadFile(templateFile)
	if err != nil {
		common.LogError("Error reading file: " + err.Error())
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
	if err := ioutil.WriteFile(templateFile, []byte(modifiedBlock), 0644); err != nil {
		common.LogError("Error writing to file: " + err.Error())
	}

	common.LogWarn("Added Z-Push block to " + templateFile + " file, restarting zimbra proxy service...") // Changed to Warn
	_, err = ExecZimbraCommand("zmproxyctl restart", false, false)
	if err != nil {
		common.LogError("Error restarting zimbra proxy service: " + err.Error())
	}
}

// ExecZimbraCommand remains unchanged
func ExecZimbraCommand(command string, fullPath bool, runAsRoot bool) (string, error) {
	zimbraUser := "zimbra"

	// Check if zimbra user exists
	cmd := exec.Command("id", "zimbra")
	err := cmd.Run()
	if err != nil {
		zimbraUser = "zextras"
	}

	if runAsRoot {
		zimbraUser = "root"
	}

	cmd = nil

	// Execute command
	if fullPath {
		cmd = exec.Command("/bin/su", zimbraUser, "-c", zimbraPath+"/"+command)
	} else {
		cmd = exec.Command("/bin/su", zimbraUser, "-c", zimbraPath+"/bin/"+command)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	cmd.Run()

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
		common.LogError(info.Message)
		common.AlarmCheckDown("zpush", "Check failed: "+info.Message, false, "", "")
		return info
	}

	resp, err := client.Do(req)
	if err != nil {
		info.Message = "Error performing Z-Push request: " + err.Error()
		common.LogError(info.Message)
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
		// common.LogInfo("Z-Push headers detected.") // Removed LogInfo
		common.AlarmCheckUp("zpush", "Z-Push is responding correctly", false)
	} else {
		info.Message = "Z-Push headers not found in response."
		common.LogWarn(info.Message)
		common.AlarmCheckDown("zpush", "Z-Push headers not found", false, "", "")
	}

	if !info.NginxConfig {
		common.LogWarn("Z-Push Nginx config file (/etc/nginx-php-fpm.conf) not found.")
		// Alarm? Or just log? Depends on requirements.
	} else {
		// common.LogInfo("Z-Push Nginx config file found.") // Removed LogInfo
		// Check if modification is needed (this logic might need refinement)
		contentBytes, _ := ioutil.ReadFile(templateFile) // Read template again
		if !strings.Contains(string(contentBytes), "nginx-php-fpm.conf") {
			common.LogWarn("Nginx template needs Z-Push include. Modifying...") // Changed to Warn
			modifyFile(templateFile)                                            // This function handles its own logging/errors
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
		common.LogError(info.Message)
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
			// common.LogInfo("Mail queue is empty.") // Removed LogInfo
		} else {
			info.Message = "Error running mailq: " + err.Error() + " - Output: " + out.String()
			common.LogError(info.Message)
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
			common.LogError(info.Message)
			common.AlarmCheckDown("mailq_error", "Failed to read mailq output: "+err.Error(), false, "", "")
			return info
		}
		info.Count = count
		info.CheckStatus = true
	}

	// Process results if check was successful
	if info.CheckStatus {
		info.Exceeded = info.Count > info.Limit
		// common.LogInfo(fmt.Sprintf("Queued messages: %d (Limit: %d)", info.Count, info.Limit)) // Removed LogInfo

		if info.Exceeded {
			common.LogWarn("Mail queue limit exceeded.")
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
		common.LogError(info.Message)
		return info
	}
	zmHostname = strings.TrimSpace(zmHostname)

	// Get service hostname for the mail host
	provOutput, err := ExecZimbraCommand("zmprov gs "+zmHostname+" zimbraServiceHostname", false, false)
	if err != nil {
		info.Message = "Error getting zimbraServiceHostname: " + err.Error()
		common.LogError(info.Message)
		return info
	}

	// Parse zimbraServiceHostname
	re := regexp.MustCompile(`zimbraServiceHostname:\s*(.*)`)
	matches := re.FindStringSubmatch(provOutput)
	if len(matches) < 2 {
		info.Message = "Could not parse zimbraServiceHostname from zmprov output: " + provOutput
		common.LogError(info.Message)
		// Fallback to zmHostname itself?
		info.MailHost = zmHostname
		common.LogWarn("Falling back to using zmhostname for SSL check.")
		// return info // Decide if fallback is acceptable or should fail
	} else {
		info.MailHost = strings.TrimSpace(matches[1])
	}

	if info.MailHost == "" {
		info.Message = "Mail host for SSL check could not be determined."
		common.LogError(info.Message)
		return info
	}

	common.LogDebug("Checking SSL certificate for host: " + info.MailHost)

	// Dial to get certificate
	conn, err := tls.Dial("tcp", info.MailHost+":443", &tls.Config{
		InsecureSkipVerify: true,          // Keep insecure for self-signed/internal CAs often used
		ServerName:         info.MailHost, // Important for SNI
	})
	if err != nil {
		info.Message = "Error connecting to " + info.MailHost + ":443 for SSL check: " + err.Error()
		common.LogError(info.Message)
		common.AlarmCheckDown("sslcert_conn", "Failed to connect for SSL check: "+err.Error(), false, "", "")
		return info
	}
	defer conn.Close()

	// Check peer certificates
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		info.Message = "No SSL certificates found for " + info.MailHost
		common.LogError(info.Message)
		common.AlarmCheckDown("sslcert_nocert", info.Message, false, "", "")
		return info
	}

	cert := certs[0]
	info.CheckStatus = true // Check performed successfully

	// Calculate days until expiry
	info.DaysUntilExpiry = int(time.Until(cert.NotAfter).Hours() / 24)
	info.ExpiringSoon = info.DaysUntilExpiry < expiryThresholdDays

	// common.LogInfo(fmt.Sprintf("SSL Certificate for %s expires in %d days.", info.MailHost, info.DaysUntilExpiry)) // Removed LogInfo

	// Handle alarms and Redmine issues
	alarmMsg := fmt.Sprintf("SSL Certificate for %s expires in %d days", info.MailHost, info.DaysUntilExpiry)
	if info.ExpiringSoon {
		common.LogWarn("SSL Certificate is expiring soon.")
		common.AlarmCheckDown("sslcert", alarmMsg, false, "", "")
		// Only create/update Redmine issue if expiring soon
		viewDeployedCert, err := ExecZimbraCommand("zmcertmgr viewdeployedcrt", false, false)
		if err != nil {
			common.LogWarn("Failed to get deployed cert details for Redmine issue: " + err.Error())
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
