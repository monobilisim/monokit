package k8sHealth

import (
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/spf13/cobra"
)

func init() {
	common.RegisterComponent(common.Component{
		Name:       "k8sHealth",
		EntryPoint: Main,
		Platform:   "any", // Relies on Kubernetes API, platform-agnostic
	})
}

type K8sHealth struct {
	K8s struct {
		Floating_Ips         []string
		Ingress_Floating_Ips []string
	}
}

var K8sHealthConfig K8sHealth

func Main(cmd *cobra.Command, args []string) {
	version := "2.0.0"
	common.ScriptName = "k8sHealth"
	common.TmpDir = common.TmpDir + "k8sHealth"
	common.Init()
	common.ConfInit("k8s", &K8sHealthConfig)

	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")

	api.WrapperGetServiceStatus("k8sHealth")

	fmt.Println("K8s Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	InitClientset(kubeconfig)

	CheckPodRunningLogs()

	common.SplitSection("Master Node(s):")
	CheckNodes(true)

	common.SplitSection("Worker Node(s):")
	CheckNodes(false)

	common.SplitSection("RKE2 Ingress Nginx:")
	CheckRke2IngressNginx()

	CheckPods()

	common.SplitSection("Cert Manager:")
	CheckCertManager()

	common.SplitSection("Kube-VIP:")
	CheckKubeVip()

	common.SplitSection("Cluster API Cert:")
	CheckClusterApiCert()
}
