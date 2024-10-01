package common

import (
    "os"
    "fmt"
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/common"
)

var RedmineCmd = &cobra.Command{
    Use:   "redmine",
    Short: "Redmine-related utilities",
}

var RedmineIssueCmd = &cobra.Command{
    Use:   "issue",
    Short: "Issue-related utilities",
}

var RedmineCreateCmd = &cobra.Command{
    Use:   "create",
    Short: "Create a new issue in Redmine",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        service, _ := cmd.Flags().GetString("service")
        subject, _ := cmd.Flags().GetString("subject")
        message, _ := cmd.Flags().GetString("message")
        RedmineCreate(service, subject, message)
    },
}

var RedmineUpdateCmd = &cobra.Command{
    Use:   "update",
    Short: "Update an existing issue in Redmine",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        service, _ := cmd.Flags().GetString("service")
        message, _ := cmd.Flags().GetString("message")
        checkNote, _ := cmd.Flags().GetBool("checkNote")
        RedmineUpdate(service, message, checkNote)
    },
}

var RedmineCloseCmd = &cobra.Command{
    Use:   "close",
    Short: "Close an existing issue in Redmine",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        service, _ := cmd.Flags().GetString("service")
        message, _ := cmd.Flags().GetString("message")
        RedmineClose(service, message)
    },
}

var RedmineShowCmd = &cobra.Command{
    Use:   "show",
    Short: "Get the issue ID of the issue if it is opened",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        service, _ := cmd.Flags().GetString("service")
        fmt.Println(RedmineShow(service))
    },
}


var RedmineExistsCmd = &cobra.Command{
    Use: "exists",
    Short: "Check if an issue has already been created",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        subject, _ := cmd.Flags().GetString("subject")
        date, _ := cmd.Flags().GetString("date")
        search, _ := cmd.Flags().GetBool("search")
        
        exists := RedmineExists(subject, date, search)
        
        if exists != "" {
            fmt.Println(exists)
            os.Exit(0)
        } else {
            os.Exit(1)
        }
    },
}

var RedmineCheckUpCmd = &cobra.Command{
    Use: "up",
    Short: "Check if an issue exists and close it if it does",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        service, _ := cmd.Flags().GetString("service")
        message, _ := cmd.Flags().GetString("message")
        RedmineCheckUp(service, message)
    },
}

var RedmineCheckDownCmd = &cobra.Command{
    Use: "down",
    Short: "Check if an issue exists and create/update it if it does not",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        service, _ := cmd.Flags().GetString("service")
        subject, _ := cmd.Flags().GetString("subject")
        message, _ := cmd.Flags().GetString("message")
        RedmineCheckDown(service, subject, message)
    },
}
