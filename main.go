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
    
    /// Redmine
    common.RedmineCmd.AddCommand(common.RedmineCreateCmd)
    
    // RedmineCreate
    common.RedmineCreateCmd.Flags().StringP("subject", "j", "", "Subject")
    common.RedmineCreateCmd.Flags().StringP("service", "s", "", "Service Name")
    common.RedmineCreateCmd.Flags().StringP("message", "m", "", "Message")
    common.RedmineCreateCmd.MarkFlagRequired("subject")
    common.RedmineCreateCmd.MarkFlagRequired("service")
    common.RedmineCreateCmd.MarkFlagRequired("message")

    // RedmineUpdate
    common.RedmineCmd.AddCommand(common.RedmineUpdateCmd)
    common.RedmineUpdateCmd.Flags().StringP("service", "s", "", "Service Name")
    common.RedmineUpdateCmd.Flags().StringP("message", "m", "", "Message")
    common.RedmineUpdateCmd.MarkFlagRequired("service")
    common.RedmineUpdateCmd.MarkFlagRequired("message")
    
    // RedmineClose
    common.RedmineCmd.AddCommand(common.RedmineCloseCmd)
    common.RedmineCloseCmd.Flags().StringP("service", "s", "", "Service Name")
    common.RedmineCloseCmd.Flags().StringP("message", "m", "", "Message")
    common.RedmineCloseCmd.MarkFlagRequired("service")
    common.RedmineCloseCmd.MarkFlagRequired("message")

    // RedmineShow
    common.RedmineCmd.AddCommand(common.RedmineShowCmd)
    common.RedmineShowCmd.Flags().StringP("service", "s", "", "Service Name")
    common.RedmineShowCmd.MarkFlagRequired("service")

    /// OS Health
    RootCmd.AddCommand(osHealthCmd)
    
    if err := RootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
