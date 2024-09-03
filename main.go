package main

import (
    "github.com/monobilisim/monokit/common"
    "github.com/monobilisim/monokit/osHealth"
    "github.com/spf13/cobra"
    "fmt"
    "os"
)

func main() {

    var RootCmd = &cobra.Command{Use: "monokit"}

    var osHealthCmd = &cobra.Command{
        Use:   "osHealth",
        Short: "OS Health",
        Run: osHealth.Main,
    }
    
    //// Common
    RootCmd.AddCommand(common.RedmineCmd)
    RootCmd.AddCommand(common.AlarmCmd)
   
    /// Alarm
    
    // AlarmSend
    common.AlarmCmd.AddCommand(common.AlarmSendCmd)

    common.AlarmSendCmd.Flags().StringP("service", "s", "", "Service Name")
    common.AlarmSendCmd.Flags().StringP("message", "m", "", "Message")

    // AlarmCheckUp
    common.AlarmCmd.AddCommand(common.AlarmCheckUpCmd)

    common.AlarmCheckUpCmd.Flags().StringP("service", "s", "", "Service Name")
    common.AlarmCheckUpCmd.Flags().StringP("message", "m", "", "Message")

    // AlarmCheckDown
    common.AlarmCmd.AddCommand(common.AlarmCheckDownCmd)

    common.AlarmCheckDownCmd.Flags().StringP("service", "s", "", "Service Name")
    common.AlarmCheckDownCmd.Flags().StringP("message", "m", "", "Message")


    /// Redmine
    common.RedmineCmd.AddCommand(common.RedmineIssueCmd)
    
    // RedmineCreate
    common.RedmineIssueCmd.AddCommand(common.RedmineCreateCmd)
    
    common.RedmineCreateCmd.Flags().StringP("subject", "j", "", "Subject")
    common.RedmineCreateCmd.Flags().StringP("service", "s", "", "Service Name")
    common.RedmineCreateCmd.Flags().StringP("message", "m", "", "Message")
    common.RedmineCreateCmd.MarkFlagRequired("subject")
    common.RedmineCreateCmd.MarkFlagRequired("service")
    common.RedmineCreateCmd.MarkFlagRequired("message")

    // RedmineUpdate
    common.RedmineIssueCmd.AddCommand(common.RedmineUpdateCmd)
    
    common.RedmineUpdateCmd.Flags().StringP("service", "s", "", "Service Name")
    common.RedmineUpdateCmd.Flags().StringP("message", "m", "", "Message")
    common.RedmineUpdateCmd.MarkFlagRequired("service")
    common.RedmineUpdateCmd.MarkFlagRequired("message")
    
    // RedmineClose
    common.RedmineIssueCmd.AddCommand(common.RedmineCloseCmd)
    
    common.RedmineCloseCmd.Flags().StringP("service", "s", "", "Service Name")
    common.RedmineCloseCmd.Flags().StringP("message", "m", "", "Message")
    common.RedmineCloseCmd.MarkFlagRequired("service")
    common.RedmineCloseCmd.MarkFlagRequired("message")

    // RedmineShow
    common.RedmineIssueCmd.AddCommand(common.RedmineShowCmd)
    
    common.RedmineShowCmd.Flags().StringP("service", "s", "", "Service Name")
    common.RedmineShowCmd.MarkFlagRequired("service")

    /// OS Health
    RootCmd.AddCommand(osHealthCmd)
    
    if err := RootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
