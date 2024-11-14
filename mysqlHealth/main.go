//go:build linux

package mysqlHealth

import (
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	db "github.com/monobilisim/monokit/common/db"
	"github.com/spf13/cobra"
)

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

	fmt.Println("MySQL Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	Connect()
	defer Connection.Close()
	if err := Connection.Ping(); err != nil {
		fmt.Println("Can't ping MySQL")
		common.LogError("Can't ping MySQL connection. err: " + err.Error())
		common.AlarmCheckDown("ping", "Can't ping MySQL connection. err: "+err.Error(), false)
		return
	}
	common.AlarmCheckUp("ping", "MySQL ping returns no error.", false)

	common.SplitSection("MySQL Access:")

	SelectNow()

	common.SplitSection("Number of Processes:")

	CheckProcessCount()

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
