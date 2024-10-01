package main

import (
	"os"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/osHealth"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
)

var MonokitVersion = "devel"
var RootCmd = &cobra.Command{
	Use:     "monokit",
	Version: MonokitVersion,
}

func main() {
	var osHealthCmd = &cobra.Command{
		Use:   "osHealth",
		Short: "OS Health",
		Run:   osHealth.Main,
	}

    var redmineCmd = &cobra.Command{
        Use:   "redmine",
        Short: "Redmine-related utilities",
    }

	//// Common
	RootCmd.AddCommand(redmineCmd)
	RootCmd.AddCommand(common.AlarmCmd)

	/// Alarm

	// AlarmSend
	common.AlarmCmd.AddCommand(common.AlarmSendCmd)

	common.AlarmSendCmd.Flags().StringP("message", "m", "", "Message")
	common.AlarmSendCmd.MarkFlagRequired("message")

	// AlarmCheckUp
	common.AlarmCmd.AddCommand(common.AlarmCheckUpCmd)

	common.AlarmCheckUpCmd.Flags().StringP("service", "s", "", "Service Name")
	common.AlarmCheckUpCmd.Flags().StringP("message", "m", "", "Message")
	common.AlarmCheckUpCmd.Flags().StringP("scriptName", "n", "", "Script name")
	common.AlarmCheckUpCmd.MarkFlagRequired("message")
	common.AlarmCheckUpCmd.MarkFlagRequired("service")
	common.AlarmCheckUpCmd.MarkFlagRequired("scriptName")

	// AlarmCheckDown
	common.AlarmCmd.AddCommand(common.AlarmCheckDownCmd)

	common.AlarmCheckDownCmd.Flags().StringP("service", "s", "", "Service Name")
	common.AlarmCheckDownCmd.Flags().StringP("message", "m", "", "Message")
	common.AlarmCheckDownCmd.Flags().StringP("scriptName", "n", "", "Script name")
	common.AlarmCheckDownCmd.MarkFlagRequired("message")
	common.AlarmCheckDownCmd.MarkFlagRequired("service")
	common.AlarmCheckDownCmd.MarkFlagRequired("scriptName")

	/// Redmine
	redmineCmd.AddCommand(issues.IssueCmd)

	// RedmineCreate
	issues.IssueCmd.AddCommand(issues.CreateCmd)

	issues.CreateCmd.Flags().StringP("subject", "j", "", "Subject")
	issues.CreateCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.CreateCmd.Flags().StringP("message", "m", "", "Message")
	issues.CreateCmd.MarkFlagRequired("subject")
	issues.CreateCmd.MarkFlagRequired("service")
	issues.CreateCmd.MarkFlagRequired("message")

	// RedmineUpdate
	issues.IssueCmd.AddCommand(issues.UpdateCmd)

	issues.UpdateCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.UpdateCmd.Flags().StringP("message", "m", "", "Message")
	issues.UpdateCmd.Flags().BoolP("checkNote", "c", false, "Check Notes")
	issues.UpdateCmd.MarkFlagRequired("service")
	issues.UpdateCmd.MarkFlagRequired("message")

	// RedmineClose
	issues.IssueCmd.AddCommand(issues.CloseCmd)

	issues.CloseCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.CloseCmd.Flags().StringP("message", "m", "", "Message")
	issues.CloseCmd.MarkFlagRequired("service")
	issues.CloseCmd.MarkFlagRequired("message")

	// RedmineShow
	issues.IssueCmd.AddCommand(issues.ShowCmd)

	issues.ShowCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.ShowCmd.MarkFlagRequired("service")

	// RedmineExists
	issues.IssueCmd.AddCommand(issues.ExistsCmd)

	issues.ExistsCmd.Flags().StringP("subject", "j", "", "Subject")
	issues.ExistsCmd.Flags().StringP("date", "d", "", "Date")
	issues.ExistsCmd.Flags().BoolP("search", "s", false, "Search")

	issues.ExistsCmd.MarkFlagRequired("subject")

	// RedmineCheckUp
	issues.IssueCmd.AddCommand(issues.CheckUpCmd)

	issues.CheckUpCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.CheckUpCmd.Flags().StringP("message", "m", "", "Message")

	issues.CheckUpCmd.MarkFlagRequired("service")
	issues.CheckUpCmd.MarkFlagRequired("message")

	// RedmineCheckDown
	issues.IssueCmd.AddCommand(issues.CheckDownCmd)

	issues.CheckDownCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.CheckDownCmd.Flags().StringP("message", "m", "", "Message")
	issues.CheckDownCmd.Flags().StringP("subject", "j", "", "Subject")
	issues.CheckDownCmd.MarkFlagRequired("subject")
	issues.CheckDownCmd.MarkFlagRequired("service")
	issues.CheckDownCmd.MarkFlagRequired("message")

	/// OS Health
	RootCmd.AddCommand(osHealthCmd)

	RedisCommandAdd()

    RmqCommandAdd()

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
