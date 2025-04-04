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

// Services checks if postal.service is active and verifies Docker container status
func RequestCheck() {
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

// RequestCheck verifies the health of Postal services by making HTTP requests to their endpoints
// It checks the following services:
// - Web interface (port 5000)
// - Worker service (port 9090)
// - SMTP service (port 9091)
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

// MySQLConnect establishes a connection to a MySQL database using configuration from /opt/postal/config/postal.yml
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

// GetMessageQueue checks the number of messages in the Postal message queue and compares it with the configured threshold.
// It queries the postal.queued_messages table in the message database and raises an alarm if the count exceeds the threshold.
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

// GetHeldMessages checks the number of held messages for each Postal server and compares them with the configured threshold.
// It queries the postal.servers table to get server IDs and names, then checks the messages table for each server
// to count messages with 'Held' status. Raises alarms if any server's held message count exceeds the threshold.
func GetHeldMessages() {
	// Select ID permalink from postal.servers
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
