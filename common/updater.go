package common

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
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

// exeName returns the file name a binary is released and installed under on
// the running platform. Release archives are built by goreleaser, which
// appends .exe to Windows builds, so both the archive entry and the file on
// disk carry the suffix there.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
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
	UpdateCmd.Flags().String("plugin-dir", DefaultPluginDir, "Plugin directory path")
}

// DetectInstalledPlugins scans the plugin directory and returns information about installed plugins
func DetectInstalledPlugins(dir string) ([]PluginInfo, error) {
	var plugins []PluginInfo

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Debug().Str("dir", dir).Msg("Plugin directory does not exist")
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
			log.Warn().Str("name", name).Err(err).Msg("Failed to get file info")
			continue
		}

		if runtime.GOOS == "windows" {
			// Windows carries no executable bit, so os.Stat reports plain
			// files as non-executable and every plugin would be skipped.
			// Plugins ship as .exe there instead.
			if !strings.HasSuffix(name, ".exe") {
				continue
			}
		} else if info.Mode()&0111 == 0 {
			continue // Not executable
		}

		plugin := PluginInfo{
			// Name is the plugin name as the release names it, without the
			// platform's executable suffix, so it compares against KnownPlugins.
			Name:        strings.TrimSuffix(name, ".exe"),
			Path:        pluginPath,
			IsInstalled: true,
		}

		plugins = append(plugins, plugin)
		log.Debug().Str("name", name).Msg("Detected installed plugin")
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
		for _, pluginName := range KnownPlugins {
			expectedPattern := fmt.Sprintf("monokit_%s_%s_%s_%s.tar.gz", pluginName, releaseVersion, os, arch)
			if assetName == expectedPattern {
				plugin := PluginInfo{
					Name:        pluginName,
					Version:     releaseVersion,
					IsInstalled: false,
					URL:         downloadURL,
				}
				plugins = append(plugins, plugin)
				log.Debug().Str("pluginName", pluginName).Str("releaseVersion", releaseVersion).Msg("Found available plugin")
				break
			}
		}
	}

	return plugins, nil
}

// DownloadAndExtractPlugin downloads and extracts a single plugin
func DownloadAndExtractPlugin(plugin PluginInfo, pluginDir string) error {
	log.Debug().Str("pluginName", plugin.Name).Str("pluginURL", plugin.URL).Msg("Downloading plugin")

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

	binaryName := exeName(plugin.Name)

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}

		if hdr.Name == binaryName {
			// Stage in the same directory as the destination so os.Rename
			// is an atomic same-filesystem move and never crosses device boundaries.
			tempPath := filepath.Join(pluginDir, binaryName+".tmp")
			finalPath := filepath.Join(pluginDir, binaryName)
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

			log.Debug().Str("pluginName", plugin.Name).Msg("Successfully updated plugin")
			return nil
		}
	}

	return fmt.Errorf("plugin binary %s not found in archive", binaryName)
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
				log.Warn().Str("specifiedPlugin", specifiedPlugin).Msg("Specified plugin not available")
			}
		}
	} else {
		// Update all available plugins, but only if they're already installed
		for _, available := range availablePlugins {
			shouldUpdate := false

			// Check if plugin is already installed
			for _, installed := range installedPlugins {
				if installed.Name == available.Name {
					shouldUpdate = true
					break
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
			log.Error().Msg(err.Error())
		}
		return fmt.Errorf("some plugin updates failed")
	}

	fmt.Printf("Successfully updated %d plugin(s)\n", successCount)
	return nil
}

// DownloadAndExtract replaces the running monokit binary with the one in the
// release archive at url. The current binary is only moved aside once the new
// one is staged on disk, so a release that does not carry a binary for this
// platform leaves the installation untouched.
func DownloadAndExtract(url string) error {
	monokitPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("couldn't get executable path: %w", err)
	}

	return downloadAndExtractTo(url, monokitPath)
}

// downloadAndExtractTo carries out the update against an explicit destination
// path, so the install logic can be exercised without targeting the binary
// that is actually running.
func downloadAndExtractTo(url, monokitPath string) error {
	// Download the release
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("couldn't download the release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("couldn't download the release: HTTP %d", resp.StatusCode)
	}

	// Extract the release
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("couldn't extract the release: %w", err)
	}
	defer gzr.Close()

	binaryName := exeName("monokit")
	stagedPath := filepath.Join(TmpDir, binaryName)
	staged := false

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("couldn't read the release archive: %w", err)
		}

		if filepath.Base(hdr.Name) != binaryName {
			continue
		}

		f, err := os.Create(stagedPath)
		if err != nil {
			return fmt.Errorf("couldn't create monokit binary: %w", err)
		}

		_, err = f.ReadFrom(tr)
		f.Close()
		if err != nil {
			os.Remove(stagedPath)
			return fmt.Errorf("couldn't write monokit binary: %w", err)
		}

		staged = true
		break
	}

	if !staged {
		return fmt.Errorf("no %s binary found in the release archive", binaryName)
	}

	if err := os.Chmod(stagedPath, 0755); err != nil {
		os.Remove(stagedPath)
		return fmt.Errorf("couldn't set permissions on the new monokit binary: %w", err)
	}

	// Windows refuses to overwrite a running executable but does allow
	// renaming it, so the live binary is moved aside instead of replaced.
	backupPath, err := backupCurrentBinary(monokitPath)
	if err != nil {
		os.Remove(stagedPath)
		return err
	}

	if err := MoveFile(stagedPath, monokitPath); err != nil {
		if restoreErr := MoveFile(backupPath, monokitPath); restoreErr != nil {
			return fmt.Errorf("couldn't install the new monokit binary (%w) and couldn't restore %s from %s: %w", err, monokitPath, backupPath, restoreErr)
		}
		os.Remove(stagedPath)
		return fmt.Errorf("couldn't install the new monokit binary, kept the current one: %w", err)
	}

	if err := os.Chmod(monokitPath, 0755); err != nil {
		log.Warn().Err(err).Str("path", monokitPath).Msg("Couldn't set permissions on the installed monokit binary")
	}

	// On Windows the backup is the image of this very process and stays
	// locked until it exits, so a leftover is expected rather than an error.
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		log.Debug().Err(err).Str("path", backupPath).Msg("Couldn't remove the backup of the previous monokit binary")
	}

	return nil
}

// backupCurrentBinary moves the running binary out of the way and returns the
// path it was moved to. A backup left behind by an earlier update is reused
// when it can be removed; when it cannot - on Windows it is the image of a
// still-running process - a unique name is used so the update can proceed.
func backupCurrentBinary(monokitPath string) (string, error) {
	backupPath := monokitPath + ".bak"

	if FileExists(backupPath) {
		if err := os.Remove(backupPath); err != nil {
			backupPath = monokitPath + ".bak." + strconv.FormatInt(time.Now().Unix(), 10)
			log.Debug().Err(err).Str("path", backupPath).Msg("Previous backup is still in use, backing up under a unique name")
		}
	}

	if err := MoveFile(monokitPath, backupPath); err != nil {
		return "", fmt.Errorf("couldn't move the current monokit binary aside: %w", err)
	}

	return backupPath, nil
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
			log.Error().Err(err).Msg("Couldn't get latest release")
			url = ""
		} else {
			defer resp.Body.Close()

			var release map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&release)
			if err != nil {
				log.Error().Err(err).Msg("Couldn't decode latest release")
				url = ""
			} else {
				url = ""
				assets, _ := release["assets"].([]interface{})
				for _, asset := range assets {
					assetMap, ok := asset.(map[string]interface{})
					if !ok {
						continue
					}
					name, _ := assetMap["name"].(string)
					if strings.Contains(name, osName) && strings.Contains(name, arch) {
						url, _ = assetMap["browser_download_url"].(string)
						version, _ = release["tag_name"].(string)
						version = strings.TrimPrefix(version, "v")
						break
					}
				}
			}
		}
	}

	log.Debug().Str("url", url).Str("version", version).Msg("Update URL and version")

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
	if err := DownloadAndExtract(url); err != nil {
		log.Error().Err(err).Str("version", version).Msg("Monokit update failed")
		fmt.Println("Monokit update failed:", err)
		return
	}

	fmt.Println("Monokit updated to version", version)

	// Replacing the binary does not affect an already running process, so a
	// daemon that is up keeps serving the old version until it is restarted.
	fmt.Println("Restart the monokit daemon for version " + version + " to take effect there")

	// Update plugins if requested
	if updatePlugins {
		fmt.Println("Updating plugins...")
		if err := UpdatePlugins(version, specificPlugins, pluginDir, force); err != nil {
			log.Error().Err(err).Msg("Plugin update failed")
			fmt.Println("Main binary updated successfully, but plugin updates failed")
		}
	}
}
