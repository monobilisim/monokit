package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/k8sHealth"
	"github.com/monobilisim/monokit/osHealth"
	"github.com/monobilisim/monokit/pritunlHealth"
	"github.com/monobilisim/monokit/wppconnectHealth"
	"github.com/spf13/cobra"
)

type HealthCheck struct {
	Name    string // Name of the health check, eg. mysqld
	Enabled bool
}

type Daemon struct {
	Frequency     int  // Frequency to run health checks
	Debug         bool // Debug mode
	Health_Checks []HealthCheck
}

var DaemonConfig Daemon

type Component struct {
	Name        string
	Command     string
	ConfigCheck bool
}

var components = []Component{
	{"pritunl", "pritunl", false},
	{"postal", "postal", false},
	{"pmg", "pmgversion", false},
	{"k8s", "k8s", true},
	{"mysql", "mysqld", false}, // Special case handled in loop
	{"redis", "redis-server", false},
	{"rabbitmq", "rabbitmq-server", false},
	{"traefik", "traefik", false},
	{"wppconnect", "wppconnect", true},
}

func IsEnabled(name string) (bool, bool) {
	for _, hc := range DaemonConfig.Health_Checks {
		if hc.Name == name {
			return true, hc.Enabled
		}
	}

	return false, false
}

func CommExists(command string, confCheckOnly bool) bool {
	path, _ := exec.LookPath(command)

	existsOnConfig, enabled := IsEnabled(command)

	if existsOnConfig {
		return enabled
	}

	if path != "" && !confCheckOnly {
		return true
	}

	return false

}

func Main(cmd *cobra.Command, args []string) {
	version := "1.0.0"
	common.Init()

	if common.ConfExists("daemon") {
		common.ConfInit("daemon", &DaemonConfig)
	} else {
		DaemonConfig.Frequency = 60
	}

	fmt.Println("Monokit daemon - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	runOnce, _ := cmd.Flags().GetBool("once")
	listComponents, _ := cmd.Flags().GetBool("list-components")

	if runOnce {
		fmt.Println("Running once")
		RunAll()
		os.Exit(0)
	}

	if listComponents {
		fmt.Print(common.GetInstalledComponents())
		common.RemoveLockfile()
		os.Exit(0)
	}

	for {
		RunAll()
		time.Sleep(time.Duration(DaemonConfig.Frequency) * time.Second)
	}
}

func RunAll() {
	common.Update("", false)

	// Run OS Health check always
	var osHealthCmd = &cobra.Command{
		Run:                osHealth.Main,
		DisableFlagParsing: true,
	}
	osHealthCmd.ExecuteC()

	// Run checks based on installed components
	installed := strings.Split(common.GetInstalledComponents(), "::")
	for _, comp := range installed {
		switch comp {
		case "pritunl":
			var pritunlHealthCmd = &cobra.Command{
				Run:                pritunlHealth.Main,
				DisableFlagParsing: true,
			}
			pritunlHealthCmd.ExecuteC()
		case "postal":
			PostalCommandExecute()
		case "pmg":
			PmgCommandExecute()
		case "k8s":
			var k8sHealthCmd = &cobra.Command{
				Run:                k8sHealth.Main,
				DisableFlagParsing: true,
			}
			k8sHealthCmd.ExecuteC()
		case "mysql":
			MysqlCommandExecute()
		case "redis":
			RedisCommandExecute()
		case "rabbitmq":
			RmqCommandExecute()
		case "traefik":
			TraefikCommandExecute()
		case "wppconnect":
			wppconnectHealthCmd := &cobra.Command{
				Run:                wppconnectHealth.Main,
				DisableFlagParsing: true,
			}
			wppconnectHealthCmd.ExecuteC()
		}
	}
}
