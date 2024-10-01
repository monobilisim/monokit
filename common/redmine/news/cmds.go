package common

import (
    "os"
    "fmt"
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/common"
)

var NewsCmd = &cobra.Command{
    Use:   "news",
    Short: "News-related utilities",
}

var CreateCmd = &cobra.Command{
    Use:   "create",
    Short: "Create news in Redmine",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        title, _ := cmd.Flags().GetString("title")
        description, _ := cmd.Flags().GetString("description")
        noDuplicate, _ := cmd.Flags().GetBool("noDuplicate")
        issueId := Create(title, description, noDuplicate)

        if issueId != "" {
            fmt.Println(issueId)
            os.Exit(0)
        } else {
            os.Exit(1)
        }
    },
}

var DeleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete news in Redmine",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        id, _ := cmd.Flags().GetString("id")
        Delete(id)
    },
}

var ExistsCmd = &cobra.Command{
    Use: "exists",
    Short: "Check if news have already been created",
    Run: func(cmd *cobra.Command, args []string) {
        common.Init()
        
        title, _ := cmd.Flags().GetString("title")
        description, _ := cmd.Flags().GetString("description")

        exists := Exists(title, description)
        
        if exists != "" {
            fmt.Println(exists)
            os.Exit(0)
        } else {
            os.Exit(1)
        }
    },
}
