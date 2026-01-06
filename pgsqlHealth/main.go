//go:build linux

package pgsqlHealth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	db "github.com/monobilisim/monokit/common/db"
	versionCheck "github.com/monobilisim/monokit/common/versionCheck"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func DetectPostgres() bool {
	// Check if any common PostgreSQL service unit file exists
	postgresServiceExists := common.SystemdUnitExists("postgresql.service") ||
		common.SystemdUnitExists("postgresql@.service") || // Check for template unit
		common.SystemdUnitExists("postgresql@13-main.service") ||
		common.SystemdUnitExists("postgresql@14-main.service") ||
		common.SystemdUnitExists("postgresql@15-main.service")

	if !postgresServiceExists {
		log.Debug().Msg("PostgreSQL detection failed: No common postgresql systemd unit file found.")
		return false
	}

	// Check if psql binary exists
	_, err := exec.LookPath("psql")
	if err != nil {
		log.Debug().Msg("PostgreSQL detection failed: psql binary not found in PATH")
		return false
	}

	// If service unit exists and binary exists, assume PostgreSQL is present
	log.Debug().Msg("PostgreSQL detected: Service unit file exists and psql binary found.")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "pgsqlHealth",
		EntryPoint: Main,
		Platform:   "linux", // Specific to Linux due to user check and patroni path
		AutoDetect: DetectPostgres,
		RunAsUser:  "postgres", // Specify the user to run this component as
	})
}

var DbHealthConfig db.DbHealth
var patroniApiUrl string
var healthData *PostgreSQLHealthData

// getPostgresVersionInfo gets PostgreSQL version information
// and determines update status
func getPostgresVersionInfo() (string, bool, string) {
	// Get the version of PostgreSQL
	out, err := exec.Command("psql", "--version").Output()
	if err != nil {
		return "", false, "Error getting version"
	}

	// Parse the version
	// Example output: psql (PostgreSQL) 13.3 (Ubuntu 13.3-1.pgdg20.04+1)
	version := strings.Split(string(out), " ")[2]

	// Get the previously stored version
	oldVersion := versionCheck.GatherVersion("postgres")

	if oldVersion != "" && oldVersion == version {
		return version, false, "PostgreSQL has not been updated"
	}

	if oldVersion != "" && oldVersion != version {
		// Don't use fmt.Println for any output here
		return version, true, fmt.Sprintf("Updated from %s to %s", oldVersion, version)
	}

	// Store the current version for future checks
	versionCheck.StoreVersion("postgres", version)

	return version, false, "Up-to-date"
}

func Main(cmd *cobra.Command, args []string) {
	version := "4.0.0"
	common.ScriptName = "pgsqlHealth"

	// Check if user is postgres or iasdb
	if os.Getenv("USER") != "postgres" && os.Getenv("USER") != "iasdb" {
		log.Error().Msg("This script must be run as the postgres or iasdb user")
		return
	}

	common.TmpDir = "/tmp/" + "pgsqlHealth"
	common.Init()
	common.ConfInit("db", &DbHealthConfig)

	// Initialize health data structure
	healthData = NewPostgreSQLHealthData()
	healthData.Version = version

	// Get the Patroni API URL
	// connection.go
	if _, err := os.Stat("/etc/patroni/patroni.yml"); !errors.Is(err, os.ErrNotExist) {
		patroniApiUrl, err = getPatroniUrl()
		if err != nil {
			log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Main").Str("action", "get_patroni_url_failed").Msg("Error getting patroni url")
			// Update health data with error
			healthData.ClusterInfo.Enabled = true
			healthData.ClusterInfo.IsHealthy = false
			healthData.ClusterInfo.Status = fmt.Sprintf("Error: %v", err)

			// Render the health data even on failure
			fmt.Println(healthData.RenderAll())
			return
		}
		// Set cluster as enabled in health data
		healthData.ClusterInfo.Enabled = true
	}

	// Connect to PostgreSQL
	// connection.go
	err := Connect()
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Main").Str("action", "connect_to_postgresql_failed").Msg("Error connecting to PostgreSQL")
		common.AlarmCheckDown("pgsql_conn", "PostgreSQL connection failed: "+err.Error(), false, "", "")

		// Update health data with connection failure
		healthData.ConnectionInfo.Connected = false
		healthData.ConnectionInfo.Error = err.Error()

		// Render the health data even on failure
		fmt.Println(healthData.RenderAll())
		return
	} else {
		common.AlarmCheckUp("pgsql_conn", "PostgreSQL connection successfully restored", false)

		// Update health data with successful connection
		healthData.ConnectionInfo.Connected = true
	}

	defer Connection.Close()

	// uptime.go - gather uptime information
	uptimeData, _ := uptime()
	if uptimeData != nil {
		healthData.UptimeInfo.Uptime = uptimeData.Uptime
		healthData.UptimeInfo.StartTime = uptimeData.StartTime
		healthData.UptimeInfo.ActiveTime = uptimeData.ActiveTime
	}

	// Check active connections - monitoring.go
	connectionsData, _ := activeConnections(DbHealthConfig)
	if connectionsData != nil {
		healthData.ConnectionsInfo.Active = connectionsData.Active
		healthData.ConnectionsInfo.Limit = connectionsData.Limit
		healthData.ConnectionsInfo.UsageRate = connectionsData.UsageRate
		healthData.ConnectionsInfo.Exceeded = connectionsData.UsageRate > 0.8 // 80% threshold
	}

	// Version check - do it directly without external output
	// versionCheck.PostgresCheck() - removing this line to prevent console output

	// Get version info for our health data structure
	pgVersion, needsUpdate, updateMsg := getPostgresVersionInfo()
	healthData.VersionInfo.Version = pgVersion
	healthData.VersionInfo.NeedsUpdate = needsUpdate
	healthData.VersionInfo.UpdateMessage = updateMsg

	// Check running queries - monitoring.go
	queriesData, _ := runningQueries(DbHealthConfig)
	if queriesData != nil {
		// Store running queries data
		healthData.QueriesInfo.RunningQueries = make([]QueryInfo, len(queriesData))
		healthData.QueriesInfo.LongRunning = 0
		healthData.QueriesInfo.QueryLimit = DbHealthConfig.Postgres.Limits.Query

		for i, q := range queriesData {
			healthData.QueriesInfo.RunningQueries[i] = QueryInfo{
				PID:      q.PID,
				Username: q.Username,
				Database: q.Database,
				Duration: q.Duration,
				State:    q.State,
				Query:    q.Query,
			}

			// Count long running queries (over 5 minutes)
			// This is just an example threshold - adjust as needed
			if q.DurationSeconds > 300 {
				healthData.QueriesInfo.LongRunning++
			}
		}
	}

	if DbHealthConfig.Postgres.Wal_g_verify_hour != "" {
		DbHealthConfig.Postgres.Wal_g_verify_hour = "03:00"
	}

	var role string
	role = "undefined"
	hour := time.Now().Format("15:04")
	lookPath, _ := exec.LookPath("wal-g")

	// Check WAL-G status
	if lookPath != "" {
		healthData.WalGInfo.Enabled = true
	}

	// Check if patroni is installed
	if _, err := os.Stat("/etc/patroni/patroni.yml"); !errors.Is(err, os.ErrNotExist) {
		// Get cluster status
		clusterStatusData, _ := clusterStatus(patroniApiUrl, DbHealthConfig)
		if clusterStatusData != nil {
			// Update cluster info in health data
			healthData.ClusterInfo.Status = clusterStatusData.Status
			healthData.ClusterInfo.IsHealthy = clusterStatusData.IsHealthy
			healthData.ClusterInfo.IsReplicated = clusterStatusData.IsReplicated

			// Process nodes information
			if len(clusterStatusData.Nodes) > 0 {
				healthData.ClusterInfo.Nodes = make([]NodeInfo, len(clusterStatusData.Nodes))
				for i, node := range clusterStatusData.Nodes {
					healthData.ClusterInfo.Nodes[i] = NodeInfo{
						Name:    node.Name,
						Role:    node.Role,
						State:   node.State,
						Host:    node.Host,
						Port:    node.Port,
						Healthy: node.State == "running",
					}
				}
			}
		}

		// Get patroni role
		patroniClient := getPatroniHTTPClient()
		patroniRole, err := patroniClient.Get(strings.TrimSuffix(patroniApiUrl, "/") + "/patroni")
		if err != nil {
			log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Main").Str("action", "get_patroni_role_failed").Msg("Error getting patroni role")
		} else {
			defer patroniRole.Body.Close()

			body, err := io.ReadAll(patroniRole.Body)
			if err != nil {
				log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Main").Str("action", "read_patroni_role_body_failed").Msg("Error reading patroni role body")
			} else {
				var patroniRoleJson map[string]interface{}
				err = json.Unmarshal(body, &patroniRoleJson)

				if err != nil {
					log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Main").Str("action", "decode_patroni_role_json_failed").Msg("Error decoding patroni role json")
				} else {
					role = patroniRoleJson["role"].(string)
					healthData.ClusterInfo.Role = role
				}
			}
		}
	}

	// Check if the current role is master or undefined and if the hour is the same as the configured hour
	// wal-g.go
	if (role == "master" || role == "undefined") && lookPath != "" && hour == DbHealthConfig.Postgres.Wal_g_verify_hour {
		walGData, _ := WalgVerify()
		if walGData != nil {
			healthData.WalGInfo.Status = walGData.Status
			healthData.WalGInfo.LastBackup = walGData.LastBackup
			healthData.WalGInfo.BackupCount = walGData.BackupCount
			healthData.WalGInfo.Healthy = walGData.Healthy
		}
	}

	// Check if PMM is installed and if the service is running
	if common.DpkgPackageExists("pmm2-client") {
		healthData.PMM.Enabled = true

		if common.SystemdUnitActive("pmm-agent.service") {
			healthData.PMM.Status = "Running"
			healthData.PMM.Active = true
			common.AlarmCheckUp("pmm_agent", "PMM Agent is now running", false)
		} else {
			healthData.PMM.Status = "Not Running"
			healthData.PMM.Active = false
			common.AlarmCheckDown("pmm_agent", "PMM Agent is not running", false, "", "")
		}
	}

	// Render and display the health data
	fmt.Println(healthData.RenderAll())
}
