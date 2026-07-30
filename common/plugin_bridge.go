package common

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/monobilisim/monokit/common/health"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// pluginConfigNameOverrides maps a plugin's component name to its monokit
// config name (as used by ConfExists) for the cases where it doesn't follow
// the "strip Health suffix, lowercase" convention (e.g. shared config files).
var pluginConfigNameOverrides = map[string]string{
	"zimbraHealth": "mail",
}

// pluginConfigName resolves the monokit config name a given plugin should be
// gated on, so that deleting /etc/mono/<name>.yaml disables the plugin even
// though it's loaded from an out-of-process binary.
func pluginConfigName(pluginName string) string {
	if name, ok := pluginConfigNameOverrides[pluginName]; ok {
		return name
	}
	return strings.ToLower(strings.TrimSuffix(pluginName, "Health"))
}

// isActualPlugin checks if a provider is a ProviderProxy (actual plugin binary)
// vs a built-in component like osHealth that's compiled into the main binary
func isActualPlugin(provider interface{}) bool {
	// Check if the provider type contains "ProviderProxy" which indicates it's from a plugin
	providerType := reflect.TypeOf(provider)
	if providerType != nil {
		// Get the type name, which should be "ProviderProxy" for actual plugins
		typeName := providerType.Elem().Name() // Use Elem() since it's likely a pointer
		isProxy := typeName == "ProviderProxy"
		log.Debug().Str("typeName", typeName).Bool("isProxy", isProxy).Msg("Provider type check")
		return isProxy
	}
	return false
}

// RegisterPluginBridgeComponents scans the health registry and creates bridge components
// for only actual plugin binaries (not built-in components). This should be called after plugins are loaded.
func RegisterPluginBridgeComponents() {
	providers := health.GetAllProviders()
	log.Debug().Int("providers", len(providers)).Msg("Scanning health registry for providers")

	for pluginName, provider := range providers {
		if provider != nil {
			// Check if this is actually a plugin (ProviderProxy) vs a built-in component
			if isActualPlugin(provider) {
				log.Debug().Str("pluginName", pluginName).Msg("Creating bridge component for actual plugin")

				// Create a closure to capture the plugin name for each component
				pluginNameCopy := pluginName // Important: capture the value, not the reference

				RegisterComponent(Component{
					Name:       pluginNameCopy,
					EntryPoint: createPluginExecutor(pluginNameCopy),
					Platform:   "any",
					AutoDetect: createPluginDetector(pluginNameCopy),
				})
			} else {
				log.Debug().Str("pluginName", pluginName).Msg("Skipping built-in component (not a plugin binary)")
			}
		}
	}
}

// RegisterPluginCLICommands scans the health registry and registers CLI commands
// for actual plugin binaries. This should be called after plugins are loaded.
func RegisterPluginCLICommands(rootCmd *cobra.Command) {
	providers := health.GetAllProviders()
	log.Debug().Int("providers", len(providers)).Msg("Scanning health registry for CLI commands")

	for pluginName, provider := range providers {
		if provider != nil && isActualPlugin(provider) {
			log.Debug().Str("pluginName", pluginName).Msg("Creating CLI command")

			// Create a closure to capture the plugin name
			pluginNameCopy := pluginName

			pluginCmd := &cobra.Command{
				Use:   pluginNameCopy,
				Short: fmt.Sprintf("Run %s checks (via plugin)", pluginNameCopy),
				Long:  fmt.Sprintf("Collects and displays %s information via the %s plugin.", pluginNameCopy, pluginNameCopy),
				Run:   createPluginExecutor(pluginNameCopy),
			}

			rootCmd.AddCommand(pluginCmd)
			log.Debug().Str("pluginName", pluginNameCopy).Msg("Added CLI command")
		}
	}
}

// createPluginDetector creates an auto-detection function for a specific plugin
func createPluginDetector(pluginName string) func() bool {
	return func() bool {
		provider := health.Get(pluginName)
		if provider == nil {
			log.Debug().Str("pluginName", pluginName).Bool("detected", false).Msg("Plugin AutoDetect check")
			return false
		}

		// The plugin binary being loaded isn't enough on its own — require its
		// monokit config file to exist too, so deleting it disables the check
		// even though the plugin process itself keeps running.
		configName := pluginConfigName(pluginName)
		if !ConfExists(configName) {
			log.Debug().Str("pluginName", pluginName).Str("configName", configName).Bool("detected", false).Msg("Plugin AutoDetect check: config file not found")
			return false
		}

		log.Debug().Str("pluginName", pluginName).Bool("detected", true).Msg("Plugin AutoDetect check")
		return true
	}
}

// createPluginExecutor creates an execution function for a specific plugin
func createPluginExecutor(pluginName string) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		// Get the plugin from health registry
		if pluginProvider := health.Get(pluginName); pluginProvider != nil {
			hostname, err := os.Hostname()
			if err != nil {
				log.Debug().Err(err).Msg("Error getting hostname, using localhost")
				hostname = "localhost"
			}

			log.Debug().Str("pluginName", pluginName).Str("hostname", hostname).Msg("Executing plugin")
			data, err := pluginProvider.Collect(hostname)
			if err != nil {
				log.Error().Str("pluginName", pluginName).Err(err).Msg("Error collecting data from plugin")
				fmt.Fprintf(os.Stderr, "Error collecting %s data from plugin: %v\n", pluginName, err)
				os.Exit(1)
			}

			// The plugin returns a pre-rendered string
			if renderedString, ok := data.(string); ok {
				fmt.Println(renderedString)
			} else {
				log.Error().Str("pluginName", pluginName).Str("data", fmt.Sprintf("%T", data)).Msg("Plugin returned unexpected type")
				fmt.Fprintf(os.Stderr, "Error: %s plugin returned unexpected type %T, expected string\n", pluginName, data)
				os.Exit(1)
			}
		} else {
			log.Error().Str("pluginName", pluginName).Msg("Plugin not available in health registry")
			fmt.Fprintf(os.Stderr, "%s plugin not available\n", pluginName)
			os.Exit(1)
		}
	}
}
