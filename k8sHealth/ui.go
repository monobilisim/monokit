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
		KubernetesEOL:    &KubernetesEOLInfo{},
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
			if khd.KubeVip.ConfigCheck != nil {
				cfg := khd.KubeVip.ConfigCheck
				if cfg.Error != "" {
					sb.WriteString(common.SimpleStatusListItem("RKE2 server endpoint", "Error", false))
					sb.WriteString(fmt.Sprintf("\n    └─ %s\n", cfg.Error))
				} else if !cfg.Executed {
					if cfg.Reason != "" {
						sb.WriteString(fmt.Sprintf("  RKE2 server endpoint check: %s\n", cfg.Reason))
					}
				} else {
					sb.WriteString(common.SimpleStatusListItem(
						"RKE2 server endpoint",
						cfg.ServerValue,
						cfg.UsesFloatingIP,
					))
					sb.WriteString("\n")
					if cfg.Reason != "" {
						sb.WriteString(fmt.Sprintf("    └─ %s\n", cfg.Reason))
					}
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

	// --- Kubernetes EOL Section ---
	if khd.KubernetesEOL != nil {
		eol := khd.KubernetesEOL
		showSection := eol.Checked || eol.Error != "" || (eol.Skipped && eol.SkipReason != "")
		if showSection {
			sb.WriteString("\n")
			sb.WriteString(common.SectionTitle("Kubernetes EOL Status"))
			sb.WriteString("\n")

			switch {
			case eol.Skipped:
				sb.WriteString(common.SimpleStatusListItem("Kubernetes EOL", "Skipped", true))
				sb.WriteString("\n")
				if eol.SkipReason != "" {
					sb.WriteString(fmt.Sprintf("    └─ %s\n", eol.SkipReason))
				}
			case eol.Error != "":
				sb.WriteString(common.SimpleStatusListItem("Kubernetes EOL", "Error", false))
				sb.WriteString(fmt.Sprintf("\n    └─ Error: %s\n", eol.Error))
				if eol.CurrentVersion != "" {
					sb.WriteString(fmt.Sprintf("    └─ Current Version: %s\n", eol.CurrentVersion))
				}
			default:
				versionDisplay := eol.CurrentVersion
				if eol.RawVersion != "" && eol.RawVersion != eol.CurrentVersion {
					versionDisplay = fmt.Sprintf("%s (%s)", eol.CurrentVersion, eol.RawVersion)
				}
				sb.WriteString(common.SimpleStatusListItem(
					fmt.Sprintf("Kubernetes %s", eol.Cycle),
					versionDisplay,
					!eol.IsEOL,
				))
				sb.WriteString("\n")

				var statusValue string
				switch {
				case eol.IsEOL:
					statusValue = fmt.Sprintf("Past EOL by %d day(s) (%s)", -eol.DaysUntilEOL, eol.EOLDate.Format("2006-01-02"))
				case eol.IsNearEOL:
					statusValue = fmt.Sprintf("EOL in %d day(s) (%s)", eol.DaysUntilEOL, eol.EOLDate.Format("2006-01-02"))
				default:
					statusValue = fmt.Sprintf("Supported, EOL %s (%d day(s))", eol.EOLDate.Format("2006-01-02"), eol.DaysUntilEOL)
				}
				sb.WriteString(common.SimpleStatusListItem("EOL Status", statusValue, !eol.IsEOL && !eol.IsNearEOL))
				sb.WriteString("\n")

				if eol.LatestInCycle != "" && eol.LatestInCycle != eol.CurrentVersion {
					sb.WriteString(common.SimpleStatusListItem(
						"Latest Patch in Cycle",
						eol.LatestInCycle,
						false,
					))
					sb.WriteString("\n")
				}
			}
		}
	}

	// --- etcd Cluster Status Section ---
	if khd.EtcdCluster != nil {
		ec := khd.EtcdCluster
		showEtcdCluster := ec.Checked || ec.Error != "" || (ec.Skipped && ec.SkipReason != "")
		if showEtcdCluster {
			sb.WriteString("\n")
			sb.WriteString(common.SectionTitle("etcd Cluster Status"))
			sb.WriteString("\n")

			switch {
			case ec.Skipped:
				sb.WriteString(common.SimpleStatusListItem("etcd Cluster", "Skipped", true))
				sb.WriteString("\n")
				if ec.SkipReason != "" {
					sb.WriteString(fmt.Sprintf("    └─ %s\n", ec.SkipReason))
				}
			case ec.Error != "" && !ec.Healthy:
				sb.WriteString(common.SimpleStatusListItem("etcd Health", "Unhealthy", false))
				sb.WriteString(fmt.Sprintf("\n    └─ %s\n", ec.Error))
			default:
				healthStr := "Unhealthy"
				if ec.Healthy {
					healthStr = "Healthy"
				}
				if ec.HealthTook != "" {
					healthStr = fmt.Sprintf("%s (%s)", healthStr, ec.HealthTook)
				}
				sb.WriteString(common.SimpleStatusListItem("etcd Health", healthStr, ec.Healthy))
				sb.WriteString("\n")

				if ec.Version != "" {
					sb.WriteString(common.SimpleStatusListItem("etcd Version", ec.Version, true))
					sb.WriteString("\n")
				}

				if ec.DbSize > 0 {
					dbUsagePercent := float64(0)
					if ec.DbSize > 0 {
						dbUsagePercent = float64(ec.DbSizeInUse) / float64(ec.DbSize) * 100
					}
					sb.WriteString(common.SimpleStatusListItem("DB Size",
						fmt.Sprintf("%s (in-use: %s, %.0f%%)", formatBytes(ec.DbSize), formatBytes(ec.DbSizeInUse), dbUsagePercent),
						true))
					sb.WriteString("\n")
				}

				if ec.LeaderName != "" {
					sb.WriteString(common.SimpleStatusListItem("Leader", ec.LeaderName, true))
					sb.WriteString("\n")
				}

				if ec.MemberCount > 0 {
					sb.WriteString(common.SimpleStatusListItem("Members",
						fmt.Sprintf("%d", ec.MemberCount), ec.MemberCount > 0))
					sb.WriteString("\n")
					for _, m := range ec.Members {
						leaderMark := ""
						if m.IsLeader {
							leaderMark = " (leader)"
						}
						sb.WriteString(fmt.Sprintf("    └─ %s%s\n", m.Name, leaderMark))
					}
				}

				if ec.Revision > 0 {
					sb.WriteString(fmt.Sprintf("    └─ Raft: term=%d, index=%d, revision=%d\n",
						ec.RaftTerm, ec.RaftIndex, ec.Revision))
				}
			}
		}
	}

	// --- etcd Backup Status Section ---
	if khd.EtcdBackup != nil {
		etcd := khd.EtcdBackup
		showEtcdSection := etcd.Checked || etcd.Error != "" || (etcd.Skipped && etcd.SkipReason != "")
		if showEtcdSection {
			sb.WriteString("\n")
			sb.WriteString(common.SectionTitle("etcd Backup Status"))
			sb.WriteString("\n")

			switch {
			case etcd.Skipped:
				sb.WriteString(common.SimpleStatusListItem("etcd Backup", "Skipped", true))
				sb.WriteString("\n")
				if etcd.SkipReason != "" {
					sb.WriteString(fmt.Sprintf("    └─ %s\n", etcd.SkipReason))
				}
			case etcd.Error != "" && etcd.TotalSnapshots == 0:
				sb.WriteString(common.SimpleStatusListItem("etcd Backup", "Error", false))
				sb.WriteString(fmt.Sprintf("\n    └─ %s\n", etcd.Error))
			default:
				isHealthy := etcd.ValidSnapshots > 0 && !etcd.IsLatestTooOld
				statusStr := fmt.Sprintf("%d Valid / %d Total", etcd.ValidSnapshots, etcd.TotalSnapshots)
				sb.WriteString(common.SimpleStatusListItem("Snapshots", statusStr, isHealthy))
				sb.WriteString("\n")

				if etcd.LatestSnapshot != nil {
					age := time.Since(etcd.LatestSnapshot.ModTime)
					ageStr := fmt.Sprintf("%.1f hours ago", age.Hours())
					sb.WriteString(common.SimpleStatusListItem(
						"Latest Snapshot",
						fmt.Sprintf("%s (%s)", etcd.LatestSnapshot.Filename, ageStr),
						!etcd.IsLatestTooOld,
					))
					sb.WriteString("\n")

					if etcd.LatestSnapshot.Hash != "" {
						sb.WriteString(fmt.Sprintf("    └─ Hash: %s, Revision: %d, Keys: %d\n",
							etcd.LatestSnapshot.Hash, etcd.LatestSnapshot.Revision, etcd.LatestSnapshot.TotalKey))
					}

					sizeStr := formatBytes(etcd.LatestSnapshot.Size)
					sb.WriteString(fmt.Sprintf("    └─ Size: %s\n", sizeStr))
				}

				if etcd.IsLatestTooOld {
					sb.WriteString(fmt.Sprintf("    └─ WARNING: Latest snapshot exceeds max age of %d hours\n", etcd.MaxAgeHours))
				}

				if etcd.InvalidSnapshots > 0 {
					sb.WriteString(fmt.Sprintf("    └─ %d invalid snapshot(s) detected\n", etcd.InvalidSnapshots))
					for _, snap := range etcd.Snapshots {
						if !snap.IsValid {
							sb.WriteString(fmt.Sprintf("       └─ %s: %s\n", snap.Filename, snap.Error))
						}
					}
				}
			}
		}
	}

	// --- Kubernetes Compliance Checks Section ---
	if khd.ComplianceChecks != nil {
		var sbCompliance strings.Builder

		sbCompliance.WriteString("\n")
		sbCompliance.WriteString(common.SectionTitle("Kubernetes Compliance Status"))
		sbCompliance.WriteString("\n")

		// Helper to render compliance list
		renderComplianceList := func(title string, items []ComplianceItem) {
			if len(items) == 0 {
				return
			}
			failCount := 0
			for _, item := range items {
				if !item.Status {
					failCount++
				}
			}
			statusStr := fmt.Sprintf("%d Passed / %d Total", len(items)-failCount, len(items))
			sbCompliance.WriteString(common.SimpleStatusListItem(title, statusStr, failCount == 0))
			sbCompliance.WriteString("\n")

			if failCount > 0 {
				for _, item := range items {
					if !item.Status {
						sbCompliance.WriteString(fmt.Sprintf("    └─ %s: %s\n", item.Resource, item.Message))
					}
				}
			}
		}

		renderComplianceList("Topology Spread Constraints", khd.ComplianceChecks.TopologySkew)
		renderComplianceList("Replica Count Match", khd.ComplianceChecks.ReplicaCount)
		renderComplianceList("Image Pull Policy", khd.ComplianceChecks.ImagePull)
		renderComplianceList("Master Node Taints", khd.ComplianceChecks.MasterTaint)

		// Only append if there's any data
		sb.WriteString(sbCompliance.String())
	}

	// Footer (Last Checked) is usually part of the common.DisplayBox title or outer formatting,
	// so not explicitly added here unless it's a specific style requirement.
	// The esHealth example also commented out LastChecked from its RenderCompact.

	return sb.String()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
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
