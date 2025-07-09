package lbPolicy

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type ConfigStruct struct {
	Caddy struct {
		Api_Urls                []string                `mapstructure:"api_urls" yaml:"api_urls"`
		Servers                 []string                `mapstructure:"servers" yaml:"servers"`
		Lb_Urls                 []string                `mapstructure:"lb_urls" yaml:"lb_urls"`
		Override_Config         bool                    `mapstructure:"override_config" yaml:"override_config"`
		Nochange_Exit_Threshold int                     `mapstructure:"nochange_exit_threshold" yaml:"nochange_exit_threshold"`
		Dynamic_Api_Urls        bool                    `mapstructure:"dynamic_api_urls" yaml:"dynamic_api_urls"`
		Loop_Order              string                  `mapstructure:"loop_order" yaml:"loop_order"`
		Lb_Policy_Change_Sleep  time.Duration           `mapstructure:"lb_policy_change_sleep" yaml:"lb_policy_change_sleep"`
		PatroniAutoSwitch       PatroniAutoSwitchConfig `mapstructure:"patroni_auto_switch" yaml:"patroni_auto_switch"`
	} `mapstructure:"caddy" yaml:"caddy"`

	Alarm struct {
		Stream string `mapstructure:"stream" yaml:"stream"`
		Topic  string `mapstructure:"topic" yaml:"topic"`
	} `mapstructure:"alarm" yaml:"alarm"`
}

var Config ConfigStruct

func ConfReset() {
	Config = ConfigStruct{}
}

func LoopOverConfigs(server string, configs []string, t string) {
	if len(configs) > 0 {
		var printConfig bool

		if len(configs) > 1 {
			printConfig = true
		}

		for _, config := range configs {
			ConfReset()
			if printConfig {
				fmt.Println("Config: " + config)
			}
			common.ConfInit("glb-"+config, &Config)
			switch t {
			case "switch":
				SwitchMain(server)
			case "list":
				ShowListMulti(Config.Caddy.Servers)
			}

			if printConfig {
				fmt.Println("")
			}
		}
	} else {
		// Loop over all files at /etc/mono that start with glb-
		entries, err := os.ReadDir("/etc/mono/")
		if err != nil {
			log.Error().Err(err).Str("component", "lbPolicy").Str("operation", "LoopOverConfigs").Str("action", "read_dir_failed").Msg("Error reading directory")
		}
		for _, file := range entries {
			if strings.HasPrefix(file.Name(), "glb-") {
				ConfReset()
				fmt.Println("Config: " + file.Name())
				common.ConfInit(file.Name(), &Config)
				switch t {
				case "switch":
					SwitchMain(server)
				case "list":
					ShowListMulti(Config.Caddy.Servers)
				}
			}
		}
	}
}

func Switch(cmd *cobra.Command, args []string) {
	//version := "2.0.0"
	common.ScriptName = "lbPolicy"
	common.TmpDir = common.TmpDir + "glb"
	common.Init()
	config, _ := cmd.Flags().GetStringArray("configs")
	server, _ := cmd.Flags().GetString("server")
	fmt.Println("Server: " + server)
	LoopOverConfigs(server, config, "switch")
}

func List(cmd *cobra.Command, args []string) {
	//version := "2.0.0"
	common.ScriptName = "lbPolicy"
	common.TmpDir = "/tmp/glb"
	common.Init()
	config, _ := cmd.Flags().GetStringArray("configs")
	LoopOverConfigs("", config, "list")
}

func PatroniMonitorStart(cmd *cobra.Command, args []string) {
	common.ScriptName = "lbPolicy"
	common.TmpDir = common.TmpDir + "glb"
	common.Init()

	config, _ := cmd.Flags().GetStringArray("configs")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	fmt.Println("Starting Patroni monitor...")

	if len(config) > 0 {
		for _, configName := range config {
			ConfReset()
			fmt.Println("Config: " + configName)
			common.ConfInit("glb-"+configName, &Config)

			// Override dry-run if flag is set
			if dryRun {
				Config.Caddy.PatroniAutoSwitch.DryRun = true
			}

			startPatroniMonitor()
		}
	} else {
		// Loop over all files at /etc/mono that start with glb-
		entries, err := os.ReadDir("/etc/mono/")
		if err != nil {
			log.Error().Err(err).Str("component", "lbPolicy").Str("operation", "PatroniMonitorStart").Str("action", "read_dir_failed").Msg("Error reading directory")
			return
		}

		for _, file := range entries {
			if strings.HasPrefix(file.Name(), "glb-") {
				ConfReset()
				fmt.Println("Config: " + file.Name())
				common.ConfInit(file.Name(), &Config)

				// Override dry-run if flag is set
				if dryRun {
					Config.Caddy.PatroniAutoSwitch.DryRun = true
				}

				startPatroniMonitor()
			}
		}
	}
}

func PatroniCheck(cmd *cobra.Command, args []string) {
	common.ScriptName = "lbPolicy"
	common.TmpDir = common.TmpDir + "glb"
	common.Init()

	config, _ := cmd.Flags().GetStringArray("configs")

	fmt.Println("Checking Patroni clusters...")

	if len(config) > 0 {
		for _, configName := range config {
			ConfReset()
			common.ConfInit("glb-"+configName, &Config)
			performPatroniCheck()
		}
	} else {
		// Loop over all files at /etc/mono that start with glb-
		entries, err := os.ReadDir("/etc/mono/")
		if err != nil {
			log.Error().Err(err).Str("component", "lbPolicy").Str("operation", "PatroniCheck").Str("action", "read_dir_failed").Msg("Error reading directory")
			return
		}

		for _, file := range entries {
			if strings.HasPrefix(file.Name(), "glb-") {
				ConfReset()
				common.ConfInit(file.Name(), &Config)
				performPatroniCheck()
			}
		}
	}
}

func startPatroniMonitor() {
	monitor := NewPatroniMonitor(Config.Caddy.PatroniAutoSwitch)

	if err := monitor.Start(); err != nil {
		log.Error().Err(err).Str("component", "lbPolicy").Str("operation", "startPatroniMonitor").Str("action", "start_failed").Msg("Failed to start Patroni monitor")
		return
	}

	fmt.Println("Patroni monitor started (use Ctrl+C to stop)")
	// Block until interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Stopping Patroni monitor...")
	monitor.Stop()
}

func performPatroniCheck() {
	monitor := NewPatroniMonitor(Config.Caddy.PatroniAutoSwitch)

	if !Config.Caddy.PatroniAutoSwitch.Enabled {
		fmt.Println("Patroni auto-switch is disabled")
		return
	}

	fmt.Printf("Checking %d Patroni mappings...\n", len(Config.Caddy.PatroniAutoSwitch.Mappings))

	for i, mapping := range Config.Caddy.PatroniAutoSwitch.Mappings {
		fmt.Printf("\nMapping %d: %s -> %s\n", i+1, mapping.Cluster, mapping.SwitchTo)
		fmt.Printf("Patroni URLs: %v\n", mapping.PatroniUrls)

		primary, err := monitor.CheckClusterPrimary(mapping)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		fmt.Printf("✅ Primary node: %s\n", primary)

	}
}
