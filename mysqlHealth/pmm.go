//go:build linux

package mysqlHealth

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/monobilisim/monokit/common"
)

// checkPMM checks if the PMM is installed
// and creates alarm if it is not healthy
func checkPMM() {
	notInstalled := `
dpkg-query: package 'pmm2-client' is not installed and no information is available
Use dpkg --info (= dpkg-deb --info) to examine archive files.
    `
	dpkgNotFound := `exec: "dpkg": executable file not found in $PATH`
	cmd := exec.Command("dpkg", "-s", "pmm2-client")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	// If the dpkg command fails, check if the error is because the package is not installed
	// or because the dpkg command is not found
	if err != nil {
		if strings.TrimSpace(stderr.String()) == strings.TrimSpace(notInstalled) || strings.TrimSpace(err.Error()) == strings.TrimSpace(dpkgNotFound) {
			return
		}
		common.LogError(fmt.Sprintf("Error executing dpkg command: %v\n", err))
		return
	}

	output := out.String()
	lines := strings.Split(output, "\n")
	var isInstalled bool
	// Check if the PMM is installed
	for _, line := range lines {
		if strings.Contains(line, "Status:") {
			status := strings.TrimSpace(strings.Split(line, ":")[1])
			if strings.HasPrefix(status, "install ok installed") {
				isInstalled = true
			}
			break
		}
	}

	// If the PMM is installed, check if the service is active
	// and create alarm if it is not active
	if isInstalled {
		common.SplitSection("PMM Status:")
		if common.SystemdUnitActive("pmm-agent.service") {
			common.PrettyPrintStr("Service pmm-agent", true, "active")
			common.AlarmCheckDown("mysql-pmm-agent", "Service pmm-agent", false, "", "")
		} else {
			common.PrettyPrintStr("Service pmm-agent", false, "active")
			common.AlarmCheckUp("mysql-pmm-agent", "Service pmm-agent", false)
		}
	}
}
