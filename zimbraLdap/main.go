//go:build linux

package zimbraLdap

import (
	_ "embed"
	"fmt"
	"os/exec"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra" // Import cobra
)

func init() {
	common.RegisterComponent(common.Component{
		Name:       "zimbraLdap", // Name used in config/daemon loop
		EntryPoint: Main,
		Platform:   "linux",
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
