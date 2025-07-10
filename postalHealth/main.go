//go:build linux

package postalHealth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	_ "github.com/go-sql-driver/mysql"
	"github.com/monobilisim/monokit/common"
	apiclient "github.com/monobilisim/monokit/common/api/client"
	mail "github.com/monobilisim/monokit/common/mail"
	issue "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DetectPostal checks if Postal seems to be installed and running.
// It first checks if the postal.service systemd unit is active,
// then checks for the config file and running postal containers via Docker.
func DetectPostal() bool {
	// 1. Check if postal.service exists
	if !common.SystemdUnitExists("postal.service") {
		log.Debug().Msg("postalHealth auto-detection failed: postal.service unit file not found.")
		return false
	}

	log.Debug().Msg("postalHealth auto-detection: postal.service exists.")

	// 2. Check for Postal config file
	viper.SetConfigName("postal")
	viper.AddConfigPath("/opt/postal/config")
	err := viper.ReadInConfig()
	if err != nil {
		log.Debug().Err(err).Msg("postalHealth auto-detection failed: Cannot read /opt/postal/config/postal.yml")
		return false
	}
	log.Debug().Msg("postalHealth auto-detection: Found /opt/postal/config/postal.yml.")

	// 2. Check Docker connection and look for postal containers
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Debug().Err(err).Msg("postalHealth auto-detection failed: Cannot connect to Docker API")
		return false // Cannot detect without Docker access
	}
	defer apiClient.Close()
	log.Debug().Msg("postalHealth auto-detection: Connected to Docker API.")

	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		log.Debug().Err(err).Msg("postalHealth auto-detection failed: Cannot list Docker containers")
		return false // Cannot detect if listing fails
	}

	foundPostalContainer := false
	for _, container := range containers {
		for _, name := range container.Names {
			if strings.Contains(name, "postal") {
				log.Debug().
					Str("component", "postalHealth").
					Str("function", "DetectPostal").
					Str("container_name", name).
					Str("container_state", container.State).
					Msg("postalHealth auto-detection: Found postal container")
				foundPostalContainer = true
				break // Found one, no need to check others for detection purposes
			}
		}
		if foundPostalContainer {
			break
		}
	}

	if !foundPostalContainer {
		log.Debug().Msg("postalHealth auto-detection failed: No containers with 'postal' in the name found")
		return false
	}

	log.Debug().Msg("postalHealth auto-detected successfully (config file found and at least one postal container exists)")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "postalHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectPostal,
	})
}

var MailHealthConfig mail.MailHealth
var MainDB *sql.DB
var MessageDB *sql.DB

// CheckPostalHealth performs all Postal health checks and returns a data structure with the results
func CheckPostalHealth(skipOutput bool) *PostalHealthData {
	data := &PostalHealthData{
		IsHealthy:     true, // Start with assumption it's healthy
		Services:      make(map[string]bool),
		Containers:    make(map[string]ContainerStatus),
		MySQLStatus:   make(map[string]bool),
		ServiceStatus: make(map[string]bool),
	}

	// Check Postal services
	data.Services = CheckServices(skipOutput)

	// Check Docker containers
	data.Containers = CheckContainers(skipOutput)

	// Check MySQL connections
	MainDB = MySQLConnect("main_db", "postal", skipOutput)
	defer MySQLDisconnect(MainDB)
	data.MySQLStatus["main_db"] = MainDB != nil

	MessageDB = MySQLConnect("message_db", "postal", skipOutput)
	defer MySQLDisconnect(MessageDB)
	data.MySQLStatus["message_db"] = MessageDB != nil

	// Check service health
	data.ServiceStatus = CheckServiceHealth(skipOutput)

	// Check message queues if enabled
	if MailHealthConfig.Postal.Check_Message {
		data.MessageQueue = GetMessageQueue(skipOutput)
		data.HeldMessages = GetHeldMessages(skipOutput)
	}

	// Determine overall health
	for _, serviceStatus := range data.Services {
		if !serviceStatus {
			data.IsHealthy = false
			break
		}
	}

	for _, container := range data.Containers {
		if !container.IsRunning {
			data.IsHealthy = false
			break
		}
	}

	for _, mysqlStatus := range data.MySQLStatus {
		if !mysqlStatus {
			data.IsHealthy = false
			break
		}
	}

	for _, serviceStatus := range data.ServiceStatus {
		if !serviceStatus {
			data.IsHealthy = false
			break
		}
	}

	if data.MessageQueue.Limit > 0 && !data.MessageQueue.IsHealthy {
		data.IsHealthy = false
	}

	// Check if any server has unhealthy held messages
	for _, server := range data.HeldMessages {
		if !server.IsHealthy {
			data.IsHealthy = false
			break
		}
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
	version := "3.1.0"
	common.ScriptName = "postalHealth"
	common.TmpDir = common.TmpDir + "postalHealth"
	common.Init()
	viper.SetDefault("postal.check_message", true)
	common.ConfInit("mail", &MailHealthConfig)

	apiclient.WrapperGetServiceStatus("postalHealth")

	// Collect all health data with skipOutput=true since we'll use our UI rendering
	healthData := CheckPostalHealth(true)

	// Create a title for the box
	title := fmt.Sprintf("Postal Health Check v%s - %s", version, time.Now().Format("2006-01-02 15:04:05"))

	// Generate content using our UI renderer
	content := healthData.RenderCompact()

	// Display the rendered box
	renderedBox := common.DisplayBox(title, content)
	fmt.Println(renderedBox)
}

// CheckServices checks the status of Postal services and returns a map of service statuses
func CheckServices(skipOutput bool) map[string]bool {
	services := make(map[string]bool)
	isActive := common.SystemdUnitActive("postal.service")
	services["postal.service"] = isActive

	if isActive {
		common.AlarmCheckUp("postal", "Postal service is active", false)
	} else {
		common.AlarmCheckDown("postal", "Postal service is not active", false, "", "")
	}

	return services
}

// CheckContainers checks the status of Postal Docker containers
func CheckContainers(skipOutput bool) map[string]ContainerStatus {
	containers := make(map[string]ContainerStatus)
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		if !skipOutput {
			log.Error().Err(err).Msg("Couldn't connect to Docker API")
			common.AlarmCheckDown("docker", "Couldn't connect to Docker API: "+err.Error(), false, "", "")
		}
		return containers
	}
	defer apiClient.Close()

	common.AlarmCheckUp("docker", "Docker API is up", false)

	containerList, err := apiClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		if !skipOutput {
			log.Error().Err(err).Msg("Couldn't list containers")
			common.AlarmCheckDown("docker", "Couldn't list containers: "+err.Error(), false, "", "")
		}
		return containers
	}

	for _, container := range containerList {
		for _, name := range container.Names {
			if strings.Contains(name, "postal") {
				// Remove / from the beginning of the name
				name = strings.TrimPrefix(name, "/")
				isRunning := container.State == "running"
				containers[name] = ContainerStatus{
					Name:      name,
					IsRunning: isRunning,
					State:     container.State,
				}

				if isRunning {
					common.AlarmCheckUp("docker_"+name, "Postal container "+name+" is running", false)
				} else {
					common.AlarmCheckDown("docker_"+name, "Postal container "+name+" is not running, state: "+container.State, false, "", "")
				}
			}
		}
	}

	return containers
}

// CheckServiceHealth checks the health of Postal services
func CheckServiceHealth(skipOutput bool) map[string]bool {
	services := make(map[string]bool)
	serviceChecks := []string{"web::5000/login", "worker::9090/health", "smtp::9091/health"}

	for _, service := range serviceChecks {
		split := strings.Split(service, "::")
		serviceName := split[0]
		port := split[1]

		resp, err := http.Get("http://localhost:" + port)
		isHealthy := err == nil && resp.StatusCode == 200
		services[serviceName] = isHealthy

		if !skipOutput {
			if isHealthy {
				common.AlarmCheckUp("service_"+serviceName, "Service health-"+serviceName+" is running", false)
			} else {
				common.AlarmCheckDown("service_"+serviceName, "Service health-"+serviceName+" is not running", false, "", "")
			}
		}
	}

	return services
}

func MySQLConnect(dbName string, dbPath string, skipOutput bool) *sql.DB {
	viper.SetConfigName("postal")
	viper.AddConfigPath("/opt/postal/config")
	err := viper.ReadInConfig()
	if err != nil {
		log.Error().Err(err).Msg("Couldn't read Postal config file")
		common.AlarmCheckDown("mysql", "Couldn't read Postal config file: "+err.Error(), false, "", "")
		return nil
	}

	dbHost := viper.GetString(dbName + ".host")
	dbPort := viper.GetString(dbName + ".port")
	if dbPort == "" {
		dbPort = "3306"
	}
	dbUser := viper.GetString(dbName + ".username")
	dbPass := viper.GetString(dbName + ".password")

	db, err := sql.Open("mysql", dbUser+":"+dbPass+"@tcp("+dbHost+":"+dbPort+")/"+dbPath)
	if err != nil {
		log.Error().Err(err).Str("db_name", dbName).Msg("Couldn't connect to MySQL")
		common.AlarmCheckDown("mysql_"+dbName, "Couldn't connect to MySQL for "+dbName+": "+err.Error(), false, "", "")
		issue.CheckDown("mysql_"+dbName, common.Config.Identifier+" sunucusunda "+dbName+" veritabanına bağlanılamadı", "Bağlantı hatası: "+err.Error(), false, 0)
		return nil
	}

	common.AlarmCheckUp("mysql_"+dbName, "MySQL connection for "+dbName+" is up", false)
	issue.CheckUp("mysql_"+dbName, "Bağlantı başarılı bir şekilde kuruldu, kapatılıyor")

	return db
}

func MySQLDisconnect(db *sql.DB) {
	if db != nil {
		db.Close()
	}
}

// GetMessageQueue checks the message queue status
func GetMessageQueue(skipOutput bool) QueueStatus {
	status := QueueStatus{
		Limit: MailHealthConfig.Postal.Message_Threshold,
	}

	if MessageDB == nil {
		return status
	}

	rows, err := MessageDB.Query("SELECT COUNT(*) FROM postal.queued_messages")
	if err != nil {
		log.Error().Err(err).Msg("Couldn't get message queue count")
		common.AlarmCheckDown("mysql_queue", "Couldn't get message queue count from database message_db: "+err.Error(), false, "", "")
		return status
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		rows.Scan(&count)
	}

	status.Count = count
	status.IsHealthy = count < status.Limit

	if !skipOutput {
		if count >= MailHealthConfig.Postal.Message_Threshold {
			common.AlarmCheckDown("mysql_queue_limit", "Message queue at or above limit: "+strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Postal.Message_Threshold), false, "", "")
		} else {
			common.AlarmCheckUp("mysql_queue_limit", "Message queue below limit: "+strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Postal.Message_Threshold), false)
		}
	}

	return status
}

// GetHeldMessages checks the held messages status for each server
func GetHeldMessages(skipOutput bool) map[string]ServerHeldMessages {
	servers := make(map[string]ServerHeldMessages)

	if MessageDB == nil {
		return servers
	}

	// Get all servers
	rows, err := MessageDB.Query("SELECT id, permalink FROM postal.servers")
	if err != nil {
		log.Error().Err(err).Msg("Couldn't get held messages")
		common.AlarmCheckDown("mysql_held", "Couldn't get held messages from database message_db: "+err.Error(), false, "", "")
		return servers
	}
	defer rows.Close()

	common.AlarmCheckUp("mysql_held", "Can get Held messages count again", false)

	for rows.Next() {
		var id int
		var name string

		err := rows.Scan(&id, &name)
		if err != nil {
			log.Error().Err(err).Msg("Error scanning server row")
			continue
		}

		variable := "postal-server-" + strconv.Itoa(id)
		dbTemp := MySQLConnect("message_db", variable, false)
		if dbTemp == nil {
			continue
		}

		dbMessageHeld, err := dbTemp.Query("SELECT COUNT(id) FROM messages WHERE status = 'Held'")
		if err != nil {
			log.Error().Err(err).Msg("Couldn't get held messages")
			common.AlarmCheckDown("mysql_held", "Couldn't get held messages from database message_db: "+err.Error(), false, "", "")
			MySQLDisconnect(dbTemp)
			continue
		}
		common.AlarmCheckUp("mysql_held", "Can get Held messages count again", false)

		var count int
		if dbMessageHeld.Next() {
			err := dbMessageHeld.Scan(&count)
			if err != nil {
				log.Error().Err(err).Msg("Error scanning held messages count")
			}
		}
		dbMessageHeld.Close()

		servers[variable] = ServerHeldMessages{
			ServerName: name,
			ServerID:   id,
			Count:      count,
			IsHealthy:  count < MailHealthConfig.Postal.Held_Threshold,
		}

		if !skipOutput {
			if count < MailHealthConfig.Postal.Held_Threshold {
				common.AlarmCheckUp("mysql_held_"+variable, "Held messages for "+name+" is below threshold", false)
			} else {
				common.AlarmCheckDown("mysql_held_"+variable, "Held messages for "+name+" is above threshold", false, "", "")
			}
		}

		MySQLDisconnect(dbTemp)
	}

	return servers
}
