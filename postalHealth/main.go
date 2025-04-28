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
	api "github.com/monobilisim/monokit/common/api"
	mail "github.com/monobilisim/monokit/common/mail"
	issue "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DetectPostal checks if Postal seems to be installed and running.
// It first checks if the postal.service systemd unit is active,
// then checks for the config file and running postal containers via Docker.
func DetectPostal() bool {
	// 1. Check if postal.service exists
	if !common.SystemdUnitExists("postal.service") {
		common.LogDebug("postalHealth auto-detection failed: postal.service unit file not found.")
		return false

	}

	common.LogDebug("postalHealth auto-detection: postal.service exists.")

	// 2. Check for Postal config file
	viper.SetConfigName("postal")
	viper.AddConfigPath("/opt/postal/config")
	err := viper.ReadInConfig()
	if err != nil {
		common.LogDebug(fmt.Sprintf("postalHealth auto-detection failed: Cannot read /opt/postal/config/postal.yml: %v", err))
		return false
	}
	common.LogDebug("postalHealth auto-detection: Found /opt/postal/config/postal.yml.")

	// 2. Check Docker connection and look for postal containers
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		common.LogDebug(fmt.Sprintf("postalHealth auto-detection failed: Cannot connect to Docker API: %v", err))
		return false // Cannot detect without Docker access
	}
	defer apiClient.Close()
	common.LogDebug("postalHealth auto-detection: Connected to Docker API.")

	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		common.LogDebug(fmt.Sprintf("postalHealth auto-detection failed: Cannot list Docker containers: %v", err))
		return false // Cannot detect if listing fails
	}

	foundPostalContainer := false
	for _, container := range containers {
		for _, name := range container.Names {
			if strings.Contains(name, "postal") {
				common.LogDebug(fmt.Sprintf("postalHealth auto-detection: Found postal container: %s (State: %s)", name, container.State))
				foundPostalContainer = true
				break // Found one, no need to check others for detection purposes
			}
		}
		if foundPostalContainer {
			break
		}
	}

	if !foundPostalContainer {
		common.LogDebug("postalHealth auto-detection failed: No containers with 'postal' in the name found.")
		return false
	}

	common.LogDebug("postalHealth auto-detected successfully (config file found and at least one postal container exists).")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "postalHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectPostal, // Add the auto-detect function here
	})
}

var MailHealthConfig mail.MailHealth
var MainDB *sql.DB
var MessageDB *sql.DB

func Main(cmd *cobra.Command, args []string) {
	version := "3.1.0"
	common.ScriptName = "postalHealth"
	common.TmpDir = common.TmpDir + "postalHealth"
	common.Init()
	viper.SetDefault("postal.check_message", true)
	common.ConfInit("mail", &MailHealthConfig)

	api.WrapperGetServiceStatus("postalHealth")

	fmt.Println("Postal Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	common.SplitSection("Postal Status:")
	Services()

	common.SplitSection("Service Status:")
	RequestCheck()

	common.SplitSection("MySQL Status:")
	MainDB = MySQLConnect("main_db", "postal", true)
	defer MySQLDisconnect(MainDB)

	MessageDB = MySQLConnect("message_db", "postal", true)
	defer MySQLDisconnect(MessageDB)

	if MailHealthConfig.Postal.Check_Message {
		common.SplitSection("Message Queue:")
		GetMessageQueue()

		common.SplitSection("Held Messages:")
		GetHeldMessages()
	}
}

func RequestCheck() {
	// Check localhost:5000/login (health-web), localhost:9090/health (health-worker), localhost:9091/health (health-smtp)

	services := []string{"web::5000/login", "worker::9090/health", "smtp::9091/health"}
	for _, service := range services {
		split := strings.Split(service, "::")
		service := split[0]
		port := split[1]

		sendAlarm := false

		// Make a request to the service
		resp, err := http.Get("http://localhost:" + port)
		if err != nil {
			sendAlarm = true
		}

		if resp.StatusCode != 200 {
			sendAlarm = true
		}

		if !sendAlarm {
			common.PrettyPrintStr("Service "+service, true, "running")
			common.AlarmCheckUp("service_"+service, "Service health-"+service+" is running", false)
		} else {
			common.PrettyPrintStr("Service "+service, false, "running")
			common.AlarmCheckDown("service_"+service, "Service health-"+service+" is not running", false, "", "")
		}
	}
}

func Services() {
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	defer apiClient.Close()
	if common.SystemdUnitActive("postal.service") {
		if err != nil {
			common.LogError("Couldn't connect to Docker API: " + err.Error())
			common.AlarmCheckDown("docker", "Couldn't connect to Docker API: "+err.Error(), false, "", "")
			common.PrettyPrintStr("Docker API", false, "connected")
		}

		common.AlarmCheckUp("docker", "Docker API is up", false)
		common.AlarmCheckUp("postal", "Postal service is active", false)

		containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{All: true})
		if err != nil {
			common.LogError("Couldn't list containers: " + err.Error())
			common.AlarmCheckDown("docker", "Couldn't list containers: "+err.Error(), false, "", "")
			common.PrettyPrintStr("Docker containers", false, "listed")
		}

		postalServicesExist := false

		for _, container := range containers {
			for _, name := range container.Names {
				if strings.Contains(name, "postal") {
					// Remove / from the beginning of the name
					name = strings.TrimPrefix(name, "/")
					if container.State == "running" {
						common.AlarmCheckUp("docker_"+name, "Postal container "+name+" is running", false)
						postalServicesExist = true
						common.PrettyPrintStr("Postal container "+name, true, "running")
					} else {
						common.AlarmCheckDown("docker_"+name, "Postal container "+name+" is not running, state: "+container.State, false, "", "")
						postalServicesExist = true
						common.PrettyPrintStr("Postal container "+name, false, "running")
					}
				}
			}
		}

		if !postalServicesExist {
			common.AlarmCheckDown("postal_containers", "Couldn't find any running Postal containers", false, "", "")
			common.PrettyPrintStr("Postal service", false, "running")
		}
	} else {
		common.AlarmCheckDown("postal", "Postal service is not active", false, "", "")
		common.PrettyPrintStr("Postal service", false, "active")
	}
}

func MySQLConnect(dbName string, dbPath string, doPrint bool) *sql.DB {
	// Get info out of /opt/postal/config/postal.yml
	viper.SetConfigName("postal")
	viper.AddConfigPath("/opt/postal/config")
	err := viper.ReadInConfig()
	if err != nil {
		common.LogError("Couldn't read Postal config file: " + err.Error())
		common.AlarmCheckDown("mysql", "Couldn't read Postal config file: "+err.Error(), false, "", "")
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
		if doPrint {
			common.PrettyPrintStr("MySQL connection for "+dbName, false, "connected")
		}
		common.LogError("Couldn't connect to MySQL for " + dbName + ": " + err.Error())
		common.AlarmCheckDown("mysql_"+dbName, "Couldn't connect to MySQL for "+dbName+": "+err.Error(), false, "", "")
		issue.CheckDown("mysql_"+dbName, common.Config.Identifier+" sunucusunda "+dbName+" veritabanına bağlanılamadı", "Bağlantı hatası: "+err.Error(), false, 0)
	} else {
		if doPrint {
			common.PrettyPrintStr("MySQL connection for "+dbName, true, "connected")
		}
		common.AlarmCheckUp("mysql_"+dbName, "MySQL connection for "+dbName+" is up", false)
		issue.CheckUp("mysql_"+dbName, "Bağlantı başarılı bir şekilde kuruldu, kapatılıyor")
	}

	return db
}

func MySQLDisconnect(db *sql.DB) {
	db.Close()
}

func GetMessageQueue() {
	rows, err := MessageDB.Query("SELECT COUNT(*) FROM postal.queued_messages")
	if err != nil {
		common.LogError("Couldn't get message queue count: " + err.Error())
		common.AlarmCheckDown("mysql_queue", "Couldn't get message queue count from database message_db: "+err.Error(), false, "", "")
	}

	var count int
	for rows.Next() {
		rows.Scan(&count)
	}

	if count >= MailHealthConfig.Postal.Message_Threshold {
		fmt.Println(common.Blue + "Message queue count" + common.Reset + " is " + common.Fail + strconv.Itoa(count) + "/" + strconv.Itoa(MailHealthConfig.Postal.Message_Threshold) + common.Reset)
		common.AlarmCheckDown("mysql_queue_limit", "Message queue at or above limit: "+strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Postal.Message_Threshold), false, "", "")
	} else {
		common.PrettyPrintStr("Message queue count", true, fmt.Sprintf("%d", count))
		common.AlarmCheckUp("mysql_queue_limit", "Message queue below limit: "+strconv.Itoa(count)+"/"+strconv.Itoa(MailHealthConfig.Postal.Message_Threshold), false)
	}
}

func GetHeldMessages() {
	// select id, permalink from postal.servers
	rows, err := MessageDB.Query("SELECT id, permalink FROM postal.servers")
	if err != nil {
		common.LogError("Couldn't get held messages: " + err.Error())
		common.AlarmCheckDown("mysql_held", "Couldn't get held messages from database message_db: "+err.Error(), false, "", "")
	} else {
		common.AlarmCheckUp("mysql_held", "Can get Held messages count again", false)
	}

	for rows.Next() {
		var id int
		var name string

		rows.Scan(&id, &name)

		variable := "postal-server-" + strconv.Itoa(id)

		dbTemp := MySQLConnect("message_db", variable, false)

		dbMessageHeld, err := dbTemp.Query("SELECT COUNT(id) FROM messages WHERE status = 'Held'")
		if err != nil {
			common.LogError("Couldn't get held messages: " + err.Error())
			common.AlarmCheckDown("mysql_held", "Couldn't get held messages from database message_db: "+err.Error(), false, "", "")
		} else {
			common.AlarmCheckUp("mysql_held", "Can get Held messages count again", false)
		}

		var count int
		for dbMessageHeld.Next() {
			count++
		}

		if count < MailHealthConfig.Postal.Held_Threshold {
			common.PrettyPrintStr("Held messages for "+name+" ("+variable+")", true, fmt.Sprintf("%d", count))

			common.AlarmCheckUp("mysql_held_"+variable, "Held messages for "+name+" is below threshold", false)
		} else {
			common.PrettyPrintStr("Held messages for "+name+" ("+variable+")", true, fmt.Sprintf("%d", count))
			common.AlarmCheckDown("mysql_held_"+variable, "Held messages for "+name+" is above threshold", false, "", "")
		}

		MySQLDisconnect(dbTemp)
	}
}
