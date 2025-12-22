//go:build linux

package mysqlHealth

import (
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	db "github.com/monobilisim/monokit/common/db"
	"github.com/spf13/cobra"

	// NOTE: Removed viper import as it's not directly used here anymore
	"github.com/rs/zerolog/log"
)

func DetectMySQL() bool {
	// Auto-detection relies solely on finding valid client credentials
	// in standard my.cnf locations and successfully connecting/pinging.
	// The presence of /etc/mono/db.yaml is not required for detection,
	// only for the full execution of the health check.

	// First, check if the db config file exists. Detection shouldn't proceed if it doesn't.
	if !common.ConfExists("db") {
		log.Debug().Msg("mysqlHealth auto-detection skipped: db config file not found in /etc/mono.")
		return false
	}

	// Load minimal necessary config for detection only if the config file exists
	var detectConf db.DbHealth         // Use a local variable to avoid side effects on the global DbHealthConfig
	common.ConfInit("db", &detectConf) // Initialize config needed by ParseMyCnfAndConnect

	// ParseMyCnfAndConnect implicitly tests the connection.
	_, err := ParseMyCnfAndConnect("client")
	if err != nil {
		log.Debug().
			Str("component", "mysqlHealth").
			Str("function", "DetectMySQL").
			Err(err).
			Msg("mysqlHealth auto-detection failed: ParseMyCnfAndConnect error")
		// Close the connection if ParseMyCnfAndConnect partially succeeded before erroring
		if Connection != nil {
			Connection.Close()
		}
		return false
	}

	// If ParseMyCnfAndConnect succeeded, MySQL is considered detectable.
	// Close the connection opened during detection.
	if Connection != nil {
		Connection.Close()
	}
	log.Debug().Msg("mysqlHealth auto-detected successfully.")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "mysqlHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectMySQL,
	})
}

var DbHealthConfig db.DbHealth
var healthData *MySQLHealthData

func Main(cmd *cobra.Command, args []string) {
	version := "3.1.0"
	common.ScriptName = "mysqlHealth"
	common.TmpDir = common.TmpDir + "mysqlHealth"
	common.Init()
	common.ConfInit("db", &DbHealthConfig)

	// Initialize health data structure
	healthData = NewMySQLHealthData()
	healthData.Version = version

	// Backward compatibility: If check_table is not explicitly enabled in its own section,
	// but check_table_day is defined under cluster, treat it as enabled and use those values.
	if !DbHealthConfig.Mysql.Check_table.Enabled && DbHealthConfig.Mysql.Cluster.Check_table_day != "" {
		DbHealthConfig.Mysql.Check_table.Enabled = true
		DbHealthConfig.Mysql.Check_table.Check_table_day = DbHealthConfig.Mysql.Cluster.Check_table_day
		DbHealthConfig.Mysql.Check_table.Check_table_hour = DbHealthConfig.Mysql.Cluster.Check_table_hour
	}

	if DbHealthConfig.Mysql.Check_table.Enabled && (DbHealthConfig.Mysql.Check_table.Check_table_day == "" || DbHealthConfig.Mysql.Check_table.Check_table_hour == "") {
		DbHealthConfig.Mysql.Check_table.Check_table_day = "Sun"
		DbHealthConfig.Mysql.Check_table.Check_table_hour = "05:00"
	}

	// Set cluster enabled status in health data
	healthData.ClusterInfo.Enabled = DbHealthConfig.Mysql.Cluster.Enabled

	client.WrapperGetServiceStatus("mysqlHealth")

	// ParseMyCnfAndConnect now handles setting the global Connection on success.
	// We don't need the returned connection string here anymore.
	var finalErr error
	_, err := ParseMyCnfAndConnect("client")
	if err != nil {
		_, err = ParseMyCnfAndConnect("client-server")
		if err != nil {
			// If both attempts fail, log the error and set finalErr
			finalErr = err
		}
	}

	if finalErr != nil {
		// Log the specific error from ParseMyCnfAndConnect
		errMsg := fmt.Sprintf("Failed to establish initial MySQL connection: %s", err.Error())
		log.Error().Err(err).Msg("Failed to establish initial MySQL connection")
		// Use the specific error for the alarm
		common.AlarmCheckDown("ping", errMsg, false, "", "")
		// Update health data with connection failure
		healthData.ConnectionInfo.Connected = false
		healthData.ConnectionInfo.Error = errMsg
		// Exit or handle the failure appropriately - cannot proceed without a connection
		fmt.Println("Error: Could not connect to MySQL. Aborting health check.")
		// Render the health data even on failure
		fmt.Println(healthData.RenderAll())
		// Consider os.Exit(1) or returning an error if this function could return one
		return // Stop execution if connection fails initially
	}

	// If we reach here, ParseMyCnfAndConnect succeeded and set the global Connection.
	log.Debug().Msg("Initial MySQL connection and ping successful.")
	// The defer Connection.Close() should be placed *after* successful connection.
	defer Connection.Close()

	// Initial connection successful, clear any previous down alarm for ping.
	common.AlarmCheckUp("ping", "Established initial MySQL connection", false)

	// Update health data with successful connection
	healthData.ConnectionInfo.Connected = true

	// Perform checks but don't output directly to console
	// MySQL access check
	SelectNow()

	// Process count check
	CheckProcessCount()

	// Certification waiting processes check
	CheckCertificationWaiting()

	if DbHealthConfig.Mysql.Cluster.Enabled {
		// Check for inaccessible clusters
		InaccessibleClusters()

		// Check cluster overall status
		CheckClusterStatus()

		// Check individual node status
		CheckNodeStatus()

		// Check if cluster is synced
		CheckClusterSynced()
	}

	// check if time matches to configured time
	if DbHealthConfig.Mysql.Check_table.Enabled {
		if time.Now().Weekday().String() == DbHealthConfig.Mysql.Check_table.Check_table_day && time.Now().Format("15:04") == DbHealthConfig.Mysql.Check_table.Check_table_hour {
			CheckDB()
		}
	}

	// Check PMM status if configured
	checkPMM()

	// Render and display the health data only once at the end
	fmt.Println(healthData.RenderAll())
}
