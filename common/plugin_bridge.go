package common

import (
	"fmt"
	"os"
	"reflect"

	"github.com/monobilisim/monokit/common/health"
	"github.com/spf13/cobra"
)

// isActualPlugin checks if a provider is a ProviderProxy (actual plugin binary)
// vs a built-in component like osHealth that's compiled into the main binary
func isActualPlugin(provider interface{}) bool {
	// Check if the provider type contains "ProviderProxy" which indicates it's from a plugin
	providerType := reflect.TypeOf(provider)
	if providerType != nil {
		// Get the type name, which should be "ProviderProxy" for actual plugins
		typeName := providerType.Elem().Name() // Use Elem() since it's likely a pointer
		isProxy := typeName == "ProviderProxy"
		LogDebug(fmt.Sprintf("Provider type check: %s, isProxy: %t", typeName, isProxy))
		return isProxy
	}
	return false
}

// RegisterPluginBridgeComponents scans the health registry and creates bridge components
// for only actual plugin binaries (not built-in components). This should be called after plugins are loaded.
func RegisterPluginBridgeComponents() {
	providers := health.GetAllProviders()
	LogDebug(fmt.Sprintf("Scanning health registry for providers, found %d total", len(providers)))

	for pluginName, provider := range providers {
		if provider != nil {
			// Check if this is actually a plugin (ProviderProxy) vs a built-in component
			if isActualPlugin(provider) {
				LogDebug(fmt.Sprintf("Creating bridge component for actual plugin: %s", pluginName))

				// Create a closure to capture the plugin name for each component
				pluginNameCopy := pluginName // Important: capture the value, not the reference

				RegisterComponent(Component{
					Name:       pluginNameCopy,
					EntryPoint: createPluginExecutor(pluginNameCopy),
					Platform:   "any",
					AutoDetect: createPluginDetector(pluginNameCopy),
				})
			} else {
				LogDebug(fmt.Sprintf("Skipping built-in component: %s (not a plugin binary)", pluginName))
			}
		}
	}
}

// RegisterPluginCLICommands scans the health registry and registers CLI commands
// for actual plugin binaries. This should be called after plugins are loaded.
func RegisterPluginCLICommands(rootCmd *cobra.Command) {
	providers := health.GetAllProviders()
	LogDebug(fmt.Sprintf("Scanning health registry for CLI commands, found %d providers", len(providers)))

	for pluginName, provider := range providers {
		if provider != nil && isActualPlugin(provider) {
			LogDebug(fmt.Sprintf("Creating CLI command for plugin: %s", pluginName))

			// Create a closure to capture the plugin name
			pluginNameCopy := pluginName

			pluginCmd := &cobra.Command{
				Use:   pluginNameCopy,
				Short: fmt.Sprintf("Run %s checks (via plugin)", pluginNameCopy),
				Long:  fmt.Sprintf("Collects and displays %s information via the %s plugin.", pluginNameCopy, pluginNameCopy),
				Run:   createPluginExecutor(pluginNameCopy),
			}

			rootCmd.AddCommand(pluginCmd)
			LogDebug(fmt.Sprintf("Added CLI command: %s", pluginNameCopy))
		}
	}
}

// createPluginDetector creates an auto-detection function for a specific plugin
func createPluginDetector(pluginName string) func() bool {
	return func() bool {
		provider := health.Get(pluginName)
		detected := provider != nil
		LogDebug(fmt.Sprintf("Plugin %s AutoDetect check: provider found = %t", pluginName, detected))
		return detected
	}
}

// createPluginExecutor creates an execution function for a specific plugin
func createPluginExecutor(pluginName string) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		// Get the plugin from health registry
		if pluginProvider := health.Get(pluginName); pluginProvider != nil {
			hostname, err := os.Hostname()
			if err != nil {
				LogDebug(fmt.Sprintf("Error getting hostname: %v, using localhost", err))
				hostname = "localhost"
			}

			LogDebug(fmt.Sprintf("Executing %s plugin with hostname: %s", pluginName, hostname))
			data, err := pluginProvider.Collect(hostname)
			if err != nil {
				LogError(fmt.Sprintf("Error collecting %s data from plugin: %v", pluginName, err))
				fmt.Fprintf(os.Stderr, "Error collecting %s data from plugin: %v\n", pluginName, err)
				os.Exit(1)
			}

			// The plugin returns a pre-rendered string
			if renderedString, ok := data.(string); ok {
				fmt.Println(renderedString)
			} else {
				LogError(fmt.Sprintf("%s plugin returned unexpected type %T, expected string", pluginName, data))
				fmt.Fprintf(os.Stderr, "Error: %s plugin returned unexpected type %T, expected string\n", pluginName, data)
				os.Exit(1)
			}
		} else {
			LogError(fmt.Sprintf("%s plugin not available in health registry", pluginName))
			fmt.Fprintf(os.Stderr, "%s plugin not available\n", pluginName)
			os.Exit(1)
		}
	}
}
