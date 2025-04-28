package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
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
		fmt.Print(common.GetInstalledComponents()) // Note: GetInstalledComponents might need adjustment depending on its intended use now
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

	// --- Run versionCheck unconditionally ---
	fmt.Println("Running version checks...")
	if vcComp, vcExists := common.GetComponent("versionCheck"); vcExists {
		if vcComp.EntryPoint != nil {
			vcCmd := &cobra.Command{Use: vcComp.Name, Run: vcComp.EntryPoint, DisableFlagParsing: true}
			vcCmd.ExecuteC()
		} else if vcComp.ExecuteFunc != nil {
			vcComp.ExecuteFunc()
		} else {
			fmt.Printf("Warning: Component versionCheck has no execution method defined.\n")
		}
	} else {
		fmt.Println("Warning: versionCheck component not found in registry.")
	}
	fmt.Println("Finished version checks.")
	// --- Get the list of *other* components to run from the centralized function ---
	componentsToRunStr := common.GetInstalledComponents() // This might need adjustment if versionCheck is included here
	if componentsToRunStr == "" {
		fmt.Println("No other components determined to run in this cycle.")
		fmt.Println("Finished component checks for this cycle.")
		return // Nothing to do
	}
	componentsToRun := strings.Split(componentsToRunStr, "::")
	fmt.Printf("Components determined to run: %v\n", componentsToRun)

	// --- Iterate through the determined components and execute them ---
	for _, compName := range componentsToRun {
		if comp, exists := common.GetComponent(compName); exists {
			// Platform/disabled checks are already handled by GetInstalledComponents
			fmt.Printf("Running component: %s\n", compName)
			if comp.EntryPoint != nil {
				cmd := &cobra.Command{Use: comp.Name, Run: comp.EntryPoint, DisableFlagParsing: true}
				cmd.ExecuteC()
			} else if comp.ExecuteFunc != nil {
				comp.ExecuteFunc()
			} else {
				fmt.Printf("Warning: Component %s determined to run but has no execution method defined.\n", compName)
			}
		} else {
			// This should ideally not happen if GetInstalledComponents is correct
			fmt.Printf("Warning: Component %s was listed to run but not found in registry.\n", compName)
		}
	} // End of component loop

	fmt.Println("Finished component checks for this cycle.")
}
