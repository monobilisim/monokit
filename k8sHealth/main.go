package k8sHealth

import (
	"fmt"
	"os" // Import the os package
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// getKubeconfigPath determines the correct kubeconfig path to use based on priority:
// 1. Explicit flag value (if provided)
// 2. KUBECONFIG environment variable
// 3. Default path ($HOME/.kube/config)
// Returns an empty string if none are found or applicable (e.g., for in-cluster detection).
func getKubeconfigPath(flagValue string) string {
	if flagValue != "" {
		common.LogDebug(fmt.Sprintf("Using kubeconfig from flag: %s", flagValue))
		return flagValue
	}

	envVar := os.Getenv("KUBECONFIG")
	if envVar != "" {
		common.LogDebug(fmt.Sprintf("Using kubeconfig from KUBECONFIG env var: %s", envVar))
		return envVar
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultPath := homeDir + "/.kube/config"
		// Check if the default file actually exists before returning it
		if _, err := os.Stat(defaultPath); err == nil {
			common.LogDebug(fmt.Sprintf("Using default kubeconfig path: %s", defaultPath))
			return defaultPath
		} else if !os.IsNotExist(err) {
			// Log error if Stat failed for reasons other than file not existing
			common.LogWarn(fmt.Sprintf("Error checking default kubeconfig path %s: %v", defaultPath, err))
		} else {
			common.LogDebug(fmt.Sprintf("Default kubeconfig %s not found.", defaultPath))
		}
	} else {
		common.LogWarn("Could not determine home directory to find default kubeconfig.")
	}

	common.LogDebug("No explicit kubeconfig path found (flag, env, default). Will rely on in-cluster config if applicable.")
	return "" // Return empty string to let client-go attempt in-cluster config
}

// DetectK8s attempts to initialize a Kubernetes clientset to detect if a cluster is accessible.
func DetectK8s() bool {
	// Try initializing the clientset using the same logic as InitClientset
	// but without modifying the global 'clientset' variable or logging errors extensively.
	// We just need to know if a connection *can* be established.

	// Get kubeconfig path using the shared logic (passing "" as flag value)
	kubeconfigPath := getKubeconfigPath("")
	common.LogDebug(fmt.Sprintf("k8sHealth auto-detection attempting with kubeconfig: %s", kubeconfigPath))

	// Use the determined kubeconfig path (or "" for default/in-cluster)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		common.LogDebug(fmt.Sprintf("k8sHealth auto-detection failed: BuildConfigFromFlags error: %v", err))
		return false
	}
	// Try creating a temporary clientset just for detection
	_, err = kubernetes.NewForConfig(config)
	if err != nil {
		common.LogDebug(fmt.Sprintf("k8sHealth auto-detection failed: NewForConfig error: %v", err))
		return false
	}
	common.LogDebug("k8sHealth auto-detected successfully.")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "k8sHealth",
		EntryPoint: Main,
		Platform:   "any", // Relies on Kubernetes API, platform-agnostic
		AutoDetect: DetectK8s,
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

	// Get kubeconfig path from flag using the shared logic
	kubeconfigFlagValue, _ := cmd.Flags().GetString("kubeconfig")
	kubeconfigPath := getKubeconfigPath(kubeconfigFlagValue)

	api.WrapperGetServiceStatus("k8sHealth")

	fmt.Println("K8s Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	InitClientset(kubeconfigPath) // Initialize the global clientset using the determined path

	// Check if clientset was initialized successfully
	if clientset == nil {
		// InitClientset already logs specific errors.
		common.LogError("Failed to initialize Kubernetes clientset. Aborting checks.")
		// os.Exit(1) // Or return an error if Main could return one
		return // Stop execution if clientset is nil
	}

	// Only run checks if clientset is not nil
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
