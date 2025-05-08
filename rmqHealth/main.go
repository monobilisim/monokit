//go:build linux

package rmqHealth

import (
	"fmt"
	"net"
	"os"
	"os/exec" // Import os/exec for command checks
	"strings"
	"time"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/spf13/cobra"
)

// DetectRmq checks if RabbitMQ seems to be installed.
// It looks for the rabbitmqctl command and the rabbitmq-server service.
func DetectRmq() bool {
	// 1. Check for rabbitmqctl command
	if _, err := exec.LookPath("rabbitmqctl"); err != nil {
		common.LogDebug("rmqHealth auto-detection failed: 'rabbitmqctl' command not found in PATH.")
		return false
	}
	common.LogDebug("rmqHealth auto-detection: 'rabbitmqctl' command found.")

	// 2. Check for /etc/rabbitmq directory
	if _, err := os.Stat("/etc/rabbitmq"); os.IsNotExist(err) {
		common.LogDebug("rmqHealth auto-detection failed: '/etc/rabbitmq' directory not found.")
		return false
	}
	common.LogDebug("rmqHealth auto-detection: '/etc/rabbitmq' directory found.")

	// 3. Check if rabbitmq-server service exists (using common function)
	if !common.SystemdUnitExists("rabbitmq-server.service") {
		common.LogDebug("rmqHealth auto-detection failed: 'rabbitmq-server.service' systemd unit not found.")
		return false
	}
	common.LogDebug("rmqHealth auto-detection: 'rabbitmq-server.service' systemd unit found.")

	common.LogDebug("rmqHealth auto-detected successfully.")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "rmqHealth", // Name used in config/daemon loop
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectRmq, // Add the auto-detect function
	})
}

var Config struct {
	User     string
	Password string
}

var rabbitmqClient *rabbithole.Client
var healthData *RmqHealthData

func checkRabbitMQClient() {
	var err error
	rabbitmqClient, err = rabbithole.NewClient("http://localhost:15672", Config.User, Config.Password)
	if err != nil {
		common.AlarmCheckDown("rabbitmq_management_api", "Failed to create RabbitMQ client; \n```"+err.Error()+"\n```", false, "", "")
		healthData.API.Connected = false
	} else {
		common.AlarmCheckUp("rabbitmq_management_api", "RabbitMQ management API is now reachable", false)
		healthData.API.Connected = true
	}
}

func checkOverview() {
	_, err := rabbitmqClient.Overview()
	if err != nil {
		common.AlarmCheckDown("rabbitmq_overview", "Failed to get RabbitMQ overview; \n```"+err.Error()+"\n```", false, "", "")
		healthData.API.OverviewOK = false
	} else {
		common.AlarmCheckUp("rabbitmq_overview", "RabbitMQ overview is now reachable", false)
		healthData.API.OverviewOK = true
	}
}

func checkService() {
	serviceActive := common.SystemdUnitActive("rabbitmq-server.service")
	healthData.Service.Active = serviceActive

	if !serviceActive {
		common.AlarmCheckDown("rabbitmq_server", "Service rabbitmq-server is not active", false, "", "")
		healthData.IsHealthy = false
	} else {
		common.AlarmCheckUp("rabbitmq_server", "Service rabbitmq-server is now active", false)
	}
}

func checkCluster() {
	nodeList, err := rabbitmqClient.ListNodes()

	if err != nil {
		common.AlarmCheckDown("rabbitmq_nodelist", "Failed to get RabbitMQ cluster node list; \n```"+err.Error()+"\n```", false, "", "")
		healthData.IsHealthy = false
		return
	} else {
		common.AlarmCheckUp("rabbitmq_nodelist", "RabbitMQ cluster node list is now reachable", false)
	}

	// Reset the cluster nodes
	healthData.Cluster.Nodes = []NodeInfo{}
	allNodesHealthy := true

	for _, node := range nodeList {
		nodeInfo := NodeInfo{
			Name:      node.Name,
			IsRunning: node.IsRunning,
		}

		healthData.Cluster.Nodes = append(healthData.Cluster.Nodes, nodeInfo)

		if node.IsRunning {
			common.AlarmCheckUp("rabbitmq_node_"+node.Name, "Node "+node.Name+" is now active", false)
		} else {
			common.AlarmCheckDown("rabbitmq_node_"+node.Name, "Node "+node.Name+" is not active", false, "", "")
			allNodesHealthy = false
		}
	}

	healthData.Cluster.IsHealthy = allNodesHealthy
	if !allNodesHealthy {
		healthData.IsHealthy = false
	}
}

func Main(cmd *cobra.Command, args []string) {
	version := "0.2.0" // Updated version number
	common.ScriptName = "rmqHealth"
	common.TmpDir = common.TmpDir + "rmqHealth"
	common.Init()

	// Initialize config
	if common.ConfExists("rabbitmq") {
		common.ConfInit("rabbitmq", &Config)
	}

	if Config.User == "" {
		Config.User = "guest"
	}

	if Config.Password == "" {
		Config.Password = "guest"
	}

	// Initialize health data
	healthData = NewRmqHealthData()
	healthData.Version = version

	api.WrapperGetServiceStatus("rmqHealth")

	// Check service status
	checkService()

	// Check ports
	checkPort("5672", true) // AMQP port

	// Check management plugin and port
	checkEnabledPlugins()

	// Check management API if plugin is enabled
	if healthData.Management.Enabled {
		checkRabbitMQClient()

		// Only check API functionality if we successfully connected
		if healthData.API.Connected {
			checkOverview()
			checkCluster()
		}
	}

	// Render the health data
	fmt.Println(healthData.RenderAll())
}

func checkPort(port string, isAMQP bool) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", port), 5*time.Second)
	portOpen := err == nil

	if err != nil {
		common.AlarmCheckDown("rabbitmq_port_"+port, "Port "+port+" is not active", false, "", "")
		healthData.IsHealthy = false
	} else {
		_ = conn.Close()
		common.AlarmCheckUp("rabbitmq_port_"+port, "Port "+port+" is now active", false)
	}

	// Update the appropriate port status
	if isAMQP {
		healthData.Ports.AMQP = portOpen
	} else if port == "15672" {
		healthData.Ports.Management = portOpen
	} else {
		healthData.Ports.OtherPorts[port] = portOpen
	}
}

func checkEnabledPlugins() {
	filePath := "/etc/rabbitmq/enabled_plugins"
	searchString := "[rabbitmq_management]."

	content, err := os.ReadFile(filePath)
	if err != nil {
		common.LogError(fmt.Sprintf("Failed to read file %s: %v", filePath, err))
		healthData.Management.Enabled = false
		healthData.IsHealthy = false
		return
	}

	fileContent := string(content)
	found := strings.Contains(fileContent, searchString)
	healthData.Management.Enabled = found

	if found {
		message := fmt.Sprintf("Found '%s' in file %s", searchString, filePath)
		common.AlarmCheckUp("rabbitmq_management", message, false)
		// Check management port
		checkPort("15672", false)
		healthData.Management.Active = healthData.Ports.Management
	} else {
		message := fmt.Sprintf("Did not find '%s' in file %s", searchString, filePath)
		common.AlarmCheckDown("rabbitmq_management", message, false, "", "")
		healthData.IsHealthy = false
	}
}
