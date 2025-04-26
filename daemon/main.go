package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	// Removed direct component imports like k8sHealth, osHealth, etc.
	// They will be accessed via the registry.
	"github.com/spf13/cobra"
)

const lastUpdateCheckFile = "/tmp/monokit_last_update_check" // User requested /tmp

type Daemon struct {
	Frequency int  // Frequency to run health checks
	Debug     bool // Debug mode
}

var DaemonConfig Daemon

func Main(cmd *cobra.Command, args []string) {
	version := "1.0.0"
	common.Init()

	if common.ConfExists("daemon") {
		common.ConfInit("daemon", &DaemonConfig)
	} else {
		DaemonConfig.Frequency = 60
	}

	fmt.Println("Monokit daemon - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	runOnce, _ := cmd.Flags().GetBool("once")
	listComponents, _ := cmd.Flags().GetBool("list-components")

	if runOnce {
		fmt.Println("Running once")
		RunAll()
		os.Exit(0)
	}

	if listComponents {
		fmt.Print(common.GetInstalledComponents())
		common.RemoveLockfile()
		os.Exit(0)
	}

	for {
		RunAll()
		time.Sleep(time.Duration(DaemonConfig.Frequency) * time.Second)
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

func RunAll() {
	// Check and run daily update if needed
	if shouldRunDailyUpdate(lastUpdateCheckFile) {
		fmt.Println("Running daily monokit update check...")
		common.Update("", false) // Check for monokit updates
		recordUpdateCheck(lastUpdateCheckFile)
	}

	// Run OS Health check always (assuming osHealth is registered)
	if comp, exists := common.GetComponent("osHealth"); exists {
		fmt.Println("Running component: osHealth")
		if comp.EntryPoint != nil {
			// If osHealth uses a cobra command structure
			cmd := &cobra.Command{
				Use:                comp.Name,
				Run:                comp.EntryPoint,
				DisableFlagParsing: true, // Avoid flag conflicts within the daemon loop
			}
			cmd.ExecuteC() // Use ExecuteC to avoid os.Exit on error
		} else if comp.ExecuteFunc != nil {
			// If osHealth uses a simple execute function
			comp.ExecuteFunc()
		}
	} else {
		fmt.Println("Warning: osHealth component not found in registry, but expected to run.")
	}

	// Run checks based on installed components from config
	installed := strings.Split(common.GetInstalledComponents(), "::")
	fmt.Printf("Installed components to check: %v\n", installed) // Debugging output

	for _, compName := range installed {
		if compName == "osHealth" {
			continue // Already ran osHealth
		}

		if comp, exists := common.GetComponent(compName); exists {
			// Check platform compatibility
			if comp.Platform == "any" || comp.Platform == runtime.GOOS {
				fmt.Printf("Running component: %s\n", compName)
				if comp.EntryPoint != nil {
					// Execute cobra command based component
					cmd := &cobra.Command{
						Use:                comp.Name,
						Run:                comp.EntryPoint,
						DisableFlagParsing: true,
					}
					// Pass through relevant flags if needed, or handle config within the component
					cmd.ExecuteC()
				} else if comp.ExecuteFunc != nil {
					// Execute simple function based component
					comp.ExecuteFunc()
				} else {
					fmt.Printf("Warning: Component %s has no execution method defined.\n", compName)
				}
			} else {
				// fmt.Printf("Skipping component %s: Platform mismatch (requires %s, running on %s)\n", compName, comp.Platform, runtime.GOOS) // Optional debug
			}
		} else {
			fmt.Printf("Warning: Component %s listed as installed but not found in registry.\n", compName)
		}
	}
}
