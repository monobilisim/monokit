//go:build linux
package pmgHealth

import (
    "fmt"
    "time"
    "bytes"
    "regexp"
    "strconv"
    "os/exec"
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/common"
    mail "github.com/monobilisim/monokit/common/mail"
)


var MailHealthConfig mail.MailHealth

func CheckPmgServices() {
    pmgServices := []string{"pmgproxy.service", "pmg-smtp-filter.service", "postfix@-.service"}

    for _, service := range pmgServices {
        if common.SystemdUnitActive(service) {
            common.PrettyPrintStr(service, true, "running")
            common.AlarmCheckUp(service, service + " is working again", false)
        } else {
            common.PrettyPrintStr(service, false, "running")
            common.AlarmCheckDown(service, service + " is not running", false, "", "")
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
        common.AlarmCheckDown("mailq_run", "Error running mailq: " + err.Error(), false, "", "")
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
        common.AlarmCheckUp("queued_msg", "Number of queued messages is acceptable - " + strconv.Itoa(count) + "/" + strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit), false)
        common.PrettyPrintStr("Number of queued messages", true, strconv.Itoa(count) + "/" + strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit))
    } else {
        common.AlarmCheckDown("queued_msg", "Number of queued messages is above limit - " + strconv.Itoa(count) + "/" + strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit), false, "", "")
        common.PrettyPrintStr("PMG Queue", true, strconv.Itoa(count) + "/" + strconv.Itoa(MailHealthConfig.Pmg.Queue_Limit))
    }
}

func Main(cmd *cobra.Command, args []string) {
    version := "2.0.0"
    common.ScriptName = "pmgHealth"
    common.TmpDir = common.TmpDir + "pmgHealth"
    common.Init()
    common.ConfInit("mail", &MailHealthConfig)

    fmt.Println("PMG Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

    common.SplitSection("PMG Services")
    CheckPmgServices()

    common.SplitSection("PostgreSQL Status")
    PostgreSQLStatus()

    common.SplitSection("Queued Messages")
    QueuedMessages()
}
