//go:build linux

package pmgHealth

import (
	"bytes"
	"fmt"
	"os" // Import os for file checks
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	mail "github.com/monobilisim/monokit/common/mail"
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
	api.WrapperGetServiceStatus("pmgHealth")

	// Collect all health data with skipOutput=true since we'll use our UI rendering
	healthData := CheckPmgHealth(true)

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
