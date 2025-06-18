package common

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var PluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Plugin management utilities",
	Long:  "Install, list, and uninstall monokit plugins",
}

var PluginInstallCmd = &cobra.Command{
	Use:   "install [plugin-name...]",
	Short: "Install one or more plugins",
	Long:  "Install plugins from the official monokit repository",
	Args:  cobra.MinimumNArgs(1),
	Run:   pluginInstallRun,
}

var PluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available and installed plugins",
	Long:  "List all available plugins or only installed ones",
	Run:   pluginListRun,
}

var PluginUninstallCmd = &cobra.Command{
	Use:   "uninstall [plugin-name...]",
	Short: "Uninstall one or more plugins",
	Long:  "Remove installed plugins from the system",
	Args:  cobra.MinimumNArgs(1),
	Run:   pluginUninstallRun,
}

func init() {
	// Plugin install flags
	PluginInstallCmd.Flags().String("version", "", "Specific version to install (default: latest)")
	PluginInstallCmd.Flags().String("plugin-dir", DefaultPluginDir, "Plugin directory path")
	PluginInstallCmd.Flags().Bool("force", false, "Force installation even if plugin exists")

	// Plugin list flags
	PluginListCmd.Flags().Bool("installed", false, "Show only installed plugins")
	PluginListCmd.Flags().String("plugin-dir", DefaultPluginDir, "Plugin directory path")

	// Plugin uninstall flags
	PluginUninstallCmd.Flags().String("plugin-dir", DefaultPluginDir, "Plugin directory path")
	PluginUninstallCmd.Flags().Bool("force", false, "Force uninstall without confirmation")
}

func pluginInstallRun(cmd *cobra.Command, args []string) {
	Init()
	version, _ := cmd.Flags().GetString("version")
	pluginDir, _ := cmd.Flags().GetString("plugin-dir")
	force, _ := cmd.Flags().GetBool("force")

	// Validate plugin names
	invalidPlugins := validatePluginNames(args)
	if len(invalidPlugins) > 0 {
		LogError(fmt.Sprintf("Invalid plugin names: %s", strings.Join(invalidPlugins, ", ")))
		LogError(fmt.Sprintf("Available plugins: %s", strings.Join(KnownPlugins, ", ")))
		os.Exit(1)
	}

	err := InstallPlugins(version, args, pluginDir, force)
	if err != nil {
		LogError("Plugin installation failed: " + err.Error())
		os.Exit(1)
	}
}

func pluginListRun(cmd *cobra.Command, args []string) {
	Init()
	installed, _ := cmd.Flags().GetBool("installed")
	pluginDir, _ := cmd.Flags().GetString("plugin-dir")

	if installed {
		listInstalledPlugins(pluginDir)
	} else {
		listAllPlugins(pluginDir)
	}
}

func pluginUninstallRun(cmd *cobra.Command, args []string) {
	Init()
	pluginDir, _ := cmd.Flags().GetString("plugin-dir")
	force, _ := cmd.Flags().GetBool("force")

	err := UninstallPlugins(args, pluginDir, force)
	if err != nil {
		LogError("Plugin uninstallation failed: " + err.Error())
		os.Exit(1)
	}
}

func validatePluginNames(pluginNames []string) []string {
	var invalid []string
	for _, name := range pluginNames {
		if !contains(KnownPlugins, name) {
			invalid = append(invalid, name)
		}
	}
	return invalid
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func listInstalledPlugins(pluginDir string) {
	plugins, err := DetectInstalledPlugins(pluginDir)
	if err != nil {
		LogError("Failed to detect installed plugins: " + err.Error())
		return
	}

	if len(plugins) == 0 {
		fmt.Println("No plugins installed")
		return
	}

	fmt.Println("Installed plugins:")
	for _, plugin := range plugins {
		fmt.Printf("  - %s\n", plugin.Name)
	}
}

func listAllPlugins(pluginDir string) {
	// Get installed plugins
	installedPlugins, err := DetectInstalledPlugins(pluginDir)
	if err != nil {
		LogWarn("Failed to detect installed plugins: " + err.Error())
	}

	installedMap := make(map[string]bool)
	for _, plugin := range installedPlugins {
		installedMap[plugin.Name] = true
	}

	// Get available plugins
	osName := runtime.GOOS
	arch := runtime.GOARCH
	availablePlugins, err := GetAvailablePlugins("", osName, arch)
	if err != nil {
		LogWarn("Failed to get available plugins: " + err.Error())
	}

	availableMap := make(map[string]PluginInfo)
	for _, plugin := range availablePlugins {
		availableMap[plugin.Name] = plugin
	}

	// Create a comprehensive list
	allPluginNames := make(map[string]bool)
	for _, name := range KnownPlugins {
		allPluginNames[name] = true
	}

	sortedNames := make([]string, 0, len(allPluginNames))
	for name := range allPluginNames {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	fmt.Println("Plugin Status:")
	fmt.Println("==============")
	for _, name := range sortedNames {
		status := "not available"
		version := ""

		if _, isInstalled := installedMap[name]; isInstalled {
			status = "installed"
		} else if available, isAvailable := availableMap[name]; isAvailable {
			status = "available"
			version = " (v" + available.Version + ")"
		}

		fmt.Printf("  %-20s [%s]%s\n", name, status, version)
	}
}

func InstallPlugins(version string, pluginNames []string, pluginDir string, force bool) error {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Create plugin directory if it doesn't exist
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory %s: %w", pluginDir, err)
	}

	// Get available plugins
	availablePlugins, err := GetAvailablePlugins(version, osName, arch)
	if err != nil {
		return fmt.Errorf("failed to get available plugins: %w", err)
	}

	var pluginsToInstall []PluginInfo
	for _, name := range pluginNames {
		found := false
		for _, available := range availablePlugins {
			if available.Name == name {
				pluginsToInstall = append(pluginsToInstall, available)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("plugin %s not available for %s/%s", name, osName, arch)
		}
	}

	// Check if plugins are already installed
	if !force {
		installedPlugins, err := DetectInstalledPlugins(pluginDir)
		if err != nil {
			return fmt.Errorf("failed to detect installed plugins: %w", err)
		}

		installedMap := make(map[string]bool)
		for _, plugin := range installedPlugins {
			installedMap[plugin.Name] = true
		}

		var alreadyInstalled []string
		for _, plugin := range pluginsToInstall {
			if installedMap[plugin.Name] {
				alreadyInstalled = append(alreadyInstalled, plugin.Name)
			}
		}

		if len(alreadyInstalled) > 0 {
			return fmt.Errorf("plugins already installed: %s (use --force to reinstall)",
				strings.Join(alreadyInstalled, ", "))
		}
	}

	// Install plugins
	fmt.Printf("Installing %d plugin(s)...\n", len(pluginsToInstall))
	for _, plugin := range pluginsToInstall {
		err := DownloadAndExtractPlugin(plugin, pluginDir)
		if err != nil {
			return fmt.Errorf("failed to install plugin %s: %w", plugin.Name, err)
		}
		fmt.Printf("✓ Installed plugin: %s\n", plugin.Name)
	}

	fmt.Printf("Successfully installed %d plugin(s)\n", len(pluginsToInstall))
	return nil
}

func UninstallPlugins(pluginNames []string, pluginDir string, force bool) error {
	// Get installed plugins
	installedPlugins, err := DetectInstalledPlugins(pluginDir)
	if err != nil {
		return fmt.Errorf("failed to detect installed plugins: %w", err)
	}

	installedMap := make(map[string]string)
	for _, plugin := range installedPlugins {
		installedMap[plugin.Name] = plugin.Path
	}

	var notInstalled []string
	var toUninstall []string
	var toUninstallNames []string

	for _, name := range pluginNames {
		if path, exists := installedMap[name]; exists {
			toUninstall = append(toUninstall, path)
			toUninstallNames = append(toUninstallNames, name)
		} else {
			notInstalled = append(notInstalled, name)
		}
	}

	if len(notInstalled) > 0 {
		return fmt.Errorf("plugins not installed: %s", strings.Join(notInstalled, ", "))
	}

	if len(toUninstall) == 0 {
		fmt.Println("No plugins to uninstall")
		return nil
	}

	// Confirm uninstallation
	if !force {
		fmt.Printf("The following plugins will be uninstalled: %s\n", strings.Join(toUninstallNames, ", "))
		fmt.Print("Are you sure? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Uninstallation cancelled")
			return nil
		}
	}

	// Remove plugins
	fmt.Printf("Uninstalling %d plugin(s)...\n", len(toUninstall))
	for i, path := range toUninstall {
		err := os.Remove(path)
		if err != nil {
			return fmt.Errorf("failed to remove plugin %s: %w", toUninstallNames[i], err)
		}
		fmt.Printf("✓ Uninstalled plugin: %s\n", toUninstallNames[i])
	}

	fmt.Printf("Successfully uninstalled %d plugin(s)\n", len(toUninstall))
	return nil
}
