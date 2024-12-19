//go:build linux
package pgsqlHealth

import (
	"os"
	"fmt"
	"time"
	"errors"
    "net/http"
    "encoding/json"
	"github.com/spf13/cobra"
	"github.com/monobilisim/monokit/common"
	db "github.com/monobilisim/monokit/common/db"
)

var DbHealthConfig db.DbHealth
var patroniApiUrl string

func Main(cmd *cobra.Command, args []string) {
	version := "3.0.0"
	common.ScriptName = "pgsqlHealth"
	common.TmpDir = common.TmpDir + "pgsqlHealth"
	common.Init()
	common.ConfInit("db", &DbHealthConfig)
    
    // Check if user is postgres
    if os.Getenv("USER") != "postgres" {
        common.LogError("This script must be run as the postgres user")
        return
    }

	//var isCluster bool

	if _, err := os.Stat("/etc/patroni/patroni.yml"); !errors.Is(err, os.ErrNotExist) {
		//isCluster = true
		patroniApiUrl, err = getPatroniUrl()
		if err != nil {
			common.LogError(fmt.Sprintf("Error getting patroni url: %v\n", err))
			return
		}
	}

	fmt.Println("PostgreSQL Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	common.SplitSection("PostgreSQL Access:")

	err := Connect()
	if err != nil {
		common.LogError(fmt.Sprintf("Error connecting to PostgreSQL: %v\n", err))
		common.PrettyPrintStr("PostgreSQL", false, "accessible")
        common.AlarmCheckDown("pgsql_conn", "PostgreSQL connection failed: " + err.Error(), false)
		return
	} else {
	    common.PrettyPrintStr("PostgreSQL", true, "accessible")
        common.AlarmCheckUp("pgsql_conn", "PostgreSQL connection successfully restored", false)
    }

	defer Connection.Close()
	uptime()

	common.SplitSection("Active Connections:")
	activeConnections()

	common.SplitSection("Running Queries:")
	runningQueries()

    if DbHealthConfig.Postgres.Wal_g_verify_hour != "" {
        DbHealthConfig.Postgres.Wal_g_verify_hour = "03:00"
    }

    //var role string

    //role = "undefined"
    
    // Check if patroni is installed
    if _, err := os.Stat("/etc/patroni/patroni.yml"); !errors.Is(err, os.ErrNotExist) {
	    common.SplitSection("Cluster Status:")
	    clusterStatus()
        // curl -s patroniApiUrl | jq -r .role
        patroniRole, _ := http.Get(patroniApiUrl + "/patroni")
        fmt.Println(patroniRole)

        patroniRoleJson := json.NewDecoder(patroniRole.Body)
        patroniRoleJson.Decode(&patroniRole)
        fmt.Println(patroniRole)

        //role = patroniRole["role"]

        //hour := time.Now().Format("15:04")

        // Check if the command wal-g exists
    }

    //if (role == "master" || role == "undefined") && exec.LookPath("wal-g") != "" && hour == DbHealthConfig.Postgres.Wal_g_verify_hour {
        //walgVerify()
    //}


    if common.DpkgPackageExists("pmm2-client") {
        common.SplitSection("PMM Status:")
        if common.SystemdUnitActive("pmm-agent.service") {
            common.PrettyPrintStr("PMM Agent", true, "running")
            common.AlarmCheckUp("pmm_agent", "PMM Agent is now running", false)
        } else {
            common.PrettyPrintStr("PMM Agent", false, "running")
            common.AlarmCheckDown("pmm_agent", "PMM Agent is not running", false)
        }
    }
}
