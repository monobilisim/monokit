package main

import (
	"os"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/osHealth"
	"github.com/monobilisim/monokit/k8sHealth"
    "github.com/monobilisim/monokit/pmgHealth"
	"github.com/monobilisim/monokit/mysqlHealth"
	"github.com/monobilisim/monokit/shutdownNotifier"
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

    var mysqlHealthCmd = &cobra.Command{
        Use:   "mysqlHealth",
        Short: "MySQL Health",
        Run: mysqlHealth.Main,
    }

    var redmineCmd = &cobra.Command{
        Use:   "redmine",
        Short: "Redmine-related utilities",
    }

    var shutdownNotifierCmd = &cobra.Command{
        Use:   "shutdownNotifier",
        Short: "Shutdown Notifier",
        Run: shutdownNotifier.Main,
    }

    var k8sHealthCmd = &cobra.Command{
        Use:   "k8sHealth",
        Short: "Kubernetes Health",
        Run: k8sHealth.Main,
    }

    var pmgHealthCmd = &cobra.Command{
        Use:   "pmgHealth",
        Short: "Proxmox Mail Gateway Health",
        Run:   pmgHealth.Main,
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
	common.AlarmCheckUpCmd.Flags().BoolP("noInterval", "i", false, "Disable interval check")
	common.AlarmCheckUpCmd.MarkFlagRequired("message")
	common.AlarmCheckUpCmd.MarkFlagRequired("service")
	common.AlarmCheckUpCmd.MarkFlagRequired("scriptName")

	// AlarmCheckDown
	common.AlarmCmd.AddCommand(common.AlarmCheckDownCmd)

	common.AlarmCheckDownCmd.Flags().StringP("service", "s", "", "Service Name")
	common.AlarmCheckDownCmd.Flags().StringP("message", "m", "", "Message")
	common.AlarmCheckDownCmd.Flags().StringP("scriptName", "n", "", "Script name")
	common.AlarmCheckDownCmd.Flags().BoolP("noInterval", "i", false, "Disable interval check")
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

    // issues.ExistsNoteCmd
    issues.IssueCmd.AddCommand(issues.ExistsNoteCmd)

    issues.ExistsNoteCmd.Flags().StringP("service", "s", "", "Service Name")
    issues.ExistsNoteCmd.Flags().StringP("note", "n", "", "Note")

    issues.ExistsNoteCmd.MarkFlagRequired("service")
    issues.ExistsNoteCmd.MarkFlagRequired("note")

    // issues.DeleteCmd
    issues.IssueCmd.AddCommand(issues.DeleteCmd)

    issues.DeleteCmd.Flags().IntP("id", "i", 0, "Issue ID")

    issues.DeleteCmd.MarkFlagRequired("id")

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

    /// MySQL Health
	RootCmd.AddCommand(mysqlHealthCmd)

	RedisCommandAdd()

    RmqCommandAdd()

    /// Shutdown Notifier
    RootCmd.AddCommand(shutdownNotifierCmd)

    /// Proxmox Mail Gateway Health
    RootCmd.AddCommand(pmgHealthCmd)

    shutdownNotifierCmd.Flags().BoolP("poweron", "1", false, "Power On")
    shutdownNotifierCmd.Flags().BoolP("poweroff", "0", false, "Power Off")

    /// Kubernetes Health
    RootCmd.AddCommand(k8sHealthCmd)
    
    kubeconfig := os.Getenv("KUBECONFIG")

    if kubeconfig == "" {
        kubeconfig = os.Getenv("HOME") + "/.kube/config"
    }

    k8sHealthCmd.Flags().StringP("kubeconfig", "k", kubeconfig, "Kubeconfig file")

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
