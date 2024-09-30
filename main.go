package main

import (
	"fmt"
	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/osHealth"
	"github.com/monobilisim/monokit/rabbitmq"
	"github.com/spf13/cobra"
	"os"
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
	RootCmd.AddCommand(common.RedmineCmd)
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
	common.RedmineUpdateCmd.Flags().BoolP("checkNote", "c", false, "Check Notes")
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

	// RedmineExists
	common.RedmineIssueCmd.AddCommand(common.RedmineExistsCmd)

	common.RedmineExistsCmd.Flags().StringP("subject", "j", "", "Subject")
	common.RedmineExistsCmd.Flags().StringP("date", "d", "", "Date")
	common.RedmineExistsCmd.Flags().BoolP("search", "s", false, "Search")

	common.RedmineExistsCmd.MarkFlagRequired("subject")

	// RedmineCheckUp
	common.RedmineIssueCmd.AddCommand(common.RedmineCheckUpCmd)

	common.RedmineCheckUpCmd.Flags().StringP("service", "s", "", "Service Name")
	common.RedmineCheckUpCmd.Flags().StringP("message", "m", "", "Message")

	common.RedmineCheckUpCmd.MarkFlagRequired("service")
	common.RedmineCheckUpCmd.MarkFlagRequired("message")

	// RedmineCheckDown
	common.RedmineIssueCmd.AddCommand(common.RedmineCheckDownCmd)

	common.RedmineCheckDownCmd.Flags().StringP("service", "s", "", "Service Name")
	common.RedmineCheckDownCmd.Flags().StringP("message", "m", "", "Message")
	common.RedmineCheckDownCmd.Flags().StringP("subject", "j", "", "Subject")
	common.RedmineCheckDownCmd.MarkFlagRequired("subject")
	common.RedmineCheckDownCmd.MarkFlagRequired("service")
	common.RedmineCheckDownCmd.MarkFlagRequired("message")

	/// OS Health
	RootCmd.AddCommand(osHealthCmd)

	RedisCommandAdd()

	RootCmd.AddCommand(&cobra.Command{
		Use:   "rabbitmqHealth",
		Short: "RabbitMQ Health",
		Run:   rabbitmq.Main,
	})

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
