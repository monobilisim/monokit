//go:build linux

package traefikHealth

import (
	"fmt"
	"time"
	"github.com/spf13/cobra"
	"github.com/monobilisim/monokit/common"
)

func Main(cmd *cobra.Command, args []string) {
	version := "0.1.0"
	common.ScriptName = "traefikHealth"
	common.TmpDir = common.TmpDir + "traefikHealth"
	common.Init()

	fmt.Println("Traefik Health - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	common.SplitSection("Service")

	if !common.SystemdUnitActive("traefik.service") {
		common.PrettyPrintStr("Service traefik", false, "active")
		common.AlarmCheckDown("traefik_svc", "Service traefik is not active", false)
	} else {
		common.PrettyPrintStr("Service traefik", true, "active")
		common.AlarmCheckUp("traefik_svc", "Service traefik is now active", false)
	}

	common.SplitSection("Ports")
	
	portsToCheck := []uint32{80, 443}

	ports := common.ConnsByProcMulti("traefik")
	
	for _, port := range portsToCheck {
		if common.ContainsUint32(port, ports) {
			common.PrettyPrintStr("Port "+fmt.Sprint(port), true, "open")
			common.AlarmCheckUp("traefik_port_"+fmt.Sprint(port), "Port "+fmt.Sprint(port)+" is open", false)
		} else {
			common.PrettyPrintStr("Port "+fmt.Sprint(port), false, "closed")
			common.AlarmCheckDown("traefik_port_"+fmt.Sprint(port), "Port "+fmt.Sprint(port)+" is closed", false)
		}
	}
}
