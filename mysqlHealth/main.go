package mysqlHealth

import (
    "fmt"
    "time"
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/common"
    db "github.com/monobilisim/monokit/common/db"
)

var DbHealthConfig db.DbHealth

func Main(cmd *cobra.Command, args []string) {
    version := "3.0.0"
    common.ScriptName = "mysqlHealth"
    common.TmpDir = common.TmpDir + "mysqlHealth"
    common.Init()
    common.ConfInit("db", &DbHealthConfig)

    fmt.Println("MySQL Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))
   
    Connect()

    common.SplitSection("MySQL Access:")

    SelectNow()


    common.SplitSection("Number of Processes:")

    CheckProcessCount()

    if DbHealthConfig.Mysql.Cluster.Enabled == true {
        common.SplitSection("Cluster Status:")
        InaccessibleClusters()
        CheckClusterStatus()
        CheckNodeStatus()
        CheckClusterSynced()
    }
}
