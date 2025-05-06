package k8sHealth

import "time"

// --- UI Data Structures ---

// K8sHealthData holds all collected Kubernetes health information for UI rendering
type K8sHealthData struct {
	Nodes            []NodeHealthInfo
	Pods             []PodHealthInfo
	Rke2IngressNginx *Rke2IngressNginxHealth
	CertManager      *CertManagerHealth
	KubeVip          *KubeVipHealth
	ClusterApiCert   *ClusterApiCertHealth
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
