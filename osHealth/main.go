package osHealth

import (
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/spf13/cobra"
)

// types.go
var OsHealthConfig OsHealth

func Main(cmd *cobra.Command, args []string) {
	version := "2.3.0"
	common.ScriptName = "osHealth"
	common.TmpDir = common.TmpDir + "osHealth"
	common.Init()
	common.ConfInit("os", &OsHealthConfig)

	api.WrapperGetServiceStatus("osHealth")

	if OsHealthConfig.Load.Issue_Multiplier == 0 {
		OsHealthConfig.Load.Issue_Multiplier = 1
	}

	if OsHealthConfig.Load.Issue_Interval == 0 {
		OsHealthConfig.Load.Issue_Interval = 15
	}

	fmt.Println("OS Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	DiskUsage()
	ZFSHealth() // Add ZFS health check

	common.SplitSection("System Load and RAM")
	SysLoad()
	RamUsage()

	// Check for systemd logs and push them to the API
	// This only runs on Linux systems with systemd
	SystemdLogs()
}
