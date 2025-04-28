//go:build linux

package pmgHealth

import (
	"bytes"
	"fmt"
	"os" // Import os for file checks
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	mail "github.com/monobilisim/monokit/common/mail"
	ver "github.com/monobilisim/monokit/common/versionCheck"
	"github.com/spf13/cobra"
)

// DetectPmg checks if Proxmox Mail Gateway seems to be installed.
// It looks for the pmgversion command and the pmgproxy service.
func DetectPmg() bool {
	// 1. Check for pmgversion command
	if _, err := exec.LookPath("pmgversion"); err != nil {
		common.LogDebug("pmgHealth auto-detection failed: 'pmgversion' command not found in PATH.")
		return false
	}
	common.LogDebug("pmgHealth auto-detection: 'pmgversion' command found.")

	// 2. Check for /etc/pmg directory
	if _, err := os.Stat("/etc/pmg"); os.IsNotExist(err) {
		common.LogDebug("pmgHealth auto-detection failed: '/etc/pmg' directory not found.")
		return false
	}
	common.LogDebug("pmgHealth auto-detection: '/etc/pmg' directory found.")

	// 3. Check if pmgproxy service exists/is active (using common function)
	// We can just check for one key service. If pmgproxy is there, it's likely PMG.
	if !common.SystemdUnitExists("pmgproxy.service") {
		common.LogDebug("pmgHealth auto-detection failed: 'pmgproxy.service' systemd unit not found.")
		return false
	}
	common.LogDebug("pmgHealth auto-detection: 'pmgproxy.service' systemd unit found.")

	common.LogDebug("pmgHealth auto-detected successfully.")
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

func CheckPmgServices() {
	pmgServices := []string{"pmgproxy.service", "pmg-smtp-filter.service", "postfix@-.service"}

	for _, service := range pmgServices {
		if common.SystemdUnitActive(service) {
			common.PrettyPrintStr(service, true, "running")
			common.AlarmCheckUp(service, service+" is working again", false)
		} else {
			common.PrettyPrintStr(service, false, "running")
			common.AlarmCheckDown(service, service+" is not running", false, "", "")
		}
	}
}

func PostgreSQLStatus() {
	cmd := exec.Command("pg_isready", "-q")
	err := cmd.Run()
	if err != nil {
		common.AlarmCheckDown("postgres", "PostgreSQL is not running", false, "", "")
		common.PrettyPrintStr("PostgreSQL", false, "running")
	} else {
		common.AlarmCheckUp("postgres", "PostgreSQL is now running", false)
		common.PrettyPrintStr("PostgreSQL", true, "running")
	}
}

func QueuedMessages() {
	// Execute the mailq command
	cmd := exec.Command("mailq")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		common.LogError("Error running mailq: " + err.Error())
		common.AlarmCheckDown("mailq_run", "Error running mailq: "+err.Error(), false, "", "")
		return
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

	if count < MailHealthConfig.Pmg.Queue_Limit {
		common.AlarmCheckUp("queued_msg", "Number of queued messages is acceptable - "+strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit), false)
		common.PrettyPrintStr("Number of queued messages", true, strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit))
	} else {
		common.AlarmCheckDown("queued_msg", "Number of queued messages is above limit - "+strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit), false, "", "")
		common.PrettyPrintStr("Number of queued messages", false, strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit))
	}
}

func Main(cmd *cobra.Command, args []string) {
	version := "2.0.0"
	common.ScriptName = "pmgHealth"
	common.TmpDir = common.TmpDir + "pmgHealth"
	common.Init()
	common.ConfInit("mail", &MailHealthConfig)
	api.WrapperGetServiceStatus("pmgHealth")

	fmt.Println("PMG Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	common.SplitSection("Version Check")
	ver.ProxmoxMGCheck()

	common.SplitSection("PMG Services")
	CheckPmgServices()

	common.SplitSection("PostgreSQL Status")
	PostgreSQLStatus()

	common.SplitSection("Queued Messages")
	QueuedMessages()
}
