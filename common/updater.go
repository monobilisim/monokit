package common

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// PluginInfo represents information about a plugin
type PluginInfo struct {
	Name        string
	Version     string
	Path        string
	IsInstalled bool
	URL         string
}

var UpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Monokit",
	Run: func(cmd *cobra.Command, args []string) {
		specificVersion, _ := cmd.Flags().GetString("version")
		force, _ := cmd.Flags().GetBool("force")
		updatePlugins, _ := cmd.Flags().GetBool("update-plugins")
		specificPlugins, _ := cmd.Flags().GetStringSlice("plugins")
		pluginDir, _ := cmd.Flags().GetString("plugin-dir")

		Update(specificVersion, force, updatePlugins, specificPlugins, pluginDir)
	},
}

func init() {
	UpdateCmd.Flags().Bool("update-plugins", true, "Update plugins along with main binary")
	UpdateCmd.Flags().StringSlice("plugins", []string{}, "Specific plugins to update (comma-separated)")
	UpdateCmd.Flags().String("plugin-dir", "/var/lib/monokit/plugins", "Plugin directory path")
}

// DetectInstalledPlugins scans the plugin directory and returns information about installed plugins
func DetectInstalledPlugins(dir string) ([]PluginInfo, error) {
	var plugins []PluginInfo

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		LogDebug("Plugin directory does not exist: " + dir)
		return plugins, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip non-executable files and backup files
		if strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, ".tmp") {
			continue
		}

		pluginPath := filepath.Join(dir, name)

		// Check if file is executable
		info, err := entry.Info()
		if err != nil {
			LogWarn("Failed to get file info for " + name + ": " + err.Error())
			continue
		}

		if info.Mode()&0111 == 0 {
			continue // Not executable
		}

		plugin := PluginInfo{
			Name:        name,
			Path:        pluginPath,
			IsInstalled: true,
		}

		plugins = append(plugins, plugin)
		LogDebug("Detected installed plugin: " + name)
	}

	return plugins, nil
}

// GetAvailablePlugins queries GitHub API to get available plugin downloads for a specific version
func GetAvailablePlugins(version, os, arch string) ([]PluginInfo, error) {
	var plugins []PluginInfo
	var url string

	if version == "" {
		url = "https://api.github.com/repos/monobilisim/monokit/releases/latest"
	} else {
		url = fmt.Sprintf("https://api.github.com/repos/monobilisim/monokit/releases/tags/v%s", version)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get release information: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		return nil, fmt.Errorf("failed to decode release information: %w", err)
	}

	releaseVersion := release["tag_name"].(string)
	releaseVersion = strings.TrimPrefix(releaseVersion, "v")

	assets, ok := release["assets"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no assets found in release")
	}

	// Known plugin names based on the codebase structure
	knownPlugins := []string{
		"k8sHealth", "osHealth", "mysqlHealth", "pgsqlHealth", "redisHealth",
		"zimbraHealth", "traefikHealth", "rmqHealth", "pritunlHealth",
		"wppconnectHealth", "pmgHealth", "esHealth", "postalHealth",
	}

	for _, asset := range assets {
		assetMap := asset.(map[string]interface{})
		assetName := assetMap["name"].(string)
		downloadURL := assetMap["browser_download_url"].(string)

		// Check if this is a plugin asset for our OS/arch
		if !strings.Contains(assetName, os) || !strings.Contains(assetName, arch) {
			continue
		}

		// Parse plugin name from asset name
		// Expected format: monokit_{pluginName}_{version}_{os}_{arch}.tar.gz
		for _, pluginName := range knownPlugins {
			expectedPattern := fmt.Sprintf("monokit_%s_%s_%s_%s.tar.gz", pluginName, releaseVersion, os, arch)
			if assetName == expectedPattern {
				plugin := PluginInfo{
					Name:        pluginName,
					Version:     releaseVersion,
					IsInstalled: false,
					URL:         downloadURL,
				}
				plugins = append(plugins, plugin)
				LogDebug("Found available plugin: " + pluginName + " v" + releaseVersion)
				break
			}
		}
	}

	return plugins, nil
}

// DownloadAndExtractPlugin downloads and extracts a single plugin
func DownloadAndExtractPlugin(plugin PluginInfo, pluginDir string) error {
	LogDebug("Downloading plugin: " + plugin.Name + " from " + plugin.URL)

	resp, err := http.Get(plugin.URL)
	if err != nil {
		return fmt.Errorf("failed to download plugin %s: %w", plugin.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to download plugin %s: HTTP %d", plugin.Name, resp.StatusCode)
	}

	// Extract the plugin
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for plugin %s: %w", plugin.Name, err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}

		if hdr.Name == plugin.Name {
			tempPath := filepath.Join(TmpDir, plugin.Name+".tmp")
			finalPath := filepath.Join(pluginDir, plugin.Name)
			backupPath := finalPath + ".bak"

			// Create temporary file
			f, err := os.Create(tempPath)
			if err != nil {
				return fmt.Errorf("failed to create temporary file for plugin %s: %w", plugin.Name, err)
			}

			_, err = f.ReadFrom(tr)
			f.Close()
			if err != nil {
				os.Remove(tempPath)
				return fmt.Errorf("failed to write plugin %s: %w", plugin.Name, err)
			}

			// Set executable permissions
			err = os.Chmod(tempPath, 0755)
			if err != nil {
				os.Remove(tempPath)
				return fmt.Errorf("failed to set permissions for plugin %s: %w", plugin.Name, err)
			}

			// Backup existing plugin if it exists
			if FileExists(finalPath) {
				err = os.Rename(finalPath, backupPath)
				if err != nil {
					os.Remove(tempPath)
					return fmt.Errorf("failed to backup existing plugin %s: %w", plugin.Name, err)
				}
			}

			// Move new plugin into place
			err = os.Rename(tempPath, finalPath)
			if err != nil {
				// Try to restore backup
				if FileExists(backupPath) {
					os.Rename(backupPath, finalPath)
				}
				os.Remove(tempPath)
				return fmt.Errorf("failed to install plugin %s: %w", plugin.Name, err)
			}

			// Remove backup on success
			if FileExists(backupPath) {
				os.Remove(backupPath)
			}

			LogDebug("Successfully updated plugin: " + plugin.Name)
			return nil
		}
	}

	return fmt.Errorf("plugin binary %s not found in archive", plugin.Name)
}

// UpdatePlugins updates all or specific plugins
func UpdatePlugins(version string, specificPlugins []string, pluginDir string, force bool) error {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Create plugin directory if it doesn't exist
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory %s: %w", pluginDir, err)
	}

	// Get installed plugins
	installedPlugins, err := DetectInstalledPlugins(pluginDir)
	if err != nil {
		return fmt.Errorf("failed to detect installed plugins: %w", err)
	}

	// Get available plugins
	availablePlugins, err := GetAvailablePlugins(version, osName, arch)
	if err != nil {
		return fmt.Errorf("failed to get available plugins: %w", err)
	}

	if len(availablePlugins) == 0 {
		fmt.Println("No plugins available for your OS and architecture")
		return nil
	}

	// Determine which plugins to update
	var pluginsToUpdate []PluginInfo

	if len(specificPlugins) > 0 {
		// Update only specified plugins
		for _, specifiedPlugin := range specificPlugins {
			found := false
			for _, available := range availablePlugins {
				if available.Name == specifiedPlugin {
					pluginsToUpdate = append(pluginsToUpdate, available)
					found = true
					break
				}
			}
			if !found {
				LogWarn("Specified plugin not available: " + specifiedPlugin)
			}
		}
	} else {
		// Update all available plugins, but only if they're already installed or force is enabled
		for _, available := range availablePlugins {
			shouldUpdate := force

			if !shouldUpdate {
				// Check if plugin is already installed
				for _, installed := range installedPlugins {
					if installed.Name == available.Name {
						shouldUpdate = true
						break
					}
				}
			}

			if shouldUpdate {
				pluginsToUpdate = append(pluginsToUpdate, available)
			}
		}
	}

	if len(pluginsToUpdate) == 0 {
		fmt.Println("No plugins to update")
		return nil
	}

	// Update plugins in parallel
	fmt.Printf("Updating %d plugin(s)...\n", len(pluginsToUpdate))

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	successCount := 0

	for _, plugin := range pluginsToUpdate {
		wg.Add(1)
		go func(p PluginInfo) {
			defer wg.Done()

			err := DownloadAndExtractPlugin(p, pluginDir)

			mu.Lock()
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to update plugin %s: %w", p.Name, err))
			} else {
				successCount++
				fmt.Printf("✓ Updated plugin: %s\n", p.Name)
			}
			mu.Unlock()
		}(plugin)
	}

	wg.Wait()

	if len(errors) > 0 {
		fmt.Printf("Plugin update completed with %d successes and %d errors:\n", successCount, len(errors))
		for _, err := range errors {
			LogError(err.Error())
		}
		return fmt.Errorf("some plugin updates failed")
	}

	fmt.Printf("Successfully updated %d plugin(s)\n", successCount)
	return nil
}

func DownloadAndExtract(url string) {
	MonokitPath, err := os.Executable()

	if err != nil {
		LogError("Couldn't get executable path: " + err.Error())
	}

	// Download the release
	resp, err := http.Get(url)
	if err != nil {
		LogError("Couldn't download the release: " + err.Error())
	}
	defer resp.Body.Close()

	// Extract the release
	gzr, err := gzip.NewReader(resp.Body)

	if err != nil {
		LogError("Couldn't extract the release: " + err.Error())
	}

	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}

		if hdr.Name == "monokit" {
			f, err := os.Create(TmpDir + "monokit")
			if err != nil {
				LogError("Couldn't create monokit binary: " + err.Error())
			}
			defer f.Close()

			_, err = f.ReadFrom(tr)
			if err != nil {
				LogError("Couldn't write monokit binary: " + err.Error())
			}
		}
	}

	os.Rename(MonokitPath, MonokitPath+".bak")

	// Move monokit binary to the correct path
	os.Rename(TmpDir+"monokit", MonokitPath)
	os.Chmod(MonokitPath, 0755)
}

func Update(specificVersion string, force bool, updatePlugins bool, specificPlugins []string, pluginDir string) {
	var url string
	var version string
	osName := runtime.GOOS
	arch := runtime.GOARCH

	if specificVersion != "" {
		version = specificVersion
		url = "https://github.com/monobilisim/monokit/releases/download/v" + specificVersion + "/monokit_" + specificVersion + "_" + osName + "_" + arch + ".tar.gz"
	} else {
		// Get latest release
		url = "https://api.github.com/repos/monobilisim/monokit/releases/latest"
		resp, err := http.Get(url)
		if err != nil {
			LogError("Couldn't get latest release: " + err.Error())
		}
		defer resp.Body.Close()

		var release map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&release)
		if err != nil {
			LogError("Couldn't decode latest release: " + err.Error())
		}

		assets := release["assets"].([]interface{})
		for _, asset := range assets {
			assetMap := asset.(map[string]interface{})
			if strings.Contains(assetMap["name"].(string), osName) && strings.Contains(assetMap["name"].(string), arch) {
				url = assetMap["browser_download_url"].(string)
				version = release["tag_name"].(string)
				version = strings.TrimPrefix(version, "v")
				break
			}
		}
	}

	if url == "" {
		fmt.Println("No release found for your OS and architecture")
		return
	}

	if (version == MonokitVersion || MonokitVersion == "devel") && !force {

		if MonokitVersion == "devel" {
			fmt.Println("Monokit is a development version, run with --force to update anyway")
		} else {
			fmt.Println("Monokit is already up to date, run with --force to update anyway")
		}

		return
	}

	fmt.Println("Current Monokit version:", MonokitVersion)

	if MonokitVersion != "devel" {
		monokitVersionSplit := strings.Split(MonokitVersion, ".")
		versionSplit := strings.Split(version, ".")

		if strings.Contains(versionSplit[0], "v") {
			versionSplit[0] = strings.TrimPrefix(versionSplit[0], "v")
		}

		if monokitVersionSplit[0] != versionSplit[0] {
			if !force {
				fmt.Println("A new major version is available. This might bring breaking changes. Monokit will attempt to migrate to the new vesrion. You can run with --force to update")
				return
			}
		}
	}

	fmt.Println("Downloading Monokit version", version)
	DownloadAndExtract(url)

	fmt.Println("Monokit updated to version", version)

	// Update plugins if requested
	if updatePlugins {
		fmt.Println("Updating plugins...")
		if err := UpdatePlugins(version, specificPlugins, pluginDir, force); err != nil {
			LogError("Plugin update failed: " + err.Error())
			fmt.Println("Main binary updated successfully, but plugin updates failed")
		}
	}
}
