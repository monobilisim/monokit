package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec" // Added for running commands
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/health/plugin" // Added for plugin cleanup

	// Removed direct component imports like k8sHealth, osHealth, etc.
	// They will be accessed via the registry.
	"github.com/spf13/cobra"
)

const lastUpdateCheckFile = "/tmp/monokit_last_update_check" // User requested /tmp

type Daemon struct {
	Frequency      int  // Frequency to run health checks
	Debug          bool // Debug mode
	MonokitUpgrade bool `mapstructure:"monokit_upgrade"` // Control daily monokit update/version check
}

var DaemonConfig Daemon

func Main(cmd *cobra.Command, args []string) {
	version := "1.0.0" // Consider fetching this dynamically if possible

	// --- Get flags before Init ---
	runOnce, _ := cmd.Flags().GetBool("once")
	listComponents, _ := cmd.Flags().GetBool("list-components")

	// --- Single-instance check ---
	if common.ProcGrep("monokit daemon", true) {
		fmt.Println("Monokit daemon is already running, exiting...")
		os.Exit(1)
	}
	// --- Set common flag BEFORE Init ---
	common.IgnoreLockfile = true

	// --- Init common (handles lockfile based on common.IgnoreLockfile) ---
	common.Init()

	if common.ConfExists("daemon") {
		common.ConfInit("daemon", &DaemonConfig)
	} else {
		DaemonConfig.Frequency = 60
		DaemonConfig.MonokitUpgrade = true // default: perform upgrade/version check daily
	}

	fmt.Println("Monokit daemon - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	// Log monokit upgrade check status at startup
	if !DaemonConfig.MonokitUpgrade {
		fmt.Println("Daily monokit update check disabled via monokit_upgrade=false")
	}

	// Flags were already parsed before common.Init()

	// Lockfile handling is now done within common.Init() based on common.IgnoreLockfile

	if runOnce {
		fmt.Println("Running once")
		RunAll() // Pass the flag value down for component execution logic
		os.Exit(0)
	}

	if listComponents {
		fmt.Print(common.GetInstalledComponents()) // Note: GetInstalledComponents might need adjustment depending on its intended use now
		// No need to remove lockfile here if it wasn't created (due to --ignore-lockfile)
		os.Exit(0)
	}

	// Main daemon loop
	ticker := time.NewTicker(time.Duration(DaemonConfig.Frequency) * time.Second)
	defer ticker.Stop()

	for {
		RunAll()   // Pass lockfile flag down
		<-ticker.C // Wait for the next tick
	}
}

// shouldRunDailyUpdate checks if the update should run based on the timestamp file.
func shouldRunDailyUpdate(filePath string) bool {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		// File doesn't exist or error reading, assume update should run
		fmt.Printf("Update check: No previous timestamp found or error reading %s: %v. Running update check.\n", filePath, err)
		return true
	}

	lastCheckUnix, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		fmt.Printf("Update check: Error parsing timestamp from %s: %v. Running update check.\n", filePath, err)
		return true // Error parsing, run the update
	}

	lastCheckTime := time.Unix(lastCheckUnix, 0)
	if time.Since(lastCheckTime) >= 24*time.Hour {
		fmt.Printf("Update check: Last check was at %s. More than 24 hours ago. Running update check.\n", lastCheckTime.Format(time.RFC3339))
		return true
	}

	fmt.Printf("Update check: Last check was at %s. Less than 24 hours ago. Skipping update check.\n", lastCheckTime.Format(time.RFC3339))
	return false
}

// recordUpdateCheck writes the current timestamp to the file.
func recordUpdateCheck(filePath string) {
	nowUnix := time.Now().Unix()
	content := []byte(strconv.FormatInt(nowUnix, 10))
	err := ioutil.WriteFile(filePath, content, 0644)
	if err != nil {
		fmt.Printf("Error writing update timestamp to %s: %v\n", filePath, err)
	} else {
		fmt.Printf("Update check: Recorded current timestamp %d to %s\n", nowUnix, filePath)
	}
}

// RunAll executes all registered and enabled components.
// It now accepts the lockfile flag to pass down to sudo calls.
func RunAll() {
	// Daily monokit update/version check controlled by config flag
	if DaemonConfig.MonokitUpgrade {
		if shouldRunDailyUpdate(lastUpdateCheckFile) {
			fmt.Println("Running daily monokit update check...")
			common.Update("", false, true, []string{}, "/var/lib/monokit/plugins") // Check for monokit updates
			recordUpdateCheck(lastUpdateCheckFile)
		}
	}

	// --- Run versionCheck unconditionally ---
	fmt.Println("Running version checks...")
	if vcComp, vcExists := common.GetComponent("versionCheck"); vcExists {
		executeComponent(vcComp) // Use helper function
	} else {
		fmt.Println("Warning: versionCheck component not found in registry.")
	}
	fmt.Println("Finished version checks.")

	// upCheck now participates via component registry like others (auto-detected)

	// --- Get the list of *other* components to run from the centralized function ---
	componentsToRunStr := common.GetInstalledComponents() // This might need adjustment if versionCheck is included here
	if componentsToRunStr == "" {
		fmt.Println("No other components determined to run in this cycle.")
		fmt.Println("Finished component checks for this cycle.")

		// Clean up plugins at the end of the cycle when no components need to run
		fmt.Println("Cleaning up plugins at the end of daemon cycle...")
		plugin.CleanupAll()
		return // Nothing to do
	}
	componentsToRun := strings.Split(componentsToRunStr, "::")
	fmt.Printf("Components determined to run: %v\n", componentsToRun)

	// --- Iterate through the determined components and execute them ---
	for _, compName := range componentsToRun {
		if comp, exists := common.GetComponent(compName); exists {
			// Platform/disabled checks are already handled by GetInstalledComponents
			executeComponent(comp) // Use helper function
		} else {
			// This should ideally not happen if GetInstalledComponents is correct
			fmt.Printf("Warning: Component %s was listed to run but not found in registry.\n", compName)
		}
	} // End of component loop

	fmt.Println("Finished component checks for this cycle.")

	// CRITICAL: Clean up plugins at the end of each daemon cycle
	// This prevents plugin processes from accumulating after all components have finished
	fmt.Println("Cleaning up plugins at the end of daemon cycle...")
	plugin.CleanupAll()
}

// executeComponent handles the logic for running a single component,
// including direct execution, sudo execution, and platform checks.
func executeComponent(comp common.Component) {
	executablePath, execErr := os.Executable()
	if execErr != nil {
		fmt.Printf("Error getting executable path: %v. Cannot run components as different users.\n", execErr)
		// Fallback or skip? For now, we'll try direct execution if possible,
		// but warn if RunAsUser was intended.
		if comp.RunAsUser != "" {
			fmt.Printf("Warning: Cannot run component %s as user %s due to executable path error. Attempting direct execution.\n", comp.Name, comp.RunAsUser)
		}
	}

	// Determine if sudo should be used
	useSudo := comp.RunAsUser != "" && execErr == nil && comp.Platform == "linux"

	if useSudo {
		fmt.Printf("Running component %s as user %s", comp.Name, comp.RunAsUser)
		args := []string{"-u", comp.RunAsUser}
		// Preserve the HOSTNAME environment variable if it exists, might help sudo
		// Although the core issue is likely /etc/hosts or DNS
		if hostname := os.Getenv("HOSTNAME"); hostname != "" {
			args = append(args, fmt.Sprintf("HOSTNAME=%s", hostname))
		}
		args = append(args, executablePath, comp.Name)
		// Add --ignore-lockfile flag
		args = append(args, "--ignore-lockfile")
		// Add --cleanup-plugins flag to ensure spawned processes clean up their plugins
		args = append(args, "--cleanup-plugins")
		fmt.Printf(" with --ignore-lockfile --cleanup-plugins")
		fmt.Println("...")

		cmd := exec.Command("sudo", args...)
		cmd.Stdout = os.Stdout // Pipe component output to daemon output
		cmd.Stderr = os.Stderr // Pipe component error to daemon error
		err := cmd.Run()
		if err != nil {
			fmt.Printf("Error running component %s as user %s: %v\n", comp.Name, comp.RunAsUser, err)
			// Log error or send alarm?
		} else {
			fmt.Printf("Finished running component %s as user %s.\n", comp.Name, comp.RunAsUser)
		}
	} else {
		// Conditions for not using sudo:
		// 1. RunAsUser is not set.
		// 2. Error getting executable path (execErr != nil).
		// 3. Not on Linux (comp.Platform != "linux").

		if comp.RunAsUser != "" && !useSudo { // Explain why sudo wasn't used if it was intended
			if execErr != nil {
				fmt.Printf("Skipping sudo for component %s: Cannot determine executable path to run as user %s. Running directly.\n", comp.Name, comp.RunAsUser)
			} else if comp.Platform != "linux" {
				fmt.Printf("Skipping sudo for component %s: Running as different user (%s) is only supported on Linux. Running directly.\n", comp.Name, comp.RunAsUser)
			}
		}

		// Run directly
		fmt.Printf("Running component: %s (directly)\n", comp.Name)
		if comp.EntryPoint != nil {
			// Need to create a temporary cobra command to execute
			// Pass the --ignore-lockfile flag to the component's execution context if needed
			originalOsArgs := os.Args // Store original args
			tempCmd := &cobra.Command{
				Use:                comp.Name,
				Run:                comp.EntryPoint,
				DisableFlagParsing: false, // Allow parsing flags like --ignore-lockfile
				Args:               cobra.ArbitraryArgs,
			}
			// Let Cobra ignore unknown flags (e.g., --ignore-lockfile) if the component
			// itself doesn't declare them, preventing duplicate-definition panics
			// and "unknown flag" errors.
			tempCmd.FParseErrWhitelist = cobra.FParseErrWhitelist{UnknownFlags: true}
			// Manually set the arguments for the component's command execution
			os.Args = []string{executablePath, comp.Name, "--ignore-lockfile", "--cleanup-plugins"}

			// ExecuteC captures errors, Execute runs and panics on error
			_, err := tempCmd.ExecuteC() // Use ExecuteC to handle errors gracefully
			if err != nil {
				fmt.Printf("Error running component %s directly: %v\n", comp.Name, err)
			}
			os.Args = originalOsArgs // Restore original os.Args
		} else if comp.ExecuteFunc != nil {
			// How to pass --ignore-lockfile to ExecuteFunc? It needs to be designed to accept it.
			// For now, we assume ExecuteFunc handles its context or doesn't need the flag.
			// If ExecuteFunc needs the flag, the component definition should change.
			fmt.Printf("Note: Cannot automatically pass --ignore-lockfile to ExecuteFunc for %s.\n", comp.Name)
			comp.ExecuteFunc() // Assuming ExecuteFunc handles its own errors or panics
		} else {
			fmt.Printf("Warning: Component %s determined to run but has no execution method defined.\n", comp.Name)
		}
		fmt.Printf("Finished running component %s (directly).\n", comp.Name)
	}
}
