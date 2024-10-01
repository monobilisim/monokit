package main

import (
	"os"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/osHealth"
	news "github.com/monobilisim/monokit/common/redmine/news"
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
    redmineCmd.AddCommand(news.NewsCmd)

	// issues.CreateCmd
	issues.IssueCmd.AddCommand(issues.CreateCmd)

	issues.CreateCmd.Flags().StringP("subject", "j", "", "Subject")
	issues.CreateCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.CreateCmd.Flags().StringP("message", "m", "", "Message")
	issues.CreateCmd.MarkFlagRequired("subject")
	issues.CreateCmd.MarkFlagRequired("service")
	issues.CreateCmd.MarkFlagRequired("message")

	// issues.UpdateCmd
	issues.IssueCmd.AddCommand(issues.UpdateCmd)

	issues.UpdateCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.UpdateCmd.Flags().StringP("message", "m", "", "Message")
	issues.UpdateCmd.Flags().BoolP("checkNote", "c", false, "Check Notes")
	issues.UpdateCmd.MarkFlagRequired("service")
	issues.UpdateCmd.MarkFlagRequired("message")

	// issues.CloseCmd
	issues.IssueCmd.AddCommand(issues.CloseCmd)

	issues.CloseCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.CloseCmd.Flags().StringP("message", "m", "", "Message")
	issues.CloseCmd.MarkFlagRequired("service")
	issues.CloseCmd.MarkFlagRequired("message")

	// issues.ShowCmd
	issues.IssueCmd.AddCommand(issues.ShowCmd)

	issues.ShowCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.ShowCmd.MarkFlagRequired("service")

	// issues.ExistsCmd
	issues.IssueCmd.AddCommand(issues.ExistsCmd)

	issues.ExistsCmd.Flags().StringP("subject", "j", "", "Subject")
	issues.ExistsCmd.Flags().StringP("date", "d", "", "Date")
	issues.ExistsCmd.Flags().BoolP("search", "s", false, "Search")

	issues.ExistsCmd.MarkFlagRequired("subject")

	// issues.CheckUpCmd
	issues.IssueCmd.AddCommand(issues.CheckUpCmd)

	issues.CheckUpCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.CheckUpCmd.Flags().StringP("message", "m", "", "Message")

	issues.CheckUpCmd.MarkFlagRequired("service")
	issues.CheckUpCmd.MarkFlagRequired("message")

	// issues.CheckDownCmd
	issues.IssueCmd.AddCommand(issues.CheckDownCmd)

	issues.CheckDownCmd.Flags().StringP("service", "s", "", "Service Name")
	issues.CheckDownCmd.Flags().StringP("message", "m", "", "Message")
	issues.CheckDownCmd.Flags().StringP("subject", "j", "", "Subject")
	issues.CheckDownCmd.MarkFlagRequired("subject")
	issues.CheckDownCmd.MarkFlagRequired("service")
	issues.CheckDownCmd.MarkFlagRequired("message")

    // news.CreateCmd
    news.NewsCmd.AddCommand(news.CreateCmd)

    news.CreateCmd.Flags().StringP("title", "t", "", "Title")
    news.CreateCmd.Flags().StringP("description", "d", "", "Description")
    news.CreateCmd.Flags().BoolP("noDuplicate", "n", false, "Check for duplicates, return ID if exists")

    news.CreateCmd.MarkFlagRequired("title")
    news.CreateCmd.MarkFlagRequired("description")

    // news.DeleteCmd
    news.NewsCmd.AddCommand(news.DeleteCmd)

    news.DeleteCmd.Flags().StringP("id", "i", "", "News ID")

    news.DeleteCmd.MarkFlagRequired("id")

    // news.ExistsCmd
    news.NewsCmd.AddCommand(news.ExistsCmd)

    news.ExistsCmd.Flags().StringP("title", "t", "", "Title")
    news.ExistsCmd.Flags().StringP("description", "d", "", "Description")

    news.ExistsCmd.MarkFlagRequired("title")
    news.ExistsCmd.MarkFlagRequired("description")

	/// OS Health
	RootCmd.AddCommand(osHealthCmd)

	RedisCommandAdd()

    RmqCommandAdd()

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
