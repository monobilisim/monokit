//go:build linux

package rmqHealth

import (
	"fmt"
	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra"
	"net"
	"os"
	"strings"
	"time"
    "github.com/michaelklishin/rabbit-hole/v2"
)

var Config struct {
	User     string
	Password string
}

var rabbitmqClient *rabbithole.Client

func newRabbitMQClient() {
    var err error
    rabbitmqClient, err = rabbithole.NewClient("http://localhost:15672", Config.User, Config.Password)
    if err != nil {
        common.PrettyPrintStr("Management API", false, "reachable")
        common.AlarmCheckDown("rabbitmq_management_api", "Failed to create RabbitMQ client; \n```" + err.Error() + "\n```", false, "", "")
    } else {
        common.PrettyPrintStr("Management API", true, "reachable")
        common.AlarmCheckUp("rabbitmq_management_api", "RabbitMQ management API is now reachable", false)
    }
}

func overviewCheck() {
    _, err := rabbitmqClient.Overview()
    if err != nil {
        common.PrettyPrintStr("Overview", false, "reachable")
        common.AlarmCheckDown("rabbitmq_overview", "Failed to get RabbitMQ overview; \n```" + err.Error() + "\n```", false, "", "")
    } else {
        common.PrettyPrintStr("Overview", true, "reachable")
        common.AlarmCheckUp("rabbitmq_overview", "RabbitMQ overview is now reachable", false)
    }
}

func serviceCheck() {
    common.SplitSection("Service")

    if common.SystemdUnitActive("rabbitmq-server.service") == false {
        common.PrettyPrintStr("rabbitmq-server", false, "active")
        common.AlarmCheckDown("rabbitmq_server", "Service rabbitmq-server is not active", false, "", "")
    } else {
        common.PrettyPrintStr("rabbitmq-server", true, "active")
        common.AlarmCheckUp("rabbitmq_server", "Service rabbitmq-server is now active", false)
    }
}

func clusterCheck() {
    common.SplitSection("Cluster")
    
    nodeList, err := rabbitmqClient.ListNodes()

    if err != nil {
        common.PrettyPrintStr("Node list", false, "reachable")
        common.AlarmCheckDown("rabbitmq_nodelist", "Failed to get RabbitMQ cluster node list; \n```" + err.Error() + "\n```", false, "", "")
    } else {
        common.PrettyPrintStr("Node list", true, "reachable")
        common.AlarmCheckUp("rabbitmq_nodelist", "RabbitMQ cluster node list is now reachable", false)
    }

    for _, node := range nodeList {
        if node.IsRunning {
            common.PrettyPrintStr("Node "+node.Name, true, "active")
            common.AlarmCheckUp("rabbitmq_node_"+node.Name, "Node "+node.Name+" is now active", false)
        } else {
            common.PrettyPrintStr("Node "+node.Name, false, "active")
            common.AlarmCheckDown("rabbitmq_node_"+node.Name, "Node "+node.Name+" is not active", false, "", "")
        }
    }
}

func Main(cmd *cobra.Command, args []string) {
	version := "0.1.0"
	common.ScriptName = "rmqHealth"
	common.TmpDir = common.TmpDir + "rmqHealth"
	common.Init()

    if common.ConfExists("rabbitmq") {
        common.ConfInit("rabbitmq", &Config)
    }

    if Config.User == "" {
        Config.User = "guest"
    }

    if Config.Password == "" {
        Config.Password = "guest"
    }

	fmt.Println("RabbitMQ Health - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

    serviceCheck()
    

	common.SplitSection("Sanity checks")
	checkPort("5672")
	checkEnabledPlugins()
    newRabbitMQClient()
    
    common.SplitSection("API")
    overviewCheck()
    clusterCheck()

}

func checkPort(port string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", port), 5*time.Second)
	if err != nil {
		common.PrettyPrintStr("Port "+port, false, "active")
		common.AlarmCheckDown("rabbitmq_port_"+port, "Port "+port+" is not active", false, "", "")
		return
	}
	_ = conn.Close()
	common.PrettyPrintStr("Port "+port, true, "active")
	common.AlarmCheckUp("rabbitmq_port_"+port, "Port "+port+" is now active", false)
}

func checkEnabledPlugins() {
	common.SplitSection("Management")

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
		common.PrettyPrintStr("Management", true, "active")
		common.AlarmCheckUp("rabbitmq_management", message, false)
		checkPort("15672")
	} else {
		message := fmt.Sprintf("Did not find '%s' in file %s\n", searchString, filePath)
		common.PrettyPrintStr("Management", false, "active")
		common.AlarmCheckDown("rabbitmq_management", message, false, "", "")
	}
}
