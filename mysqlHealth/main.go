//go:build linux

package mysqlHealth

import (
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	db "github.com/monobilisim/monokit/common/db"
	"github.com/spf13/cobra"
	// NOTE: Removed viper import as it's not directly used here anymore
)

func DetectMySQL() bool {
	// Auto-detection relies solely on finding valid client credentials
	// in standard my.cnf locations and successfully connecting/pinging.
	// The presence of /etc/mono/db.yaml is not required for detection,
	// only for the full execution of the health check.

	// First, check if the db config file exists. Detection shouldn't proceed if it doesn't.
	if !common.ConfExists("db") {
		common.LogDebug("mysqlHealth auto-detection skipped: db config file not found in /etc/mono.")
		return false
	}

	// Load minimal necessary config for detection only if the config file exists
	var detectConf db.DbHealth         // Use a local variable to avoid side effects on the global DbHealthConfig
	common.ConfInit("db", &detectConf) // Initialize config needed by ParseMyCnfAndConnect

	// ParseMyCnfAndConnect implicitly tests the connection.
	_, err := ParseMyCnfAndConnect("client")
	if err != nil {
		common.LogDebug(fmt.Sprintf("mysqlHealth auto-detection failed: ParseMyCnfAndConnect error: %v", err))
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
	common.LogDebug("mysqlHealth auto-detected successfully.")
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

func Main(cmd *cobra.Command, args []string) {
	version := "3.1.0"
	common.ScriptName = "mysqlHealth"
	common.TmpDir = common.TmpDir + "mysqlHealth"
	common.Init()
	common.ConfInit("db", &DbHealthConfig)

	if DbHealthConfig.Mysql.Cluster.Enabled && (DbHealthConfig.Mysql.Cluster.Check_table_day == "" || DbHealthConfig.Mysql.Cluster.Check_table_hour == "") {
		DbHealthConfig.Mysql.Cluster.Check_table_day = "Sun"
		DbHealthConfig.Mysql.Cluster.Check_table_hour = "05:00"
	}

	api.WrapperGetServiceStatus("mysqlHealth")

	fmt.Println("MySQL Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	// ParseMyCnfAndConnect now handles setting the global Connection on success.
	// We don't need the returned connection string here anymore.
	_, err := ParseMyCnfAndConnect("client")

	if err != nil {
		// Log the specific error from ParseMyCnfAndConnect
		errMsg := fmt.Sprintf("Failed to establish initial MySQL connection: %s", err.Error())
		common.LogError(errMsg)
		// Use the specific error for the alarm
		common.AlarmCheckDown("ping", errMsg, false, "", "")
		// Exit or handle the failure appropriately - cannot proceed without a connection
		fmt.Println("Error: Could not connect to MySQL. Aborting health check.")
		// Consider os.Exit(1) or returning an error if this function could return one
		return // Stop execution if connection fails initially
	}

	// If we reach here, ParseMyCnfAndConnect succeeded and set the global Connection.
	common.LogDebug("Initial MySQL connection and ping successful.")
	// The defer Connection.Close() should be placed *after* successful connection.
	defer Connection.Close()

	// Initial connection successful, clear any previous down alarm for ping.
	common.AlarmCheckUp("ping", "Established initial MySQL connection", false)

	common.SplitSection("MySQL Access:")

	SelectNow()

	common.SplitSection("Number of Processes:")

	CheckProcessCount()

	common.SplitSection("Certification Waiting Processes:")

	CheckCertificationWaiting()

	if DbHealthConfig.Mysql.Cluster.Enabled {
		common.SplitSection("Cluster Status:")
		InaccessibleClusters()
		CheckClusterStatus()
		CheckNodeStatus()
		CheckClusterSynced()
	}

	// check if time matches to configured time
	if time.Now().Weekday().String() == DbHealthConfig.Mysql.Cluster.Check_table_day && time.Now().Format("15:04") == DbHealthConfig.Mysql.Cluster.Check_table_hour {
		CheckDB()
	}

	checkPMM()
}
