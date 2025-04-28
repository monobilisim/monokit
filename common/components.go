package common

import (
	"fmt"
	"runtime" // Added import
	"strings"

	"github.com/spf13/cobra"
)

// Component represents a runnable health check or task within the daemon.
type Component struct {
	Name        string
	EntryPoint  func(cmd *cobra.Command, args []string) // Standard cobra Run function
	Platform    string                                  // "linux", "windows", "darwin", "any" (default: "any")
	CobraCmd    *cobra.Command                          // Pre-configured cobra command if needed
	ExecuteFunc func()                                  // Alternative simpler execution function
	AutoDetect  func() bool                             // Function to detect if component should be enabled
}

// DaemonConfig represents the daemon configuration structure.
type DaemonConfig struct {
	Frequency    int           `yaml:"frequency"`
	Debug        bool          `yaml:"debug"`
	HealthChecks []HealthCheck `yaml:"health_checks"`
}

// HealthCheck represents a health check configuration.
type HealthCheck struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
}

// ComponentRegistry holds all registered components.
var ComponentRegistry = make(map[string]Component)

// RegisterComponent adds a component to the registry.
// Use this in the init() function of each component package.
func RegisterComponent(comp Component) {
	if _, exists := ComponentRegistry[comp.Name]; exists {
		// Handle potential duplicate registration if necessary
		fmt.Printf("Warning: Component %s already registered. Overwriting.\n", comp.Name)
	}
	if comp.Platform == "" {
		comp.Platform = "any" // Default to any platform if not specified
	}
	ComponentRegistry[comp.Name] = comp
	// fmt.Printf("Registered component: %s\n", comp.Name) // Optional: for debugging registration
}

// GetComponent retrieves a component from the registry.
func GetComponent(name string) (Component, bool) {
	comp, exists := ComponentRegistry[name]
	return comp, exists
}

// GetInstalledComponents determines the list of enabled components.
// If /etc/mono/daemon.yml exists, it uses the config file.
// Otherwise, it defaults to osHealth + any auto-detected components.
func GetInstalledComponents() string {
	var enabled []string

	if !ConfExists("daemon") {
		// Config file doesn't exist: Default to osHealth + auto-detected components
		LogDebug("Daemon config file not found. Using default: osHealth + auto-detected.")
		enabled = append(enabled, "osHealth") // Always include osHealth by default

		for name, comp := range ComponentRegistry {
			if name == "osHealth" { // Already added
				continue
			}
			if comp.AutoDetect != nil {
				if comp.AutoDetect() {
					LogDebug(fmt.Sprintf("Component %s auto-detected and enabled (no config).", name))
					enabled = append(enabled, name)
				} else {
					LogDebug(fmt.Sprintf("Component %s failed auto-detection (no config).", name))
				}
			} else {
				// Components without auto-detect are not enabled by default without config
				LogDebug(fmt.Sprintf("Component %s skipped (no config, no auto-detect).", name))
			}
		}
	} else {
		// Config file exists: Load config to check for disabled components
		var config DaemonConfig
		ConfInit("daemon", &config) // Load config using common function
		disabledComponents := make(map[string]bool)
		for _, check := range config.HealthChecks {
			if !check.Enabled {
				disabledComponents[check.Name] = true
				LogDebug(fmt.Sprintf("Component %s explicitly disabled in config.", check.Name))
			}
		}

		LogDebug("Processing components with config file present.")

		// Always consider osHealth unless explicitly disabled
		if _, isDisabled := disabledComponents["osHealth"]; !isDisabled {
			if comp, exists := ComponentRegistry["osHealth"]; exists {
				// Check platform compatibility for osHealth
				if comp.Platform == "any" || comp.Platform == runtime.GOOS {
					LogDebug("Including osHealth (config exists, not disabled, platform matches).")
					enabled = append(enabled, "osHealth")
				} else {
					LogDebug("Skipping osHealth (config exists, not disabled, platform mismatch).")
				}
			} else {
				LogDebug("osHealth component not found in registry, but expected.")
			}
		} else {
			LogDebug("Skipping osHealth (explicitly disabled in config).")
		}

		// Iterate through all other registered components
		for name, comp := range ComponentRegistry {
			if name == "osHealth" {
				continue // Already handled
			}

			// 1. Check if explicitly disabled
			if _, isDisabled := disabledComponents[name]; isDisabled {
				LogDebug(fmt.Sprintf("Component %s skipped (disabled in config).", name))
				continue
			}

			// 2. Check platform compatibility
			if !(comp.Platform == "any" || comp.Platform == runtime.GOOS) {
				LogDebug(fmt.Sprintf("Component %s skipped (platform mismatch).", name))
				continue
			}

			// 3. Perform auto-detection if available
			if comp.AutoDetect != nil {
				LogDebug(fmt.Sprintf("Performing auto-detection for component %s (config exists).", name))
				if comp.AutoDetect() {
					LogDebug(fmt.Sprintf("Component %s included (passed auto-detection with config).", name))
					enabled = append(enabled, name)
				} else {
					LogDebug(fmt.Sprintf("Component %s skipped (failed auto-detection with config).", name))
				}
			} else {
				// No auto-detect: Skip by default, even if config exists (consistent behavior)
				LogDebug(fmt.Sprintf("Component %s skipped (no auto-detect function).", name))
			}
		}
	}

	// Remove duplicates just in case (e.g., if osHealth logic somehow added it twice)
	uniqueEnabled := make(map[string]bool)
	finalEnabled := []string{}
	for _, name := range enabled {
		if !uniqueEnabled[name] {
			uniqueEnabled[name] = true
			finalEnabled = append(finalEnabled, name)
		}
	}

	if len(finalEnabled) > 0 {
		return strings.Join(finalEnabled, "::")
	}

	// Fallback if absolutely nothing is enabled (should ideally not happen with osHealth default)
	LogDebug("No components enabled after processing. Returning empty list.")
	return ""
}

// IsComponentEnabled checks if a specific component is listed in the installed components.
func IsComponentEnabled(name string) bool {
	installed := strings.Split(GetInstalledComponents(), "::")
	for _, comp := range installed {
		if comp == name {
			return true
		}
	}
	return false
}
