package common

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/monobilisim/monokit/common"
	news "github.com/monobilisim/monokit/common/redmine/news"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeconfigPath determines the correct kubeconfig path to use based on priority:
// 1. Explicit flag value (if provided)
// 2. KUBECONFIG environment variable
// 3. Default path ($HOME/.kube/config)
// Returns an empty string if none are found (for in-cluster config)
func GetKubeconfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	// Check environment variable
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}

	// Default path
	if homeDir, err := os.UserHomeDir(); err == nil {
		defaultPath := homeDir + "/.kube/config"
		if _, err := os.Stat(defaultPath); err == nil {
			return defaultPath
		}
	}

	return "" // Will use in-cluster config
}

// GetCurrentNodeName determines the current node name using various methods
func GetCurrentNodeName() string {
	// Extract from common.Config.Identifier if available
	identifier := common.Config.Identifier
	if identifier != "" {
		return identifier
	}

	// Fallback to hostname
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return ""
}

// GetKubernetesServerVersion gets the Kubernetes server version using discovery client
func GetKubernetesServerVersion() (*version.Info, error) {
	kubeconfigPath := GetKubeconfigPath("")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build k8s config: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	versionInfo, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	return versionInfo, nil
}

// IsMasterNodeViaAPI checks if current node is a master/control-plane node via Kubernetes API
func IsMasterNodeViaAPI() bool {
	kubeconfigPath := GetKubeconfigPath("")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return false
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false
	}

	// Get current node name
	nodeName := GetCurrentNodeName()
	if nodeName == "" {
		return false
	}

	// Check node labels for master/control-plane role
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return false
	}

	// Check for standard master node labels
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
		return true
	}

	return false
}

// CreateKubernetesClient creates a Kubernetes clientset using the detected kubeconfig
func CreateKubernetesClient() (*kubernetes.Clientset, error) {
	kubeconfigPath := GetKubeconfigPath("")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return clientset, nil
}

// createK8sVersionNews creates news for Kubernetes version updates
func createK8sVersionNews(clusterName, distroName, oldVersion, newVersion string) {
	title := fmt.Sprintf("%s Cluster'ı %s sürümü güncellendi", clusterName, distroName)
	description := fmt.Sprintf("%s Cluster'ı %s sürümünden %s sürümüne güncellendi.",
		clusterName, oldVersion, newVersion)

	newsId := news.Create(title, description, true) // true for noDuplicate

	if newsId != "" {
		fmt.Printf("Created news item for %s version update: %s\n", distroName, newsId)
	} else {
		fmt.Printf("Failed to create or news already exists for this %s version update\n", distroName)
	}
}

// GetClusterName implements both Option 1 (auto-detection) and Option 3 (config override)
// for extracting cluster names from node identifiers.
func GetClusterName(configOverride string) string {
	// Option 3: Use config override if provided
	if configOverride != "" {
		return configOverride
	}

	// Option 1: Extract from common.Config.Identifier
	// Example: "test-rke2-worker1" → "test-rke2"
	identifier := common.Config.Identifier
	if identifier == "" {
		return ""
	}

	// Split by hyphen and remove the last part (node identifier)
	parts := strings.Split(identifier, "-")

	// If there's only one part, return it as-is
	if len(parts) <= 1 {
		return identifier
	}

	// Remove the last part (assuming it's the node identifier like worker1, master1, etc.)
	clusterParts := parts[:len(parts)-1]
	return strings.Join(clusterParts, "-")
}
