package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	"github.com/monobilisim/monokit/common/api/server"
	"github.com/monobilisim/monokit/common/health"
	plugin "github.com/monobilisim/monokit/common/health/plugin"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	news "github.com/monobilisim/monokit/common/redmine/news"
	verCheck "github.com/monobilisim/monokit/common/versionCheck"
	"github.com/monobilisim/monokit/daemon"
	"github.com/monobilisim/monokit/esHealth"
	"github.com/monobilisim/monokit/lbPolicy"
	"github.com/monobilisim/monokit/logs"
	"github.com/monobilisim/monokit/osHealth"
	"github.com/monobilisim/monokit/shutdownNotifier"
	"github.com/monobilisim/monokit/sshNotifier"
	"github.com/monobilisim/monokit/ufwApply"
)

var RootCmd = &cobra.Command{
	Use:     "monokit",
	Version: common.MonokitVersion,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// This function runs before any command's Run function.
		// Cobra automatically binds the flag value to the variable via PersistentFlags().
	},
}

func init() {
	common.InitZerolog()

	// Define persistent flags in init() so they are available early.
	RootCmd.PersistentFlags().BoolVar(&common.IgnoreLockfile, "ignore-lockfile", false, "Ignore lockfile check during initialization")
	RootCmd.PersistentFlags().BoolVar(&common.CleanupPluginsOnExit, "cleanup-plugins", false, "Clean up plugin processes before exit (used internally by daemon)")
}

func main() {
	// Set up graceful shutdown handling for plugins
	setupGracefulShutdown()

	// Ensure plugins are always cleaned up before exit
	defer func() {

		plugin.CleanupAll()
	}()

	// Wire up plugin loader registration to health registry
	plugin.RegisterProviderGlobally = func(p interface{}) {
		if prov, ok := p.(health.Provider); ok {
			health.Register(prov)
		}
	}

	// Load health plugins at startup
	if err := plugin.LoadAll(common.DefaultPluginDir); err != nil {
		log.Warn().Str("pluginDir", common.DefaultPluginDir).Err(err).Msg("Failed to load some plugins")
	}

	// Register bridge components for all loaded plugins
	common.RegisterPluginBridgeComponents()

	// Register CLI commands for all loaded plugins
	common.RegisterPluginCLICommands(RootCmd)

	var osHealthCmd = &cobra.Command{
		Use:   "osHealth",
		Short: "OS Health",
		Run:   osHealth.Main,
	}

	var redmineCmd = &cobra.Command{
		Use:   "redmine",
		Short: "Redmine-related utilities",
	}

	var shutdownNotifierCmd = &cobra.Command{
		Use:   "shutdownNotifier",
		Short: "Shutdown Notifier",
		Run:   shutdownNotifier.Main,
	}

	// Removing monolithic k8sHealth CLI command for plugin-only operation.

	var esHealthCmd = &cobra.Command{
		Use:   "esHealth",
		Short: "Elasticsearch Health",
		Run:   esHealth.Main,
	}

	var sshNotifierCmd = &cobra.Command{
		Use:   "sshNotifier",
		Short: "SSH Notifier",
		Run:   sshNotifier.Main,
	}

	var lbPolicyCmd = &cobra.Command{
		Use:   "lbPolicy",
		Short: "Load Balancer Policy Switcher/Viewer",
	}

	var lbPolicySwitchCmd = &cobra.Command{
		Use:   "switch",
		Short: "Switch Load Balancer Policy",
		Run:   lbPolicy.Switch,
	}

	var lbPolicyListCmd = &cobra.Command{
		Use:   "list",
		Short: "List Load Balancer Policies",
		Run:   lbPolicy.List,
	}

	var lbPolicyPatroniCmd = &cobra.Command{
		Use:   "patroni",
		Short: "Patroni cluster management",
	}

	var lbPolicyPatroniMonitorCmd = &cobra.Command{
		Use:   "monitor",
		Short: "Start Patroni Auto-Switch Monitor",
		Run:   lbPolicy.PatroniMonitorStart,
	}

	var lbPolicyPatroniCheckCmd = &cobra.Command{
		Use:   "check",
		Short: "Check Patroni Cluster Status",
		Run:   lbPolicy.PatroniCheck,
	}

	var daemon = &cobra.Command{
		Use:   "daemon",
		Short: "Daemon",
		Run:   daemon.Main,
	}

	var versionCheckCmd = &cobra.Command{
		Use:   "versionCheck",
		Short: "Version Check",
		Run:   verCheck.VersionCheck,
	}

	var serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Monokit API Server",
		Run:   server.ServerMain,
	}

	var clientCmd = &cobra.Command{
		Use:   "client",
		Short: "Monokit API Client, used for managing and updating information",
	}

	var clientUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update the server info",
		Run:   client.Update,
	}

	//// Common
	RootCmd.AddCommand(redmineCmd)
	RootCmd.AddCommand(common.AlarmCmd)

	common.UpdateCmd.Flags().StringP("version", "v", "", "Custom version")
	common.UpdateCmd.Flags().BoolP("force", "f", false, "Force update")
	RootCmd.AddCommand(common.UpdateCmd)

	// Plugin commands
	RootCmd.AddCommand(common.PluginCmd)
	common.PluginCmd.AddCommand(common.PluginInstallCmd)
	common.PluginCmd.AddCommand(common.PluginListCmd)
	common.PluginCmd.AddCommand(common.PluginUninstallCmd)

	/// Alarm

	// AlarmSend
	common.AlarmCmd.AddCommand(common.AlarmSendCmd)

	common.AlarmSendCmd.Flags().StringP("message", "m", "", "Message")
	common.AlarmSendCmd.Flags().StringP("stream", "s", "", "Stream")
	common.AlarmSendCmd.Flags().StringP("topic", "t", "", "Topic")
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

	/// Daemon
	RootCmd.AddCommand(daemon)

	daemon.Flags().Bool("once", false, "Run once and exit")
	daemon.Flags().Bool("list-components", false, "List installed components")

	/// OS Health
	RootCmd.AddCommand(osHealthCmd)

	/// Vault Health
	VaultCommandAdd()

	/// Search Health
	RootCmd.AddCommand(esHealthCmd)

	MysqlCommandAdd()

	RmqCommandAdd()

	/// Shutdown Notifier
	RootCmd.AddCommand(shutdownNotifierCmd)

	PostalCommandAdd()

	PmgCommandAdd()

	TraefikCommandAdd()

	PgsqlCommandAdd()

	ZimbraCommandAdd()

	shutdownNotifierCmd.Flags().BoolP("poweron", "1", false, "Power On")
	shutdownNotifierCmd.Flags().BoolP("poweroff", "0", false, "Power Off")

	/// SSH Notifier
	RootCmd.AddCommand(sshNotifierCmd)

	/// Version Check
	RootCmd.AddCommand(versionCheckCmd)

	/// API
	RootCmd.AddCommand(serverCmd)
	clientCmd.AddCommand(clientUpdateCmd)

	RootCmd.AddCommand(clientCmd)

	// / UFW Applier
	RootCmd.AddCommand(ufwApply.UfwCmd) // Add UFW command to root command

	// / Load Balancer Policy
	RootCmd.AddCommand(lbPolicyCmd)

	// / Logs
	RootCmd.AddCommand(logs.NewLogsCmd())

	lbPolicyCmd.AddCommand(lbPolicySwitchCmd)
	lbPolicySwitchCmd.Flags().StringP("server", "s", "", "Server")
	lbPolicySwitchCmd.MarkFlagRequired("server")
	lbPolicySwitchCmd.Flags().StringArrayP("configs", "c", nil, "Config name (default: all)")

	lbPolicyCmd.AddCommand(lbPolicyListCmd)
	lbPolicyListCmd.Flags().StringArrayP("configs", "c", nil, "Config names to output the list of (default: all)")

	// Add patroni parent command to lbPolicy
	lbPolicyCmd.AddCommand(lbPolicyPatroniCmd)

	// Add subcommands to patroni
	lbPolicyPatroniCmd.AddCommand(lbPolicyPatroniMonitorCmd)
	lbPolicyPatroniMonitorCmd.Flags().StringArrayP("configs", "c", nil, "Config names to monitor (default: all)")
	lbPolicyPatroniMonitorCmd.Flags().BoolP("dry-run", "d", false, "Enable dry-run mode")

	lbPolicyPatroniCmd.AddCommand(lbPolicyPatroniCheckCmd)
	lbPolicyPatroniCheckCmd.Flags().StringArrayP("configs", "c", nil, "Config names to check (default: all)")

	sshNotifierCmd.Flags().BoolP("login", "1", false, "Login")
	sshNotifierCmd.Flags().BoolP("logout", "0", false, "Logout")

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		// Plugin cleanup will be handled by defer
		os.Exit(1)
	}
	// Plugin cleanup will be handled by defer
}

// setupGracefulShutdown sets up signal handlers for graceful shutdown
func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Debug().Msg("Received shutdown signal, cleaning up plugins...")
		plugin.CleanupAll()
		os.Exit(0)
	}()
}
