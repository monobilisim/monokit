//go:build linux

package zimbraLdap

import (
	_ "embed"
	"fmt"
	"os" // Import the os package
	"os/exec"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra" // Import cobra
)

// DetectZimbraLdap checks for the presence of Zimbra installation directories.
// This logic is similar to zimbraHealth's detection.
func DetectZimbraLdap() bool {
	// Check for standard Zimbra path
	if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
		common.LogDebug("Zimbra detected at /opt/zimbra for zimbraLdap.")
		return true
	}
	// Check for Carbonio/Zextras path
	if _, err := os.Stat("/opt/zextras"); !os.IsNotExist(err) {
		common.LogDebug("Zextras/Carbonio detected at /opt/zextras for zimbraLdap.")
		return true
	}
	common.LogDebug("Neither /opt/zimbra nor /opt/zextras found. Zimbra LDAP not detected.")
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

// Adjusted signature to match common.Component.EntryPoint
func Main(cmd *cobra.Command, args []string) {
	c := exec.Command("bash")
	c.Stdin = strings.NewReader(script)

	b, e := c.Output()
	if e != nil {
		common.LogError(e.Error())
	}
	fmt.Println(string(b))
}
