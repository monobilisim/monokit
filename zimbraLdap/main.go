//go:build linux

package zimbraLdap

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// DetectZimbraLdap checks for the presence of Zimbra installation directories.
// This logic is similar to zimbraHealth's detection.
func DetectZimbraLdap() bool {
	// Check for standard Zimbra path
	if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
		log.Debug().Str("path", "/opt/zimbra").Msg("Zimbra detected for zimbraLdap.")
		return true
	}
	// Check for Carbonio/Zextras path
	if _, err := os.Stat("/opt/zextras"); !os.IsNotExist(err) {
		log.Debug().Str("path", "/opt/zextras").Msg("Zextras/Carbonio detected for zimbraLdap.")
		return true
	}
	log.Debug().Str("path", "none").Msg("Neither /opt/zimbra nor /opt/zextras found. Zimbra LDAP not detected for zimbraLdap.")
	return false
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "zimbraLdap", // Name used in config/daemon loop
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectZimbraLdap, // Add the AutoDetect function
	})
}

//go:embed ldap.sh
var script string

const lastRunFile = "/tmp/monokit_zimbraLdap_last_run"

// shouldRunDaily checks if the component should run based on a daily timestamp.
func shouldRunDaily(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		// File doesn't exist or error reading, assume run is needed
		fmt.Printf("zimbraLdap: No previous timestamp found or error reading %s: %v. Will run.\n", filePath, err)
		return true
	}

	lastCheckUnix, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		fmt.Printf("zimbraLdap: Error parsing timestamp from %s: %v. Will run.\n", filePath, err)
		return true // Error parsing, run anyway
	}

	lastCheckTime := time.Unix(lastCheckUnix, 0)
	if time.Since(lastCheckTime) >= 24*time.Hour {
		fmt.Printf("zimbraLdap: Last run was at %s. More than 24 hours ago. Will run.\n", lastCheckTime.Format(time.RFC3339))
		return true
	}

	fmt.Printf("zimbraLdap: Last run was at %s. Less than 24 hours ago. Skipping run.\n", lastCheckTime.Format(time.RFC3339))
	return false
}

// recordRun writes the current timestamp to the file.
func recordRun(filePath string) {
	nowUnix := time.Now().Unix()
	content := []byte(strconv.FormatInt(nowUnix, 10))
	err := os.WriteFile(filePath, content, 0644)
	if err != nil {
		fmt.Printf("zimbraLdap: Error writing last run timestamp to %s: %v\n", filePath, err)
	} else {
		fmt.Printf("zimbraLdap: Recorded current timestamp %d to %s\n", nowUnix, filePath)
	}
}

// Adjusted signature to match common.Component.EntryPoint
func Main(cmd *cobra.Command, args []string) {
	if !shouldRunDaily(lastRunFile) {
		fmt.Println("zimbraLdap: already executed in last 24 hours; skipping.")
		return
	}
	defer recordRun(lastRunFile)

	c := exec.Command("bash")
	c.Stdin = strings.NewReader(script)

	b, e := c.Output()
	if e != nil {
		log.Error().Msg(e.Error())
	}
	fmt.Println(string(b))
}
