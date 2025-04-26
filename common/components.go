package common

import (
	"fmt"
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
		// Config file exists: Load and process it
		var config DaemonConfig
		ConfInit("daemon", &config) // Load config using common function

		LogDebug("Processing daemon config file.")
		for _, check := range config.HealthChecks {
			if check.Enabled {
				if comp, exists := ComponentRegistry[check.Name]; exists && comp.AutoDetect != nil {
					// Run auto-detection for components that support it and are enabled
					if comp.AutoDetect() {
						LogDebug(fmt.Sprintf("Component %s auto-detected and enabled (config).", check.Name))
						enabled = append(enabled, check.Name)
					} else {
						LogDebug(fmt.Sprintf("Component %s configured but failed auto-detection (config).", check.Name))
					}
				} else if exists {
					// For components without auto-detect, use config value (if enabled)
					LogDebug(fmt.Sprintf("Component %s enabled via config (no auto-detect).", check.Name))
					enabled = append(enabled, check.Name)
				} else {
					LogDebug(fmt.Sprintf("Component %s configured but not found in registry.", check.Name))
				}
			} else {
				LogDebug(fmt.Sprintf("Component %s disabled in config.", check.Name))
			}
		}
		// Ensure osHealth is always included if config exists but doesn't mention it or disables it
		osHealthFound := false
		for _, name := range enabled {
			if name == "osHealth" {
				osHealthFound = true
				break
			}
		}
		if !osHealthFound {
			LogDebug("osHealth not found in enabled list from config, adding it by default.")
			// Check if osHealth is explicitly disabled in config
			osHealthDisabled := false
			for _, check := range config.HealthChecks {
				if check.Name == "osHealth" && !check.Enabled {
					osHealthDisabled = true
					LogDebug("osHealth is explicitly disabled in config.")
					break
				}
			}
			if !osHealthDisabled {
				enabled = append([]string{"osHealth"}, enabled...) // Prepend osHealth
			}
		}
	}

	if len(enabled) > 0 {
		return strings.Join(enabled, "::")
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
