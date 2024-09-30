//go:build linux

package rabbitmq

import (
	"fmt"
	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

var Config struct {
	User     string
	Password string
}

func Main(cmd *cobra.Command, args []string) {
	version := "0.1.0"
	common.ScriptName = "rabbitmqHealth"
	common.TmpDir = common.TmpDir + "rabbitmqHealth"
	common.Init()
	//common.ConfInit("rabbitmq", &Config)

	fmt.Println("RabbitMQ Health - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	common.SplitSection("rabbitmq-server Service")

	if common.SystemdUnitActive("rabbitmq-server.service") == false {
		common.PrettyPrintStr("Service rabbitmq-server", false, "active")
		common.AlarmCheckDown("rabbitmq_server", "Service rabbitmq-server is not active")
	} else {
		common.PrettyPrintStr("Service rabbitmq-server", true, "active")
		common.AlarmCheckUp("rabbitmq_server", "Service rabbitmq-server is now active")
	}

	common.SplitSection("Port 5672")
	checkPort("5672")

	checkEnabledPlugins()
}

func checkPort(port string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", port), 5*time.Second)
	if err != nil {
		common.PrettyPrintStr("Port "+port, false, "active")
		common.AlarmCheckDown("rabbitmq_port_"+port, "Port "+port+" is not active")
		return
	}
	_ = conn.Close()
	common.PrettyPrintStr("Port "+port, true, "active")
	common.AlarmCheckUp("rabbitmq_port_"+port, "Port "+port+" is now active")
}

func checkEnabledPlugins() {
	common.SplitSection("RabbitMQ Management")

	filePath := "/etc/rabbitmq/enabled_plugins"
	searchString := "[rabbitmq_management]."

	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Failed to read file %s: %v", filePath, err)
		return
	}

	fileContent := string(content)

	found := strings.Contains(fileContent, searchString)

	if found {
		message := fmt.Sprintf("Found '%s' in file %s\n", searchString, filePath)
		common.PrettyPrintStr("RabbitMQ Management", true, "active")
		common.AlarmCheckUp("rabbitmq_management", message)
		checkPort("15672")
	} else {
		message := fmt.Sprintf("Did not find '%s' in file %s\n", searchString, filePath)
		common.PrettyPrintStr("RabbitMQ Management", false, "active")
		common.AlarmCheckDown("rabbitmq_management", message)
	}
}

func checkCliPing() {
	cmdStruct := exec.Command("/usr/sbin/rabbitmqctl", "ping")
	out, err := cmdStruct.Output()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(out))
}
