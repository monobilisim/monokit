package osHealth

import (
    "fmt"
    "time"
    "github.com/spf13/cobra"
    "github.com/monobilisim/mono-go/common"
)


type OsHealth struct {
     Filesystems []string 
     System_Load_And_Ram bool
     Part_use_limit float64

     Load struct {
         Limit_Multiplier float64
     }

     Ram_Limit float64

     Alarm struct {
         Enabled bool
     }
}

var OsHealthConfig OsHealth

func Main(cmd *cobra.Command, args []string) {
    version := "0.1.0"
    common.ScriptName = "osHealth"
    common.TmpDir = common.TmpDir + "osHealth"
    common.Init()
    common.ConfInit("os", &OsHealthConfig)

    fmt.Println("OS Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))
    
    DiskUsage()

    common.SplitSection("System Load and RAM")
    SysLoad()
    RamUsage()
}
