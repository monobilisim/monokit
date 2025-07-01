//go:build plugin

package k8sHealth

import (
	"time"

	"github.com/monobilisim/monokit/common" // For ConfInit, ConfExists
)

// K8sHealthProvider implements the health.Provider interface.
// It's defined here to be accessible by both plugin and non-plugin builds.
type K8sHealthProvider struct{}

// Name returns the name of the provider
func (p *K8sHealthProvider) Name() string {
	return "k8sHealth"
}

// Collect gathers Kubernetes health data.
// The 'hostname' parameter is used for context but k8s data is cluster-wide.
// Note: This method relies on functions (e.g., getKubeconfigPath, InitClientset, collectK8sHealthData)
// and variables (e.g., clientset) that might need to be made accessible/exported
// from other files in the k8sHealth package if they are not already.
func (p *K8sHealthProvider) Collect(hostname string) (interface{}, error) {
	// Initialize config if not already done
	// This uses the global K8sHealthConfig from this types.go file
	if len(K8sHealthConfig.K8s.Floating_Ips) == 0 && len(K8sHealthConfig.K8s.Ingress_Floating_Ips) == 0 {
		if common.ConfExists("k8s") {
			if err := common.ConfInit("k8s", &K8sHealthConfig); err != nil {
				// Consider how to handle/log this error, as plugins might not have easy stdout
				// For now, let it proceed, Collect might fail later if config is crucial and missing
			}
		}
	}

	// Initialize clientset if not already done.
	// These functions/variables (clientset, getKubeconfigPath, InitClientset, collectK8sHealthData)
	// are expected to be available from the k8sHealth package (e.g., defined in k8s.go or main.go and exported).
	// If they are not, this will cause a compile error that we'll address next.
	if Clientset == nil { // Expects global 'Clientset *kubernetes.Clientset' from k8s.go
		kubeconfigPath := GetKubeconfigPath("") // Call exported func from k8s.go
		InitClientset(kubeconfigPath)           // Call exported func from k8s.go (already was)
	}

	return CollectK8sHealthData(), nil // Call exported func from k8s.go
}

// Ensure the original import "time" is preserved and Go tooling will merge/format correctly.
// The line directive was to insert after line 3, which had "import \"time\"".
// The closing parenthesis for imports is added above.

// Config holds configuration specific to k8sHealth checks,
// typically loaded from a YAML file (e.g., k8s.yml).
type Config struct {
	K8s struct {
		Floating_Ips         []string `yaml:"floating_ips"`
		Ingress_Floating_Ips []string `yaml:"ingress_floating_ips"`
		EnableCertManager    *bool    `yaml:"enable_cert_manager"` // nil = auto-detect, true/false = force
		EnableKubeVip        *bool    `yaml:"enable_kube_vip"`     // nil = auto-detect, true/false = force
	} `yaml:"k8s"`
	Alarm struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"alarm"`
}

// K8sHealthConfig is the global instance of the k8sHealth configuration.
// This will be populated by the plugin's main or init function.
var K8sHealthConfig Config

// RKE2Info holds the information collected by the RKE2 checker functionality
// and is used for RKE2-specific health checks within k8sHealth.
type RKE2Info struct {
	IsRKE2Environment bool   `json:"isRke2Environment"`
	ClusterName       string `json:"clusterName"`
	CurrentVersion    string `json:"currentVersion"`
	IsMasterNode      bool   `json:"isMasterNode"`
	Error             string `json:"error,omitempty"`
}

// --- UI Data Structures ---

// K8sHealthData holds all collected Kubernetes health information for UI rendering
type K8sHealthData struct {
	Nodes            []NodeHealthInfo
	Pods             []PodHealthInfo
	Rke2IngressNginx *Rke2IngressNginxHealth
	CertManager      *CertManagerHealth
	KubeVip          *KubeVipHealth
	ClusterApiCert   *ClusterApiCertHealth
	RKE2Info         *RKE2Info // Added RKE2 information
	// PodRunningLogChecks []PodLogCheckInfo // Removed as per user request
	LastChecked string
	Errors      []string // To store any general error messages
}

// NodeHealthInfo holds information about a Kubernetes node
type NodeHealthInfo struct {
	Name    string
	Role    string // "master" or "worker"
	Status  string // e.g., "Ready", "NotReady"
	Reason  string // e.g., "KubeletReady", "MemoryPressure"
	IsReady bool
}

// PodHealthInfo holds information about a Kubernetes pod
type PodHealthInfo struct {
	Namespace         string
	Name              string
	Phase             string // e.g., "Running", "Pending", "Failed", "Succeeded"
	IsProblem         bool   // True if phase is not Running or Succeeded (and not Pending for too long)
	ContainerStates   []ContainerHealthInfo
	DeletionCandidate bool // True if log file exists but pod doesn't (from CheckPodRunningLogs)
}

// ContainerHealthInfo holds information about a container within a pod
type ContainerHealthInfo struct {
	Name    string
	State   string // e.g., "Running", "Waiting", "Terminated"
	Reason  string // e.g., "Completed", "CrashLoopBackOff", "ImagePullBackOff"
	Message string
	IsReady bool // Simplified: true if Running
}

// Rke2IngressNginxHealth holds information about RKE2 Ingress Nginx
type Rke2IngressNginxHealth struct {
	ManifestAvailable     bool
	ManifestPath          string
	PublishServiceEnabled *bool // Pointer to distinguish between false and not set
	ServiceEnabled        *bool // Pointer to distinguish between false and not set
	FloatingIPChecks      []FloatingIPCheck
	Error                 string
}

// FloatingIPCheck holds information about a floating IP check
type FloatingIPCheck struct {
	IP          string
	StatusCode  int
	IsAvailable bool   // True if status code is expected (e.g., 404 for ingress default backend)
	TestType    string // "ingress" or "kube-vip"
}

// CertManagerHealth holds information about Cert-Manager
type CertManagerHealth struct {
	NamespaceAvailable bool
	Certificates       []CertificateInfo
	Error              string
}

// CertificateInfo holds information about a cert-manager.io/Certificate
type CertificateInfo struct {
	Name    string
	IsReady bool
	Reason  string
	Message string
}

// KubeVipHealth holds information about Kube-VIP
type KubeVipHealth struct {
	PodsAvailable    bool
	FloatingIPChecks []FloatingIPCheck
	Error            string
}

// ClusterApiCertHealth holds information about the Kube API server certificate
type ClusterApiCertHealth struct {
	CertFileAvailable bool
	CertFilePath      string
	IsExpired         bool
	NotAfter          time.Time
	Error             string
}

// PodLogCheckInfo is removed as per user request
// type PodLogCheckInfo struct {
// LogFileName  string
// PodName      string
// PodNamespace string
// PodExists    bool
// Message      string
// }

// Helper to add an error to K8sHealthData
func (khd *K8sHealthData) AddError(err string) {
	if err != "" {
		khd.Errors = append(khd.Errors, err)
	}
}
