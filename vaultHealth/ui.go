//go:build linux

package vaultHealth

import (
	"fmt"
	"strings"

	"github.com/monobilisim/monokit/common"
)

// RenderVaultHealthCLI renders Vault health data for CLI output
func RenderVaultHealthCLI(healthData *VaultHealthData, version string) string {
	var sb strings.Builder

	// ====== Service Status Section ======
	sb.WriteString(common.SectionTitle("Service Status"))
	sb.WriteString("\n")

	sb.WriteString(common.SimpleStatusListItem(
		"Vault Binary",
		getInstallationStatus(healthData.Service.Installed),
		healthData.Service.Installed))
	sb.WriteString("\n")

	sb.WriteString(common.SimpleStatusListItem(
		"Service Status",
		healthData.Service.Status,
		healthData.Service.Active))
	sb.WriteString("\n")

	// ====== Connection Status Section ======
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("API Connection"))
	sb.WriteString("\n")

	sb.WriteString(common.SimpleStatusListItem(
		"API Address",
		healthData.Connection.Address,
		true)) // Address is informational
	sb.WriteString("\n")

	sb.WriteString(common.SimpleStatusListItem(
		"Connection",
		getConnectionStatus(healthData.Connection.Connected),
		healthData.Connection.Connected))
	sb.WriteString("\n")

	if healthData.Connection.TLSEnabled {
		sb.WriteString(common.SimpleStatusListItem(
			"TLS",
			"Enabled",
			true))
	} else {
		sb.WriteString(common.SimpleStatusListItem(
			"TLS",
			"Disabled",
			false))
	}
	sb.WriteString("\n")

	if !healthData.Connection.Connected {
		if healthData.Connection.Error != "" {
			sb.WriteString(common.SimpleStatusListItem(
				"Error",
				healthData.Connection.Error,
				false))
			sb.WriteString("\n")
		}
		// Create title and wrap in DisplayBox even for connection errors
		title := fmt.Sprintf("Vault Health Check - v%s - %s", version, healthData.LastChecked)
		return common.DisplayBox(title, sb.String())
	}

	// ====== Version Information Section ======
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Version Information"))
	sb.WriteString("\n")

	if healthData.VersionInfo.Version != "" {
		sb.WriteString(common.SimpleStatusListItem(
			"Vault Version",
			healthData.VersionInfo.Version,
			true))
		sb.WriteString("\n")

		sb.WriteString(common.SimpleStatusListItem(
			"Update Status",
			healthData.VersionInfo.UpdateMessage,
			!healthData.VersionInfo.NeedsUpdate))
		sb.WriteString("\n")

		if healthData.VersionInfo.BuildDate != "" {
			sb.WriteString(common.SimpleStatusListItem(
				"Build Date",
				healthData.VersionInfo.BuildDate,
				true))
			sb.WriteString("\n")
		}
	}

	// ====== Seal Status Section ======
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Seal Status"))
	sb.WriteString("\n")

	sb.WriteString(common.SimpleStatusListItem(
		"Initialization",
		getInitializationStatus(healthData.Seal.Initialized),
		healthData.Seal.Initialized))
	sb.WriteString("\n")

	sb.WriteString(common.SimpleStatusListItem(
		"Seal Status",
		getSealStatus(healthData.Seal.Sealed),
		!healthData.Seal.Sealed))
	sb.WriteString("\n")

	if healthData.Seal.SealType != "" {
		sb.WriteString(common.SimpleStatusListItem(
			"Seal Type",
			healthData.Seal.SealType,
			true))
		sb.WriteString("\n")
	}

	if healthData.Seal.Threshold > 0 {
		thresholdInfo := fmt.Sprintf("%d of %d", healthData.Seal.Threshold, healthData.Seal.Shares)
		sb.WriteString(common.SimpleStatusListItem(
			"Threshold",
			thresholdInfo,
			true))
		sb.WriteString("\n")
	}

	// ====== Storage Backend Section ======
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Storage Backend"))
	sb.WriteString("\n")

	if healthData.Storage.Type != "" {
		sb.WriteString(common.SimpleStatusListItem(
			"Storage Type",
			healthData.Storage.Type,
			true))
		sb.WriteString("\n")
	}

	// ====== Cluster Status Section ======
	if healthData.Cluster.HAEnabled {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Cluster Status"))
		sb.WriteString("\n")

		sb.WriteString(common.SimpleStatusListItem(
			"HA Mode",
			"Enabled",
			true))
		sb.WriteString("\n")

		if healthData.Cluster.ClusterName != "" {
			sb.WriteString(common.SimpleStatusListItem(
				"Cluster Name",
				healthData.Cluster.ClusterName,
				true))
			sb.WriteString("\n")
		}

		sb.WriteString(common.SimpleStatusListItem(
			"Node Role",
			getNodeRole(healthData.Cluster.Mode, healthData.Cluster.IsLeader),
			healthData.Cluster.IsLeader || healthData.Cluster.Mode == "active"))
		sb.WriteString("\n")

		if healthData.Cluster.LeaderAddr != "" {
			sb.WriteString(common.SimpleStatusListItem(
				"Leader Address",
				healthData.Cluster.LeaderAddr,
				true))
			sb.WriteString("\n")
		}

		// Overall cluster health
		sb.WriteString(common.SimpleStatusListItem(
			"Cluster Health",
			getClusterHealthStatus(healthData.Cluster.Healthy),
			healthData.Cluster.Healthy))
		sb.WriteString("\n")

		// Display cluster health reason only when unhealthy
		if !healthData.Cluster.Healthy && healthData.Cluster.HealthReason != "" {
			sb.WriteString(common.SimpleStatusListItem(
				"Health Reason",
				healthData.Cluster.HealthReason,
				false))
			sb.WriteString("\n")
		}

		// Display specific health issues only when there are any
		if len(healthData.Cluster.HealthIssues) > 0 {
			for i, issue := range healthData.Cluster.HealthIssues {
				issueLabel := "Health Issue"
				if i > 0 {
					issueLabel = fmt.Sprintf("Health Issue %d", i+1)
				}
				sb.WriteString(common.SimpleStatusListItem(
					issueLabel,
					issue,
					false))
				sb.WriteString("\n")
			}
		}

		// Raft-specific information
		if healthData.Storage.Type == "raft" && healthData.Storage.RaftInfo != nil {
			// Failure tolerance
			if healthData.Cluster.FailureTolerance > 0 {
				sb.WriteString(common.SimpleStatusListItem(
					"Failure Tolerance",
					fmt.Sprintf("%d nodes", healthData.Cluster.FailureTolerance),
					healthData.Cluster.FailureTolerance > 0))
				sb.WriteString("\n")
			}

			// Cluster size and node details
			if len(healthData.Cluster.Nodes) > 0 {
				totalNodes := len(healthData.Cluster.Nodes)
				healthyNodes := 0
				voterNodes := 0

				for _, node := range healthData.Cluster.Nodes {
					if node.Healthy {
						healthyNodes++
					}
					if node.NodeType == "voter" {
						voterNodes++
					}
				}

				// Display cluster size
				sb.WriteString(common.SimpleStatusListItem(
					"Cluster Size",
					fmt.Sprintf("%d nodes", totalNodes),
					true))
				sb.WriteString("\n")

				// Display healthy nodes
				sb.WriteString(common.SimpleStatusListItem(
					"Healthy Nodes",
					fmt.Sprintf("%d/%d", healthyNodes, totalNodes),
					healthyNodes == totalNodes))
				sb.WriteString("\n")

				// Display voter nodes
				if voterNodes > 0 {
					sb.WriteString(common.SimpleStatusListItem(
						"Voter Nodes",
						fmt.Sprintf("%d/%d", voterNodes, totalNodes),
						true))
					sb.WriteString("\n")
				}

				// Calculate and display quorum status
				requiredQuorum := (voterNodes / 2) + 1
				hasQuorum := healthyNodes >= requiredQuorum
				quorumStatus := fmt.Sprintf("Met (%d/%d)", healthyNodes, requiredQuorum)
				if !hasQuorum {
					quorumStatus = fmt.Sprintf("Lost (%d/%d)", healthyNodes, requiredQuorum)
				}

				sb.WriteString(common.SimpleStatusListItem(
					"Quorum Status",
					quorumStatus,
					hasQuorum))
				sb.WriteString("\n")

				// Individual node details section
				if len(healthData.Cluster.Nodes) > 0 {
					sb.WriteString("\n")
					sb.WriteString(common.SectionTitle("Node Details"))
					sb.WriteString("\n")

					for i, node := range healthData.Cluster.Nodes {
						nodeNumber := fmt.Sprintf("Node %d", i+1)

						// Node ID and type
						nodeInfo := fmt.Sprintf("%s (%s)", node.ID[:8], node.NodeType)
						if node.Address != "" {
							nodeInfo = fmt.Sprintf("%s - %s", nodeInfo, node.Address)
						}

						sb.WriteString(common.SimpleStatusListItem(
							nodeNumber,
							nodeInfo,
							true))
						sb.WriteString("\n")

						// Node health status
						healthStatus := node.HealthReason
						if healthStatus == "" {
							healthStatus = "Unknown"
						}

						sb.WriteString(common.SimpleStatusListItem(
							"  Status",
							healthStatus,
							node.Healthy))
						sb.WriteString("\n")

						// Last seen information
						if node.LastSeen != "" {
							sb.WriteString(common.SimpleStatusListItem(
								"  Last Seen",
								node.LastSeen,
								!strings.Contains(node.LastSeen, "ago") ||
									strings.Contains(node.LastSeen, "Leader")))
							sb.WriteString("\n")
						}

						// Version information
						if node.Version != "" {
							versionMatch := node.Version == healthData.VersionInfo.Version
							sb.WriteString(common.SimpleStatusListItem(
								"  Version",
								node.Version,
								versionMatch))
							sb.WriteString("\n")
						}

						// Display specific issues if any
						if len(node.Issues) > 0 {
							for j, issue := range node.Issues {
								issueLabel := "  Issue"
								if j > 0 {
									issueLabel = fmt.Sprintf("  Issue %d", j+1)
								}
								sb.WriteString(common.SimpleStatusListItem(
									issueLabel,
									issue,
									false))
								sb.WriteString("\n")
							}
						}

						// Add spacing between nodes (except for the last one)
						if i < len(healthData.Cluster.Nodes)-1 {
							sb.WriteString("\n")
						}
					}
				}
			}

			// Raft log status
			if healthData.Storage.RaftInfo.CommittedIndex > 0 {
				sb.WriteString(common.SimpleStatusListItem(
					"Raft Index",
					fmt.Sprintf("%d", healthData.Storage.RaftInfo.CommittedIndex),
					true))
				sb.WriteString("\n")
			}
		}
	}

	// ====== Replication Status Section ======
	if healthData.Replication.Enabled {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Replication Status"))
		sb.WriteString("\n")

		sb.WriteString(common.SimpleStatusListItem(
			"Replication",
			"Enabled",
			true))
		sb.WriteString("\n")

		sb.WriteString(common.SimpleStatusListItem(
			"Mode",
			healthData.Replication.Mode,
			healthData.Replication.Mode != "disabled"))
		sb.WriteString("\n")

		if healthData.Replication.Status != "" {
			sb.WriteString(common.SimpleStatusListItem(
				"Status",
				healthData.Replication.Status,
				isReplicationHealthy(healthData.Replication.Status)))
			sb.WriteString("\n")
		}

		if len(healthData.Replication.KnownSecondaries) > 0 {
			sb.WriteString(common.SimpleStatusListItem(
				"Known Secondaries",
				fmt.Sprintf("%d", len(healthData.Replication.KnownSecondaries)),
				true))
			sb.WriteString("\n")
		}
	}

	// Create title with version and timestamp, then wrap in DisplayBox
	title := fmt.Sprintf("Vault Health Check - v%s - %s", version, healthData.LastChecked)
	return common.DisplayBox(title, sb.String())
}

// Helper functions for status formatting

func getInstallationStatus(installed bool) string {
	if installed {
		return "Installed"
	}
	return "Not Installed"
}

func getConnectionStatus(connected bool) string {
	if connected {
		return "Connected"
	}
	return "Disconnected"
}

func getInitializationStatus(initialized bool) string {
	if initialized {
		return "Initialized"
	}
	return "Not Initialized"
}

func getSealStatus(sealed bool) string {
	if sealed {
		return "Sealed"
	}
	return "Unsealed"
}

func getNodeRole(mode string, isLeader bool) string {
	switch mode {
	case "active":
		return "Active (Leader)"
	case "standby":
		return "Standby"
	case "performance_standby":
		return "Performance Standby"
	case "dr_secondary":
		return "DR Secondary"
	default:
		if isLeader {
			return "Leader"
		}
		return mode
	}
}

func getClusterHealthStatus(healthy bool) string {
	if healthy {
		return "Healthy"
	}
	return "Unhealthy"
}

func isReplicationHealthy(status string) bool {
	healthyStates := []string{"running", "ready", "stream-wals"}
	for _, state := range healthyStates {
		if status == state {
			return true
		}
	}
	return false
}

// RenderVaultHealthCompact renders a compact view for dashboard use
func RenderVaultHealthCompact(healthData *VaultHealthData) string {
	var sb strings.Builder

	// Service Status
	sb.WriteString(common.SectionTitle("Vault Status"))
	sb.WriteString("\n")

	// Basic status indicators
	sb.WriteString(common.SimpleStatusListItem(
		"Service",
		healthData.Service.Status,
		healthData.Service.Active))
	sb.WriteString("\n")

	if healthData.Connection.Connected {
		sb.WriteString(common.SimpleStatusListItem(
			"API",
			"Connected",
			true))
		sb.WriteString("\n")

		sb.WriteString(common.SimpleStatusListItem(
			"Seal",
			getSealStatus(healthData.Seal.Sealed),
			!healthData.Seal.Sealed))
		sb.WriteString("\n")

		if healthData.Cluster.HAEnabled {
			sb.WriteString(common.SimpleStatusListItem(
				"Role",
				getNodeRole(healthData.Cluster.Mode, healthData.Cluster.IsLeader),
				healthData.Cluster.IsLeader || healthData.Cluster.Mode == "active"))
			sb.WriteString("\n")
		}

		if healthData.VersionInfo.Version != "" {
			sb.WriteString(common.SimpleStatusListItem(
				"Version",
				healthData.VersionInfo.Version,
				!healthData.VersionInfo.NeedsUpdate))
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString(common.SimpleStatusListItem(
			"API",
			"Disconnected",
			false))
		sb.WriteString("\n")
	}

	return sb.String()
}
