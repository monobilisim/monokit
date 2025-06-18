//go:build plugin

package k8sHealth

import (
	"fmt"
	"path/filepath" // Added for directory extraction
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
)

// NewK8sHealthData creates and initializes a new K8sHealthData struct.
func NewK8sHealthData() *K8sHealthData {
	return &K8sHealthData{
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
		Nodes:       make([]NodeHealthInfo, 0),
		Pods:        make([]PodHealthInfo, 0),
		// Initialize other slices and pointers as needed
		Rke2IngressNginx: &Rke2IngressNginxHealth{},
		CertManager:      &CertManagerHealth{},
		KubeVip:          &KubeVipHealth{},
		ClusterApiCert:   &ClusterApiCertHealth{},
		// PodRunningLogChecks: make([]PodLogCheckInfo, 0), // Removed as per user request
		Errors: make([]string, 0),
	}
}

// RenderAll renders all health data as a single string for display.
func (khd *K8sHealthData) RenderAll() string {
	return khd.RenderCompact()
}

// RenderCompact renders a compact view of Kubernetes health data, styled similarly to esHealth.
func (khd *K8sHealthData) RenderCompact() string {
	var sb strings.Builder

	if len(khd.Errors) > 0 {
		sb.WriteString(common.SectionTitle("Kubernetes Health Errors"))
		sb.WriteString("\n")
		for _, errMsg := range khd.Errors {
			sb.WriteString(fmt.Sprintf("- %s\n", errMsg)) // Retain simple error listing
		}
		// If there are general errors, perhaps return early or clearly separate from other data
		sb.WriteString("\n") // Add a separator
	}

	// --- Nodes Section ---
	sb.WriteString(common.SectionTitle("Kubernetes Node Status"))
	sb.WriteString("\n")
	if len(khd.Nodes) == 0 {
		sb.WriteString("No node information available.\n")
	} else {
		masterNodes := 0
		workerNodes := 0
		readyMasters := 0
		readyWorkers := 0
		problematicNodes := []NodeHealthInfo{}

		for _, node := range khd.Nodes {
			if node.Role == "master" {
				masterNodes++
				if node.IsReady {
					readyMasters++
				} else {
					problematicNodes = append(problematicNodes, node)
				}
			} else if node.Role == "worker" {
				workerNodes++
				if node.IsReady {
					readyWorkers++
				} else {
					problematicNodes = append(problematicNodes, node)
				}
			} else { // Unknown role
				if !node.IsReady {
					problematicNodes = append(problematicNodes, node)
				}
			}
		}
		sb.WriteString(common.StatusListItem(
			"Master Nodes Ready",
			"",                              // Unit
			fmt.Sprintf("%d", masterNodes),  // Limit (Total)
			fmt.Sprintf("%d", readyMasters), // Actual (Ready)
			readyMasters == masterNodes && masterNodes > 0,
		))
		sb.WriteString("\n")
		sb.WriteString(common.StatusListItem(
			"Worker Nodes Ready",
			"",                              // Unit
			fmt.Sprintf("%d", workerNodes),  // Limit (Total)
			fmt.Sprintf("%d", readyWorkers), // Actual (Ready)
			readyWorkers == workerNodes && workerNodes > 0,
		))
		sb.WriteString("\n")

		if len(problematicNodes) > 0 {
			sb.WriteString("  Problematic Node Details:\n")
			for _, node := range problematicNodes {
				sb.WriteString(fmt.Sprintf("    └─ %s (%s): Status %s, Reason: %s\n", node.Name, node.Role, node.Status, node.Reason))
			}
		}
	}

	// --- Pods Section ---
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Kubernetes Pod Status"))
	sb.WriteString("\n")
	if len(khd.Pods) == 0 {
		sb.WriteString("No pod information available.\n")
	} else {
		runningOrSucceededPods := 0
		problematicPods := []PodHealthInfo{}
		for _, pod := range khd.Pods {
			if !pod.IsProblem { // IsProblem should be true if not Running or Succeeded
				runningOrSucceededPods++
			} else {
				problematicPods = append(problematicPods, pod)
			}
		}
		sb.WriteString(common.SimpleStatusListItem( // Changed to SimpleStatusListItem
			"Pods Healthy",
			fmt.Sprintf("%d / %d", runningOrSucceededPods, len(khd.Pods)), // Value shows Healthy / Total
			len(problematicPods) == 0 && len(khd.Pods) > 0,                // Success if no problematic pods
		))
		sb.WriteString("\n")

		if len(problematicPods) > 0 {
			sb.WriteString("  Problematic Pod Details:\n")
			for _, pod := range problematicPods {
				sb.WriteString(fmt.Sprintf("    └─ %s/%s\n", pod.Namespace, pod.Name)) // Pod name on its own line
				sb.WriteString(fmt.Sprintf("         └─ Phase: %s\n", pod.Phase))      // Pod phase indented underneath
				for _, cs := range pod.ContainerStates {
					if !cs.IsReady && !(cs.State == "Terminated" && cs.Reason == "Completed") {
						msg := cs.Message
						if len(msg) > 60 { // Basic truncation for very long messages
							msg = msg[:57] + "..."
						}
						// Container details also indented under the pod name, at the same level as Phase
						sb.WriteString(fmt.Sprintf("         └─ Container %s: %s\n", cs.Name, cs.State))
						if cs.Reason != "" {
							sb.WriteString(fmt.Sprintf("              └─ Reason: %s\n", cs.Reason))
						}
						if msg != "" {
							sb.WriteString(fmt.Sprintf("              └─ Message: %s\n", msg))
						}
					}
				}
			}
		}
	}

	// --- Pod Log Orphan Checks Section (Removed as per user request) ---
	// if len(khd.PodRunningLogChecks) > 0 {
	// 	sb.WriteString("\n")
	// 	sb.WriteString(common.SectionTitle("Pod Log Orphan Analysis"))
	// 	sb.WriteString("\n")
	// 	orphansFound := false
	// 	for _, plc := range khd.PodRunningLogChecks {
	// 		if !plc.PodExists {
	// 			orphansFound = true
	// 			// Using SimpleStatusListItem, marking success as false to highlight it (though it's informational)
	// 			sb.WriteString(common.SimpleStatusListItem(
	// 				fmt.Sprintf("Orphaned Log: %s", plc.LogFileName),
	// 				fmt.Sprintf("Pod %s/%s missing", plc.PodNamespace, plc.PodName),
	// 				false, // Visually flag as an issue/warning
	// 			))
	// 			sb.WriteString("\n")
	// 		}
	// 	}
	// 	if !orphansFound {
	// 		sb.WriteString("  No orphaned pod logs found.\n")
	// 	}
	// }

	// --- RKE2 Ingress Nginx Section ---
	if khd.Rke2IngressNginx != nil {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("RKE2 Ingress Nginx Status"))
		sb.WriteString("\n")
		if khd.Rke2IngressNginx.Error != "" {
			sb.WriteString(common.SimpleStatusListItem("RKE2 Ingress Check", "Error", false))
			sb.WriteString(fmt.Sprintf("\n    └─ Error: %s\n", khd.Rke2IngressNginx.Error))
		} else {
			// Display only the directory of the manifest path
			manifestDisplayValue := "Not Found"
			if khd.Rke2IngressNginx.ManifestPath != "" && khd.Rke2IngressNginx.ManifestPath != "/" {
				if khd.Rke2IngressNginx.ManifestAvailable {
					manifestDisplayValue = filepath.Dir(khd.Rke2IngressNginx.ManifestPath)
				} else {
					// If not available, but a path was attempted, show the dir of the attempted path
					manifestDisplayValue = "Not Found (at " + filepath.Dir(khd.Rke2IngressNginx.ManifestPath) + ")"
				}
			}

			sb.WriteString(common.SimpleStatusListItem(
				"Manifest Dir", // Changed label to "Manifest Dir"
				manifestDisplayValue,
				khd.Rke2IngressNginx.ManifestAvailable,
			))
			sb.WriteString("\n")

			if khd.Rke2IngressNginx.ManifestAvailable { // Details below only make sense if manifest was found and parsed
				if khd.Rke2IngressNginx.PublishServiceEnabled != nil {
					sb.WriteString(common.SimpleStatusListItem(
						"PublishService Enabled",
						fmt.Sprintf("%t", *khd.Rke2IngressNginx.PublishServiceEnabled),
						*khd.Rke2IngressNginx.PublishServiceEnabled,
					))
					sb.WriteString("\n")
				}
				// ServiceEnabled might be redundant or a different check, include if distinct
				if khd.Rke2IngressNginx.ServiceEnabled != nil &&
					khd.Rke2IngressNginx.ServiceEnabled != khd.Rke2IngressNginx.PublishServiceEnabled { // Avoid duplicate if they are same
					sb.WriteString(common.SimpleStatusListItem(
						"Service Enabled (Overall)", // Clarify if different from PublishService
						fmt.Sprintf("%t", *khd.Rke2IngressNginx.ServiceEnabled),
						*khd.Rke2IngressNginx.ServiceEnabled,
					))
					sb.WriteString("\n")
				}
			}
			for _, fip := range khd.Rke2IngressNginx.FloatingIPChecks {
				sb.WriteString(common.SimpleStatusListItem(
					fmt.Sprintf("Ingress Floating IP %s", fip.IP),
					fmt.Sprintf("HTTP %d", fip.StatusCode),
					fip.IsAvailable, // IsAvailable should mean HTTP 404 or other "OK" codes for ingress
				))
				sb.WriteString("\n")
			}
		}
	}

	// --- Cert-Manager Section ---
	if khd.CertManager != nil {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Cert-Manager Status"))
		sb.WriteString("\n")
		if khd.CertManager.Error != "" {
			sb.WriteString(common.SimpleStatusListItem("Cert-Manager Check", "Error", false))
			sb.WriteString(fmt.Sprintf("\n    └─ Error: %s\n", khd.CertManager.Error))
		} else {
			sb.WriteString(common.SimpleStatusListItem(
				"Namespace (cert-manager)",
				"Present", // Value if NamespaceAvailable is true
				khd.CertManager.NamespaceAvailable,
			))
			sb.WriteString("\n")

			if khd.CertManager.NamespaceAvailable {
				if len(khd.CertManager.Certificates) == 0 {
					sb.WriteString("  No cert-manager certificates found or monitored.\n")
				} else {
					for _, cert := range khd.CertManager.Certificates {
						sb.WriteString(common.SimpleStatusListItem(
							fmt.Sprintf("Certificate: %s", cert.Name),
							fmt.Sprintf("Ready: %t", cert.IsReady),
							cert.IsReady,
						))
						sb.WriteString("\n")
						if !cert.IsReady {
							sb.WriteString(fmt.Sprintf("    └─ Reason: %s, Message: %s\n", cert.Reason, cert.Message))
						}
					}
				}
			}
		}
	}
	// Note: If khd.CertManager is nil, cert-manager checking is disabled and we skip this section

	// --- Kube-VIP Section ---
	if khd.KubeVip != nil {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Kube-VIP Status"))
		sb.WriteString("\n")
		if khd.KubeVip.Error != "" {
			sb.WriteString(common.SimpleStatusListItem("Kube-VIP Check", "Error", false))
			sb.WriteString(fmt.Sprintf("\n    └─ Error: %s\n", khd.KubeVip.Error))
		} else {
			status := "Not Detected"
			if khd.KubeVip.PodsAvailable {
				status = "Detected"
			}
			sb.WriteString(common.SimpleStatusListItem(
				"Kube-VIP Pods",
				status, // Value based on PodsAvailable
				khd.KubeVip.PodsAvailable,
			))
			sb.WriteString("\n")
			if khd.KubeVip.PodsAvailable {
				if len(khd.KubeVip.FloatingIPChecks) == 0 {
					sb.WriteString("  No Kube-VIP floating IPs configured for checking.\n")
				}
				for _, fip := range khd.KubeVip.FloatingIPChecks {
					sb.WriteString(common.SimpleStatusListItem(
						fmt.Sprintf("Kube-VIP Floating IP %s", fip.IP),
						"Ping Test",     // Test type
						fip.IsAvailable, // IsAvailable means ping successful
					))
					sb.WriteString("\n")
				}
			}
		}
	}
	// Note: If khd.KubeVip is nil, kube-vip checking is disabled and we skip this section

	// --- Cluster API Cert Section ---
	if khd.ClusterApiCert != nil {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Cluster API Certificate Status"))
		sb.WriteString("\n")
		if khd.ClusterApiCert.Error != "" {
			sb.WriteString(common.SimpleStatusListItem("API Certificate Check", "Error", false))
			sb.WriteString(fmt.Sprintf("\n    └─ Error: %s\n", khd.ClusterApiCert.Error))
		} else {
			sb.WriteString(common.SimpleStatusListItem(
				"Serving Cert File",
				khd.ClusterApiCert.CertFilePath,
				khd.ClusterApiCert.CertFileAvailable,
			))
			sb.WriteString("\n")
			if khd.ClusterApiCert.CertFileAvailable {
				sb.WriteString(common.SimpleStatusListItem(
					"Expiration Status",
					fmt.Sprintf("Expires: %s", khd.ClusterApiCert.NotAfter.Format("2006-01-02")),
					!khd.ClusterApiCert.IsExpired,
				))
				sb.WriteString("\n")
			}
		}
	}

	// Footer (Last Checked) is usually part of the common.DisplayBox title or outer formatting,
	// so not explicitly added here unless it's a specific style requirement.
	// The esHealth example also commented out LastChecked from its RenderCompact.

	return sb.String()
}

// RenderK8sHealthCLI formats the K8sHealthData for command-line display using common.DisplayBox.
// Version string can be used in the title.
func RenderK8sHealthCLI(data *K8sHealthData, version string) string {
	if data == nil {
		return common.DisplayBox("monokit k8sHealth", "Error: No data to display.")
	}
	title := "monokit k8sHealth"
	if version != "" {
		title = fmt.Sprintf("monokit k8sHealth v%s", version) // Indicate it's via plugin
	}
	// Use RenderCompact as it's designed for text UI.
	// RenderAll currently just calls RenderCompact.
	content := data.RenderCompact()
	return common.DisplayBox(title, content)
}
