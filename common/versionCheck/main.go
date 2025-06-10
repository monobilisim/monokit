package common

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	news "github.com/monobilisim/monokit/common/redmine/news"
	"github.com/spf13/cobra"
)

func init() {
	common.RegisterComponent(common.Component{
		Name:       "versionCheck",
		EntryPoint: VersionCheck,
		Platform:   "any", // Assuming it can run anywhere to check external versions
	})
}

func StoreVersion(service string, version string) {
	common.WriteToFile(common.TmpDir+"/"+service+".version", version)
}

func GatherVersion(service string) string {
	// Check if the service has a file
	if _, err := os.Stat(common.TmpDir + "/" + service + ".version"); os.IsNotExist(err) {
		return ""
	}

	// Read the file
	content, err := ioutil.ReadFile(common.TmpDir + "/" + service + ".version")
	if err != nil {
		return ""
	}

	return string(content)
}

func CreateNews(service string, oldVersion string, newVersion string, compactTitle bool) {
	var identifier string = common.Config.Identifier
	if compactTitle {
		parts := strings.Split(identifier, "-")
		if len(parts) > 1 {
			identifier = strings.Join(parts[1:], "-")
		}
	}
	news.Create(identifier+" sunucusunun "+service+" sürümü güncellendi", common.Config.Identifier+" sunucusunda "+service+", "+oldVersion+" sürümünden "+newVersion+" sürümüne yükseltildi.", true)
}

func VersionCheck(cmd *cobra.Command, args []string) {
	version := "0.1.0"
	common.ScriptName = "versionCheck"
	common.TmpDir = "/var/cache/mono/" + common.ScriptName
	common.Init()

	fmt.Println("versionCheck - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	// Proxmox
	ProxmoxVECheck()
	ProxmoxMGCheck()
	ProxmoxBSCheck()

	// Zimbra
	ZimbraCheck()

	// OPNsense
	OPNsenseCheck()

	// PostgreSQL
	PostgresCheck()

	// RKE2 Kubernetes
	RKE2VersionCheck()
}
