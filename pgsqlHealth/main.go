package pgsqlHealth

import (
	"errors"
	"fmt"
	"github.com/monobilisim/monokit/common"
	db "github.com/monobilisim/monokit/common/db"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var DbHealthConfig db.DbHealth
var PATRONI_API_URL string

func Main(cmd *cobra.Command, args []string) {
	version := "3.0.0"
	common.ScriptName = "pgsqlHealth"
	common.TmpDir = common.TmpDir + "pgsqlHealth"
	common.Init()
	common.ConfInit("db", &DbHealthConfig)
	//var isCluster bool

	if _, err := os.Stat("/etc/patroni/patroni.yml"); !errors.Is(err, os.ErrNotExist) {
		//isCluster = true
		PATRONI_API_URL, err = getPatroniUrl()
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
		return
	}
	defer Connection.Close()
	common.PrettyPrintStr("PostgreSQL", true, "accessible")
	uptime()

	common.SplitSection("Active Connections:")
	activeConnections()

	common.SplitSection("Running Queries:")
	runningQueries()

	common.SplitSection("Cluster Status:")
	clusterStatus()
}
