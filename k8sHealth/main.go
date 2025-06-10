package k8sHealth

import (
	"fmt"
	"os" // Import the os package

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/monobilisim/monokit/common/health"
	versionCheck "github.com/monobilisim/monokit/common/versionCheck"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sHealthConfig is now defined in k8sHealth/types.go and initialized here.
// var K8sHealthConfig K8sHealth // This line is removed as K8sHealthConfig is in types.go and used directly

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

// K8sHealthProvider implements the health.Provider interface
type K8sHealthProvider struct{}

// Name returns the name of the provider
func (p *K8sHealthProvider) Name() string {
	return "k8sHealth"
}

// Collect gathers Kubernetes health data.
// The 'hostname' parameter is used for context but k8s data is cluster-wide
func (p *K8sHealthProvider) Collect(hostname string) (interface{}, error) {
	// Initialize config if not already done
	if len(K8sHealthConfig.K8s.Floating_Ips) == 0 && len(K8sHealthConfig.K8s.Ingress_Floating_Ips) == 0 {
		if common.ConfExists("k8s") {
			common.ConfInit("k8s", &K8sHealthConfig)
		}
	}

	// Initialize clientset if not already done
	if clientset == nil {
		kubeconfigPath := getKubeconfigPath("")
		InitClientset(kubeconfigPath)
	}

	return collectK8sHealthData(), nil
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "k8sHealth",
		EntryPoint: Main,
		Platform:   "any", // Relies on Kubernetes API, platform-agnostic
		AutoDetect: DetectK8s,
	})
	// Register health provider
	health.Register(&K8sHealthProvider{})
}

type K8sHealth struct {
	K8s struct {
		Floating_Ips         []string
		Ingress_Floating_Ips []string
	}
}

var K8sHealthConfig K8sHealth         // This remains for ConfInit
var disableCleanupOrphanedAlarms bool // Flag to control orphaned alarm cleanup

func Main(cmd *cobra.Command, args []string) {
	version := "2.1.0" // Updated version due to refactor
	common.ScriptName = "k8sHealth"
	common.TmpDir = common.TmpDir + "k8sHealth"
	common.Init()
	common.ConfInit("k8s", &K8sHealthConfig) // K8sHealthConfig is from this file

	// Run RKE2 version check first
	versionCheck.RKE2VersionCheck()

	api.WrapperGetServiceStatus("k8sHealth") // Keep this for service status reporting

	// Get kubeconfig path from flag using the shared logic
	kubeconfigFlagValue, _ := cmd.Flags().GetString("kubeconfig")
	kubeconfigPath := getKubeconfigPath(kubeconfigFlagValue)

	// Get cleanup flag
	disableCleanupOrphanedAlarms, _ = cmd.Flags().GetBool("disable-cleanup-orphaned-alarms")

	// Initialize the Kubernetes clientset
	InitClientset(kubeconfigPath) // clientset is a global in k8s.go

	// Collect health data
	healthData := collectK8sHealthData() // This function will orchestrate calls to k8s.go

	// Display as a nice box UI
	displayBoxUI(healthData, version)
}

// collectK8sHealthData gathers all Kubernetes health information.
// This function will call the refactored check functions from k8s.go
func collectK8sHealthData() *K8sHealthData {
	healthData := NewK8sHealthData() // From ui.go

	if clientset == nil {
		errMsg := "Failed to initialize Kubernetes clientset. Aborting checks."
		healthData.AddError(errMsg)
		common.LogError(errMsg)
		// Consider an alarm for k8s client initialization failure
		common.AlarmCheckDown("kubernetes_client_init", errMsg, false, "", "")
		return healthData
	}
	common.AlarmCheckUp("kubernetes_client_init", "Kubernetes clientset initialized successfully.", false)

	// Placeholder for calling refactored check functions.
	// These will populate healthData.
	// Example:
	// healthData.Nodes = CollectNodeHealth()
	// healthData.Pods = CollectPodHealth()
	// healthData.Rke2IngressNginx = CollectRke2IngressNginxHealth()
	// healthData.CertManager = CollectCertManagerHealth()
	// healthData.KubeVip = CollectKubeVipHealth()
	var err error // Declare error variable to reuse

	// Collect Node Health
	healthData.Nodes, err = CollectNodeHealth()
	if err != nil {
		errMsg := fmt.Sprintf("Error collecting node health: %v", err)
		healthData.AddError(errMsg)
		common.LogError(errMsg)
		// Specific alarm for node health collection failure can be added if desired
	}

	// Collect Pod Health
	healthData.Pods, err = CollectPodHealth()
	if err != nil {
		errMsg := fmt.Sprintf("Error collecting pod health: %v", err)
		healthData.AddError(errMsg)
		common.LogError(errMsg)
	}

	// Collect RKE2 Ingress Nginx Health
	healthData.Rke2IngressNginx, err = CollectRke2IngressNginxHealth()
	if err != nil {
		errMsg := fmt.Sprintf("Error collecting RKE2 Ingress Nginx health: %v", err)
		healthData.AddError(errMsg)
		common.LogError(errMsg)
		// Note: CollectRke2IngressNginxHealth itself might set healthData.Rke2IngressNginx.Error for some cases
	}

	// Collect Cert-Manager Health
	healthData.CertManager, err = CollectCertManagerHealth()
	if err != nil {
		errMsg := fmt.Sprintf("Error collecting Cert-Manager health: %v", err)
		healthData.AddError(errMsg)
		common.LogError(errMsg)
		// Note: CollectCertManagerHealth itself might set healthData.CertManager.Error
	}

	// Collect Kube-VIP Health
	healthData.KubeVip, err = CollectKubeVipHealth()
	if err != nil {
		errMsg := fmt.Sprintf("Error collecting Kube-VIP health: %v", err)
		healthData.AddError(errMsg)
		common.LogError(errMsg)
		// Note: CollectKubeVipHealth itself might set healthData.KubeVip.Error
	}

	// Collect Cluster API Cert Health
	healthData.ClusterApiCert, err = CollectClusterApiCertHealth()
	if err != nil {
		errMsg := fmt.Sprintf("Error collecting Cluster API Cert health: %v", err)
		healthData.AddError(errMsg)
		common.LogError(errMsg)
		// Note: CollectClusterApiCertHealth itself might set healthData.ClusterApiCert.Error
	}

	// Pod Running Log Checks are being removed as per user request.
	// healthData.PodRunningLogChecks, err = CollectPodRunningLogChecks()
	// if err != nil {
	// errMsg := fmt.Sprintf("Error collecting pod running log checks: %v", err)
	// healthData.AddError(errMsg)
	// common.LogError(errMsg)
	// }

	// Clean up orphaned alarm logs for pods and containers that no longer exist
	if !disableCleanupOrphanedAlarms {
		common.LogInfo("Cleaning up orphaned alarm logs...")
		if err := CleanupOrphanedAlarms(); err != nil {
			errMsg := fmt.Sprintf("Error cleaning up orphaned alarms: %v", err)
			healthData.AddError(errMsg)
			common.LogError(errMsg)
		}
	} else {
		common.LogDebug("Skipping orphaned alarm cleanup (disabled by flag)")
	}

	// The individual check functions handle their specific alarms.
	// General alarms or summary alarms can be placed here based on aggregated healthData.

	return healthData
}

// displayBoxUI displays the health data in a nice box UI.
func displayBoxUI(healthData *K8sHealthData, version string) {
	title := fmt.Sprintf("monokit k8sHealth v%s", version)
	content := healthData.RenderAll() // From ui.go

	renderedBox := common.DisplayBox(title, content)
	fmt.Println(renderedBox)
}
