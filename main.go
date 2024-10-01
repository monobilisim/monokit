package main

import (
	"os"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/osHealth"
	redmine "github.com/monobilisim/monokit/common/redmine"
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

	//// Common
	RootCmd.AddCommand(redmine.RedmineCmd)
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
	redmine.RedmineCmd.AddCommand(redmine.RedmineIssueCmd)

	// RedmineCreate
	redmine.RedmineIssueCmd.AddCommand(redmine.RedmineCreateCmd)

	redmine.RedmineCreateCmd.Flags().StringP("subject", "j", "", "Subject")
	redmine.RedmineCreateCmd.Flags().StringP("service", "s", "", "Service Name")
	redmine.RedmineCreateCmd.Flags().StringP("message", "m", "", "Message")
	redmine.RedmineCreateCmd.MarkFlagRequired("subject")
	redmine.RedmineCreateCmd.MarkFlagRequired("service")
	redmine.RedmineCreateCmd.MarkFlagRequired("message")

	// RedmineUpdate
	redmine.RedmineIssueCmd.AddCommand(redmine.RedmineUpdateCmd)

	redmine.RedmineUpdateCmd.Flags().StringP("service", "s", "", "Service Name")
	redmine.RedmineUpdateCmd.Flags().StringP("message", "m", "", "Message")
	redmine.RedmineUpdateCmd.Flags().BoolP("checkNote", "c", false, "Check Notes")
	redmine.RedmineUpdateCmd.MarkFlagRequired("service")
	redmine.RedmineUpdateCmd.MarkFlagRequired("message")

	// RedmineClose
	redmine.RedmineIssueCmd.AddCommand(redmine.RedmineCloseCmd)

	redmine.RedmineCloseCmd.Flags().StringP("service", "s", "", "Service Name")
	redmine.RedmineCloseCmd.Flags().StringP("message", "m", "", "Message")
	redmine.RedmineCloseCmd.MarkFlagRequired("service")
	redmine.RedmineCloseCmd.MarkFlagRequired("message")

	// RedmineShow
	redmine.RedmineIssueCmd.AddCommand(redmine.RedmineShowCmd)

	redmine.RedmineShowCmd.Flags().StringP("service", "s", "", "Service Name")
	redmine.RedmineShowCmd.MarkFlagRequired("service")

	// RedmineExists
	redmine.RedmineIssueCmd.AddCommand(redmine.RedmineExistsCmd)

	redmine.RedmineExistsCmd.Flags().StringP("subject", "j", "", "Subject")
	redmine.RedmineExistsCmd.Flags().StringP("date", "d", "", "Date")
	redmine.RedmineExistsCmd.Flags().BoolP("search", "s", false, "Search")

	redmine.RedmineExistsCmd.MarkFlagRequired("subject")

	// RedmineCheckUp
	redmine.RedmineIssueCmd.AddCommand(redmine.RedmineCheckUpCmd)

	redmine.RedmineCheckUpCmd.Flags().StringP("service", "s", "", "Service Name")
	redmine.RedmineCheckUpCmd.Flags().StringP("message", "m", "", "Message")

	redmine.RedmineCheckUpCmd.MarkFlagRequired("service")
	redmine.RedmineCheckUpCmd.MarkFlagRequired("message")

	// RedmineCheckDown
	redmine.RedmineIssueCmd.AddCommand(redmine.RedmineCheckDownCmd)

	redmine.RedmineCheckDownCmd.Flags().StringP("service", "s", "", "Service Name")
	redmine.RedmineCheckDownCmd.Flags().StringP("message", "m", "", "Message")
	redmine.RedmineCheckDownCmd.Flags().StringP("subject", "j", "", "Subject")
	redmine.RedmineCheckDownCmd.MarkFlagRequired("subject")
	redmine.RedmineCheckDownCmd.MarkFlagRequired("service")
	redmine.RedmineCheckDownCmd.MarkFlagRequired("message")

	/// OS Health
	RootCmd.AddCommand(osHealthCmd)

	RedisCommandAdd()

    RmqCommandAdd()

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
