//go:build linux
package pgsqlHealth

import (
    "io"
	"os"
	"fmt"
	"time"
	"errors"
    "os/exec"
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

    // Check if user is postgres
    if os.Getenv("USER") != "postgres" {
        common.LogError("This script must be run as the postgres user")
        return
    }

	common.TmpDir = os.Getenv("HOME") + "/.local/share/mono/" + "pgsqlHealth"
	common.Init()
	common.ConfInit("db", &DbHealthConfig)
    

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
        common.AlarmCheckDown("pgsql_conn", "PostgreSQL connection failed: " + err.Error(), false, "", "")
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

    var role string

    role = "undefined"
    hour := time.Now().Format("15:04")

    lookPath, _ := exec.LookPath("wal-g")
    
    // Check if patroni is installed
    if _, err := os.Stat("/etc/patroni/patroni.yml"); !errors.Is(err, os.ErrNotExist) {
	    common.SplitSection("Cluster Status:")
	    clusterStatus()
        // curl -s patroniApiUrl | jq -r .role
        patroniRole, err := http.Get("http://" + patroniApiUrl + "/patroni")
        if err != nil {
            common.LogError(fmt.Sprintf("Error getting patroni role: %v\n", err))
            return
        }
        
        defer patroniRole.Body.Close()

        body, err := io.ReadAll(patroniRole.Body)
        if err != nil {
            common.LogError(fmt.Sprintf("Error reading patroni role body: %v\n", err))
            return
        }
        
        var patroniRoleJson map[string]interface{}
        err = json.Unmarshal(body, &patroniRoleJson)

        if err != nil {
            common.LogError(fmt.Sprintf("Error decoding patroni role json: %v\n", err))
            return
        }

        role = patroniRoleJson["role"].(string)
    }

    if (role == "master" || role == "undefined") && lookPath != "" && hour == DbHealthConfig.Postgres.Wal_g_verify_hour {
        walgVerify()
    }


    if common.DpkgPackageExists("pmm2-client") {
        common.SplitSection("PMM Status:")
        if common.SystemdUnitActive("pmm-agent.service") {
            common.PrettyPrintStr("PMM Agent", true, "running")
            common.AlarmCheckUp("pmm_agent", "PMM Agent is now running", false)
        } else {
            common.PrettyPrintStr("PMM Agent", false, "running")
            common.AlarmCheckDown("pmm_agent", "PMM Agent is not running", false, "", "")
        }
    }
}
