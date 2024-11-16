package shutdownNotifier

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/common"
)


func Main(cmd *cobra.Command, args []string) {
    common.ScriptName = "shutdownNotifier"
    common.Init()

    poweron, _ := cmd.Flags().GetBool("poweron")
    poweroff, _ := cmd.Flags().GetBool("poweroff")

    if poweron {
        common.Alarm("[ " + common.Config.Identifier + " ] [:info: Info] Server is up...", "", "", false)
    } else if poweroff {
        common.Alarm("[ " + common.Config.Identifier + " ] [:warning: Warning] Server is shutting down...", "", "", false)   } else {
        fmt.Println("No action specified")
    }

}
