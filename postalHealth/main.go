//go:build linux

package postalHealth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
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
	// Require mail.yaml so deleting the config disables the check
	if !common.ConfExists("mail") {
		log.Debug().Msg("postalHealth auto-detection failed: mail.yaml config not found.")
		return false
	}

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

	containers, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		log.Debug().Err(err).Msg("postalHealth auto-detection failed: Cannot list Docker containers")
		return false // Cannot detect if listing fails
	}

	foundPostalContainer := false
	for _, c := range containers.Items {
		for _, name := range c.Names {
			if strings.Contains(name, "postal") {
				log.Debug().
					Str("component", "postalHealth").
					Str("function", "DetectPostal").
					Str("container_name", name).
					Str("container_state", string(c.State)).
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

// queueBacklogMinutes is how long a ready, unlocked queued message may wait before it counts as backlog.
// Postal workers poll every 5 seconds, so a message waiting minutes means workers are not picking up work.
const queueBacklogMinutes = 5

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
		// queued_messages, servers and per-server message DBs are shared by every Postal node, so only
		// the node holding Postal's "tasks" worker role alarms on them; otherwise each node alarms once.
		runDBChecks, holder := shouldRunDBChecks(data.Containers)
		if runDBChecks {
			data.MessageQueue = GetMessageQueue()
			data.HeldMessages = GetHeldMessages()
			data.StaleLocks = GetStaleLocks()
		} else {
			data.DBChecksNode = holder
			releaseDBAlarms(holder)
		}
	}

	// Check SSL certificate expiry (port 25 STARTTLS)
	if MailHealthConfig.Postal.Ssl.Enabled {
		data.SSLCert = CheckSSL(skipOutput)
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

	if !data.MessageQueue.IsHealthy {
		data.IsHealthy = false
	}

	// Check if any server has unhealthy held messages
	for _, server := range data.HeldMessages {
		if !server.IsHealthy {
			data.IsHealthy = false
			break
		}
	}

	if data.StaleLocks.CheckStatus && !data.StaleLocks.IsHealthy {
		data.IsHealthy = false
	}

	if data.SSLCert.CheckStatus && data.SSLCert.ExpiringSoon {
		data.IsHealthy = false
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
	viper.SetDefault("postal.ssl.enabled", true)
	viper.SetDefault("postal.ssl.host", "localhost")
	viper.SetDefault("postal.ssl.port", 25)
	viper.SetDefault("postal.ssl.expiry_threshold_days", 10)
	viper.SetDefault("postal.stale_lock_minutes", 15)
	viper.SetDefault("postal.backlog_threshold", 10)
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

	containerList, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		if !skipOutput {
			log.Error().Err(err).Msg("Couldn't list containers")
			common.AlarmCheckDown("docker", "Couldn't list containers: "+err.Error(), false, "", "")
		}
		return containers
	}

	for _, c := range containerList.Items {
		for _, name := range c.Names {
			if strings.Contains(name, "postal") {
				// Remove / from the beginning of the name
				name = strings.TrimPrefix(name, "/")
				state := string(c.State)
				isRunning := c.State == container.StateRunning
				containers[name] = ContainerStatus{
					Name:        name,
					IsRunning:   isRunning,
					State:       state,
					NetworkMode: c.HostConfig.NetworkMode,
				}

				if isRunning {
					common.AlarmCheckUp("docker_"+name, "Postal container "+name+" is running", false)
				} else {
					common.AlarmCheckDown("docker_"+name, "Postal container "+name+" is not running, state: "+state, false, "", "")
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

// GetMessageQueue checks the queue and alarms on two thresholds:
//   - message_threshold: total queued messages, including ones waiting for a soft-fail retry (alarm key mysql_queue_limit)
//   - backlog_threshold: unlocked messages whose (retry) time passed more than queueBacklogMinutes ago without a
//     worker picking them up, i.e. workers are not keeping up (alarm key mysql_queue_backlog)
//
// A threshold <= 0 means unconfigured: that alarm is not raised.
func GetMessageQueue() QueueStatus {
	status := QueueStatus{
		Limit:          MailHealthConfig.Postal.Message_Threshold,
		BacklogLimit:   MailHealthConfig.Postal.Backlog_Threshold,
		BacklogMinutes: queueBacklogMinutes,
		IsHealthy:      true,
	}

	if MainDB == nil {
		return status
	}

	err := MainDB.QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(locked_by IS NULL AND COALESCE(retry_after, created_at) < UTC_TIMESTAMP() - INTERVAL ? MINUTE), 0) FROM queued_messages",
		queueBacklogMinutes,
	).Scan(&status.Count, &status.Backlog)
	if err != nil {
		log.Error().Err(err).Msg("Couldn't get message queue count")
		common.AlarmCheckDown("mysql_queue", "Couldn't get message queue count from main_db: "+err.Error(), false, "", "")
		return status
	}
	common.AlarmCheckUp("mysql_queue", "Can get message queue count again", false)

	if status.Limit > 0 {
		healthy := status.Count < status.Limit
		status.IsHealthy = status.IsHealthy && healthy
		msg := fmt.Sprintf("Total queued messages: %d/%d", status.Count, status.Limit)
		if healthy {
			common.AlarmCheckUp("mysql_queue_limit", msg, false)
		} else {
			common.AlarmCheckDown("mysql_queue_limit", msg, false, "", "")
		}
	}

	if status.BacklogLimit > 0 {
		healthy := status.Backlog < status.BacklogLimit
		status.IsHealthy = status.IsHealthy && healthy
		msg := fmt.Sprintf("Queued messages not picked up by a worker for %d+ minutes: %d/%d", queueBacklogMinutes, status.Backlog, status.BacklogLimit)
		if healthy {
			common.AlarmCheckUp("mysql_queue_backlog", msg, false)
		} else {
			common.AlarmCheckDown("mysql_queue_backlog", msg, false, "", "")
		}
	}

	return status
}

// GetStaleLocks counts queued messages whose worker lock is older than stale_lock_minutes.
// Postal does not retry such messages and TidyQueuedMessagesTask later deletes them, leaving the
// message Pending forever, so any non-zero count is alarmed. Postal stores timestamps in UTC.
func GetStaleLocks() StaleLockStatus {
	status := StaleLockStatus{ThresholdMinutes: MailHealthConfig.Postal.Stale_Lock_Minutes}
	if status.ThresholdMinutes <= 0 {
		status.ThresholdMinutes = 15
	}

	if MainDB == nil {
		return status
	}

	err := MainDB.QueryRow(
		"SELECT COUNT(*) FROM queued_messages WHERE locked_by IS NOT NULL AND locked_at < UTC_TIMESTAMP() - INTERVAL ? MINUTE",
		status.ThresholdMinutes,
	).Scan(&status.Count)
	if err != nil {
		log.Error().Err(err).Msg("Couldn't get stale queued message lock count")
		common.AlarmCheckDown("mysql_stale_lock_query", "Couldn't get stale queued message lock count from main_db: "+err.Error(), false, "", "")
		return status
	}
	common.AlarmCheckUp("mysql_stale_lock_query", "Can get stale queued message lock count again", false)

	status.CheckStatus = true
	status.IsHealthy = status.Count == 0

	if status.IsHealthy {
		common.AlarmCheckUp("postal_stale_lock", fmt.Sprintf("No queued messages locked longer than %d minutes", status.ThresholdMinutes), false)
	} else {
		common.AlarmCheckDown("postal_stale_lock", fmt.Sprintf("%d queued messages locked longer than %d minutes; Postal will not retry them", status.Count, status.ThresholdMinutes), false, "", "")
	}

	return status
}

// Held reason categories, derived from the details Postal writes on the message's latest Held delivery.
const (
	heldSuppression = "suppression"
	heldSendLimit   = "send_limit"
	heldSuspended   = "suspended"
	heldCredential  = "credential"
	heldDevelopment = "development"
	heldQuarantine  = "quarantine"
	heldRouteHold   = "route_hold"
	heldOther       = "other"
)

// heldReasonOrder is the display order of held reason categories.
var heldReasonOrder = []string{heldSendLimit, heldSuspended, heldCredential, heldDevelopment, heldOther, heldSuppression, heldQuarantine, heldRouteHold}

// heldReasonSQL maps a Held delivery's details to a category (see the held* constants).
const heldReasonSQL = `CASE
	WHEN details LIKE 'Recipient % is on the suppression list%' THEN 'suppression'
	WHEN details LIKE 'Message held because send limit%' THEN 'send_limit'
	WHEN details LIKE 'Mail server has been suspended%' THEN 'suspended'
	WHEN details LIKE 'Credential is configured to hold%' THEN 'credential'
	WHEN details LIKE 'Server is in development mode%' THEN 'development'
	WHEN details LIKE 'Message placed into quarantine%' THEN 'quarantine'
	WHEN details LIKE 'Message has been accepted but not sent%' THEN 'route_hold'
	ELSE 'other' END`

// evaluateHeld decides whether a server's held messages need an alarm and returns the alarmed count.
// send_limit and suspended mean outgoing mail has stopped, so any of them alarms. credential, development and
// unknown reasons alarm from held_threshold on (threshold <= 0 disables only this part). suppression, quarantine
// and route_hold are expected behaviour and never alarm.
func evaluateHeld(reasons map[string]int, threshold int) (healthy bool, alarmed int) {
	immediate := reasons[heldSendLimit] + reasons[heldSuspended]
	thresholded := reasons[heldCredential] + reasons[heldDevelopment] + reasons[heldOther]
	alarmed = immediate + thresholded
	healthy = immediate == 0 && (threshold <= 0 || thresholded < threshold)
	return healthy, alarmed
}

// formatHeldReasons renders non-zero categories as "send_limit 3, suppression 120".
func formatHeldReasons(reasons map[string]int) string {
	var parts []string
	for _, r := range heldReasonOrder {
		if reasons[r] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", r, reasons[r]))
		}
	}
	return strings.Join(parts, ", ")
}

// GetHeldMessages counts held messages per server by reason and alarms per evaluateHeld.
func GetHeldMessages() map[string]ServerHeldMessages {
	servers := make(map[string]ServerHeldMessages)

	if MainDB == nil || MessageDB == nil {
		return servers
	}

	serverList, err := listServers()
	if err != nil {
		log.Error().Err(err).Msg("Couldn't get held messages")
		common.AlarmCheckDown("mysql_held", "Couldn't list Postal servers from main_db: "+err.Error(), false, "", "")
		return servers
	}

	threshold := MailHealthConfig.Postal.Held_Threshold
	queryFailed := false
	for _, srv := range serverList {
		variable := heldDBName(srv.ID)
		reasons, err := heldReasons(variable)
		if err != nil {
			log.Error().Err(err).Str("database", variable).Msg("Couldn't get held messages")
			queryFailed = true
			continue
		}

		healthy, alarmed := evaluateHeld(reasons, threshold)
		servers[variable] = ServerHeldMessages{
			ServerName: srv.Name,
			ServerID:   srv.ID,
			Count:      alarmed,
			Reasons:    reasons,
			IsHealthy:  healthy,
		}

		msg := fmt.Sprintf("Held messages for %s: %s", srv.Name, formatHeldReasons(reasons))
		if healthy {
			common.AlarmCheckUp("mysql_held_"+variable, msg, false)
		} else {
			common.AlarmCheckDown("mysql_held_"+variable, msg, false, "", "")
		}
	}

	if queryFailed {
		common.AlarmCheckDown("mysql_held", "Couldn't get held messages for some servers from message_db, see monokit log", false, "", "")
	} else {
		common.AlarmCheckUp("mysql_held", "Can get Held messages count again", false)
	}

	return servers
}

// heldReasons counts a server's held messages by the category of their latest Held delivery.
func heldReasons(database string) (map[string]int, error) {
	rows, err := MessageDB.Query(
		"SELECT " + heldReasonSQL + " AS reason, COUNT(*) FROM (" +
			"SELECT (SELECT d.details FROM `" + database + "`.deliveries d WHERE d.message_id = m.id AND d.status = 'Held' ORDER BY d.id DESC LIMIT 1) AS details " +
			"FROM `" + database + "`.messages m WHERE m.status = 'Held') t GROUP BY reason",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reasons := make(map[string]int)
	for rows.Next() {
		var reason string
		var count int
		if err := rows.Scan(&reason, &count); err != nil {
			return nil, err
		}
		reasons[reason] = count
	}
	return reasons, rows.Err()
}

type postalServer struct {
	ID   int
	Name string
}

func listServers() ([]postalServer, error) {
	rows, err := MainDB.Query("SELECT id, permalink FROM servers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []postalServer
	for rows.Next() {
		var s postalServer
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func heldDBName(serverID int) string {
	return "postal-server-" + strconv.Itoa(serverID)
}

// shouldRunDBChecks reports whether this node runs the shared DB checks, and the "tasks" role holder host when not.
// Postal workers name the role holder "host:<hostname> pid:.. thread:..". That hostname equals this node's hostname
// only when worker containers use host networking; otherwise the role holder can't be mapped to a node and every
// node keeps running the checks. A missing or stale (>5 min, Postal's takeover time) role also keeps them running.
func shouldRunDBChecks(containers map[string]ContainerStatus) (bool, string) {
	if MainDB == nil {
		return true, ""
	}

	hostNetwork := false
	for _, c := range containers {
		if strings.Contains(c.Name, "worker") && c.NetworkMode == "host" {
			hostNetwork = true
			break
		}
	}
	if !hostNetwork {
		return true, ""
	}

	hostname, err := os.Hostname()
	if err != nil {
		return true, ""
	}

	var worker string
	var stale sql.NullBool
	err = MainDB.QueryRow(
		"SELECT worker, acquired_at < UTC_TIMESTAMP() - INTERVAL 5 MINUTE FROM worker_roles WHERE role = 'tasks'",
	).Scan(&worker, &stale)
	if err != nil || !stale.Valid || stale.Bool {
		return true, ""
	}

	fields := strings.Fields(worker)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "host:") {
		return true, ""
	}
	holder := strings.TrimPrefix(fields[0], "host:")
	if strings.EqualFold(holder, hostname) {
		return true, ""
	}
	return false, holder
}

// releaseDBAlarms recovers this node's shared DB alarms after the checks moved to another node, so alarms raised
// while this node held the role don't stay open forever. AlarmCheckUp only sends a recovery for alarms that were
// actually delivered and silently drops pending ones; it is a no-op for keys without alarm state.
func releaseDBAlarms(holder string) {
	msg := "Postal DB checks moved to " + holder
	for _, key := range []string{"mysql_queue", "mysql_queue_limit", "mysql_stale_lock_query", "postal_stale_lock", "mysql_held"} {
		common.AlarmCheckUp(key, msg, false)
	}
	serverList, err := listServers()
	if err != nil {
		return
	}
	for _, srv := range serverList {
		common.AlarmCheckUp("mysql_held_"+heldDBName(srv.ID), msg, false)
	}
}
