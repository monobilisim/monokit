//go:build plugin

package k8sHealth

import (
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common" // For ConfInit, ConfExists
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
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
	if !k8sConfigLoaded {
		log.Debug().
			Str("component", "k8sHealth").
			Str("operation", "Collect").
			Str("action", "config_check").
			Msg(fmt.Sprintf("Before config load: K8sHealthConfig.Alarm.Enabled=%v (effective=%v)", K8sHealthConfig.Alarm.Enabled, isK8sAlarmEnabled()))
		if common.ConfExists("k8s") {
			log.Debug().Str("component", "k8sHealth").Str("operation", "Collect").Str("action", "config_exists").Msg("k8s config file exists, loading...")
			// Use our clean config loader to avoid global Viper state pollution
			if err := loadK8sConfig(); err != nil {
				log.Error().Err(err).Str("component", "k8sHealth").Str("operation", "Collect").Str("action", "load_config_failed").Msg("Failed to load k8s config")
			} else {
				log.Debug().Str("component", "k8sHealth").Str("operation", "Collect").Str("action", "config_loaded").Msg("k8s config loaded successfully")
			}
		} else {
			log.Debug().Str("component", "k8sHealth").Str("operation", "Collect").Str("action", "config_not_found").Msg("k8s config file not found")
		}
		k8sConfigLoaded = true // Mark as loaded regardless of success/failure to avoid repeated attempts
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
		Floating_ips         []string
		Ingress_floating_ips []string
		Enable_cert_manager  *bool    // nil = auto-detect, true/false = force
		Enable_kube_vip      *bool    // nil = auto-detect, true/false = force
		Check_namespaces     []string `mapstructure:"namespaces"`
	}

	Alarm struct {
		Enabled *bool `mapstructure:"enabled"`
	}
}

// K8sHealthConfig is the global instance of the k8sHealth configuration.
// This will be populated by the plugin's main or init function.
var K8sHealthConfig Config

// k8sConfigLoaded tracks whether we've attempted to load the config to avoid reloading
var k8sConfigLoaded bool

// loadK8sConfig loads the k8s configuration using a fresh Viper instance
// This avoids global Viper state pollution that affects boolean parsing
func loadK8sConfig() error {
	v := viper.New()
	v.SetConfigName("k8s")
	v.AddConfigPath("/etc/mono")
	v.SetConfigType("yaml")
	v.SetDefault("alarm.interval", 3) // Match the default from common.ConfInit

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Backward compatibility: honor legacy send_alarm flag if alarm.enabled is not set
	if !v.IsSet("alarm.enabled") && v.IsSet("send_alarm") {
		v.Set("alarm.enabled", v.GetBool("send_alarm"))
	}
	// Default to global alarm setting when not explicitly configured for k8sHealth
	if !v.IsSet("alarm.enabled") {
		v.SetDefault("alarm.enabled", common.Config.Alarm.Enabled)
	}

	if err := v.Unmarshal(&K8sHealthConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

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
	RKE2Info         *RKE2Info               // Added RKE2 information
	ComplianceChecks *ComplianceCheckResults // Added Compliance Checks
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

// KubeVipConfigCheck captures whether RKE2 config.yaml is using the kube-vip floating IP
type KubeVipConfigCheck struct {
	ConfigPath      string
	ServerValue     string
	UsesFloatingIP  bool
	FloatingIPs     []string
	MasterNodeCount int
	Executed        bool
	Reason          string
	Error           string
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
	PodsAvailable       bool
	FloatingIPChecks    []FloatingIPCheck
	DetectedFloatingIPs []string
	ConfigCheck         *KubeVipConfigCheck
	Error               string
}

// ClusterApiCertHealth holds information about the Kube API server certificate
type ClusterApiCertHealth struct {
	CertFileAvailable bool
	CertFilePath      string
	IsExpired         bool
	NotAfter          time.Time
	Error             string
}

// ComplianceItem holds the result of a single compliance check
type ComplianceItem struct {
	Resource string // e.g., "namespace/name" or "node-name"
	Status   bool   // true = Pass, false = Fail
	Message  string // Detailed message or error
}

// ComplianceCheckResults holds all compliance check results
type ComplianceCheckResults struct {
	TopologySkew []ComplianceItem
	ReplicaCount []ComplianceItem
	ImagePull    []ComplianceItem
	MasterTaint  []ComplianceItem
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
