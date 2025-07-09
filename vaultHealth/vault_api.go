//go:build linux

package vaultHealth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

// createHTTPClient creates an HTTP client with appropriate TLS settings
func createHTTPClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !VaultHealthConfig.Vault.Tls.Verify,
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

// makeVaultAPIRequest makes a request to the Vault API
func makeVaultAPIRequest(endpoint string) (*http.Response, error) {
	baseURL := VaultHealthConfig.Vault.Address
	if baseURL == "" {
		baseURL = "https://127.0.0.1:8200"
	}

	// Ensure baseURL doesn't end with /
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Ensure endpoint starts with /
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	fullURL := baseURL + endpoint
	log.Debug().Str("url", fullURL).Msg("Making Vault API request")

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add token if configured
	if VaultHealthConfig.Vault.Token != "" {
		req.Header.Set("X-Vault-Token", VaultHealthConfig.Vault.Token)
		log.Debug().Msg("Using Vault token for authentication")
	} else {
		log.Debug().Msg("No Vault token configured")
	}

	req.Header.Set("User-Agent", "MonoKit-VaultHealth/1.0.0")

	client := createHTTPClient()
	return client.Do(req)
}

// checkVaultAPI checks basic API connectivity and populates connection info
func checkVaultAPI(healthData *VaultHealthData) error {
	baseURL := VaultHealthConfig.Vault.Address
	if baseURL == "" {
		baseURL = "https://127.0.0.1:8200"
	}

	healthData.Connection.Address = baseURL
	healthData.Connection.TLSEnabled = strings.HasPrefix(baseURL, "https")

	// Try to reach the health endpoint (no auth required)
	resp, err := makeVaultAPIRequest("/v1/sys/health")
	if err != nil {
		return fmt.Errorf("failed to connect to Vault API: %w", err)
	}
	defer resp.Body.Close()

	// Any response (even error status) means API is reachable
	healthData.Connection.Connected = true

	// Parse the health response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read health response: %w", err)
	}

	var healthResp HealthResponse
	if err := json.Unmarshal(body, &healthResp); err != nil {
		return fmt.Errorf("failed to parse health response: %w", err)
	}

	// Populate basic info from health response
	healthData.VersionInfo.Version = healthResp.Version
	healthData.Cluster.ClusterName = healthResp.ClusterName
	healthData.Cluster.ClusterID = healthResp.ClusterID
	healthData.Connection.Healthy = resp.StatusCode == 200

	// Determine node mode based on status code and response
	switch resp.StatusCode {
	case 200:
		healthData.Cluster.Mode = "active"
		healthData.Cluster.IsLeader = true
	case 429:
		if healthResp.PerformanceStandby {
			healthData.Cluster.Mode = "performance_standby"
		} else {
			healthData.Cluster.Mode = "standby"
		}
		healthData.Cluster.IsLeader = false
	case 472:
		healthData.Cluster.Mode = "dr_secondary"
		healthData.Cluster.IsLeader = false
	case 473:
		healthData.Cluster.Mode = "performance_standby"
		healthData.Cluster.IsLeader = false
	default:
		healthData.Cluster.Mode = "unknown"
		healthData.Cluster.IsLeader = false
	}

	log.Debug().Str("mode", healthData.Cluster.Mode).Int("status", resp.StatusCode).Msg("Vault API check successful")
	return nil
}

// checkSealStatus checks Vault seal status
func checkSealStatus(healthData *VaultHealthData) error {
	resp, err := makeVaultAPIRequest("/v1/sys/seal-status")
	if err != nil {
		return fmt.Errorf("failed to get seal status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read seal status response: %w", err)
	}

	var sealResp SealStatusResponse
	if err := json.Unmarshal(body, &sealResp); err != nil {
		return fmt.Errorf("failed to parse seal status response: %w", err)
	}

	// Populate seal information
	healthData.Seal.Sealed = sealResp.Sealed
	healthData.Seal.SealType = sealResp.Type
	healthData.Seal.Threshold = sealResp.T
	healthData.Seal.Shares = sealResp.N
	healthData.Seal.Initialized = sealResp.Initialized
	healthData.Seal.Progress = sealResp.Progress
	healthData.Seal.Nonce = sealResp.Nonce
	healthData.Seal.Migration = sealResp.Migration

	// Update storage info
	healthData.Storage.Type = sealResp.StorageType

	// Update version info if available
	if sealResp.Version != "" {
		healthData.VersionInfo.Version = sealResp.Version
	}
	if sealResp.BuildDate != "" {
		healthData.VersionInfo.BuildDate = sealResp.BuildDate
	}

	// Create alerts for seal status changes
	if VaultHealthConfig.Vault.Alerts.Sealed_vault {
		if sealResp.Sealed {
			common.AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
		} else {
			common.AlarmCheckUp("vault_sealed", "Vault is unsealed", false)
		}
	}

	log.Debug().Bool("sealed", sealResp.Sealed).Str("type", sealResp.Type).Msg("Vault seal status")
	return nil
}

// checkClusterStatus checks Vault cluster/HA status
func checkClusterStatus(healthData *VaultHealthData) error {
	// Initialize cluster health tracking
	healthData.Cluster.HealthIssues = []string{}
	healthData.Cluster.HealthReason = ""

	// Get leader information
	resp, err := makeVaultAPIRequest("/v1/sys/leader")
	if err != nil {
		healthData.Cluster.HealthReason = "Failed to check leader status"
		healthData.Cluster.HealthIssues = append(healthData.Cluster.HealthIssues, fmt.Sprintf("Leader API error: %v", err))
		healthData.Cluster.Healthy = false
		return fmt.Errorf("failed to get leader status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		healthData.Cluster.HealthReason = "Failed to read leader response"
		healthData.Cluster.HealthIssues = append(healthData.Cluster.HealthIssues, "Could not read leader API response")
		healthData.Cluster.Healthy = false
		return fmt.Errorf("failed to read leader response: %w", err)
	}

	var leaderResp LeaderResponse
	if err := json.Unmarshal(body, &leaderResp); err != nil {
		healthData.Cluster.HealthReason = "Failed to parse leader response"
		healthData.Cluster.HealthIssues = append(healthData.Cluster.HealthIssues, "Invalid leader API response format")
		healthData.Cluster.Healthy = false
		return fmt.Errorf("failed to parse leader response: %w", err)
	}

	// Update cluster information
	healthData.Cluster.HAEnabled = leaderResp.HAEnabled
	healthData.Cluster.IsLeader = leaderResp.IsSelf
	healthData.Cluster.LeaderAddr = leaderResp.LeaderAddress

	// Assess basic cluster health for all storage types
	basicHealthIssues := []string{}
	basicHealthy := true

	// Check if HA is enabled and leader is available
	if healthData.Cluster.HAEnabled {
		if healthData.Cluster.LeaderAddr == "" {
			basicHealthIssues = append(basicHealthIssues, "No leader available in HA cluster")
			basicHealthy = false
		}
	} else {
		// Single node setup - check if this node is active
		if !healthData.Cluster.IsLeader {
			basicHealthIssues = append(basicHealthIssues, "Node is not active in single-node setup")
			basicHealthy = false
		}
	}

	// If using Raft storage, get detailed cluster state
	if healthData.Storage.Type == "raft" {
		if err := checkRaftClusterState(healthData); err != nil {
			log.Debug().Err(err).Msg("Failed to get Raft cluster state")
			healthData.Cluster.HealthIssues = append(healthData.Cluster.HealthIssues, "Failed to retrieve Raft cluster state")
			basicHealthy = false
		}

		// Perform additional Raft checks only if enabled in configuration
		if VaultHealthConfig.Vault.ClusterChecks.Enabled {
			if VaultHealthConfig.Vault.ClusterChecks.Check_configuration {
				if err := checkRaftConfiguration(healthData); err != nil {
					log.Debug().Err(err).Msg("Failed to get Raft configuration")
					healthData.Cluster.HealthIssues = append(healthData.Cluster.HealthIssues, "Raft configuration check failed")
				}
			}

			if VaultHealthConfig.Vault.ClusterChecks.Check_node_health {
				if err := checkClusterNodeHealth(healthData); err != nil {
					log.Debug().Err(err).Msg("Failed to check cluster node health")
					healthData.Cluster.HealthIssues = append(healthData.Cluster.HealthIssues, "Node health check failed")
				}
			}

			if VaultHealthConfig.Vault.ClusterChecks.Check_quorum {
				validateClusterQuorum(healthData)
			}

			if VaultHealthConfig.Vault.ClusterChecks.Check_performance {
				if err := checkClusterPerformance(healthData); err != nil {
					log.Debug().Err(err).Msg("Failed to check cluster performance")
					healthData.Cluster.HealthIssues = append(healthData.Cluster.HealthIssues, "Performance check failed")
				}
			}
		} else {
			log.Debug().Msg("Advanced cluster checks disabled in configuration")
		}
	} else {
		// For non-Raft storage (consul, postgresql, etc.), use basic health assessment
		healthData.Cluster.HealthIssues = append(healthData.Cluster.HealthIssues, basicHealthIssues...)

		// If no Raft-specific health was set, use basic assessment
		if healthData.Cluster.HealthReason == "" {
			if basicHealthy {
				healthData.Cluster.Healthy = true
				healthData.Cluster.HealthReason = fmt.Sprintf("Cluster healthy (%s storage)", healthData.Storage.Type)
			} else {
				healthData.Cluster.Healthy = false
				healthData.Cluster.HealthReason = "Basic cluster health checks failed"
			}
		}
	}

	// Final health assessment - if we have any issues, explain them
	if len(healthData.Cluster.HealthIssues) > 0 {
		if healthData.Cluster.HealthReason == "" || healthData.Cluster.HealthReason == "Cluster healthy" {
			healthData.Cluster.HealthReason = fmt.Sprintf("Cluster has %d health issues", len(healthData.Cluster.HealthIssues))
		}
		// Don't override if Raft checks already set healthy to false
		if healthData.Storage.Type != "raft" {
			healthData.Cluster.Healthy = false
		}
	} else if healthData.Cluster.HealthReason == "" {
		// No issues found and no reason set
		healthData.Cluster.Healthy = true
		if healthData.Storage.Type == "raft" {
			healthData.Cluster.HealthReason = "Raft cluster healthy"
		} else {
			healthData.Cluster.HealthReason = fmt.Sprintf("Cluster healthy (%s storage)", healthData.Storage.Type)
		}
	}

	// Check for leader changes and alerts
	if VaultHealthConfig.Vault.Alerts.Leader_changes && healthData.Cluster.HAEnabled {
		// Log current leader status
		log.Debug().Str("leader", healthData.Cluster.LeaderAddr).Bool("is_leader", healthData.Cluster.IsLeader).Msg("Current Vault leader")

		// Alert if no leader is available
		if healthData.Cluster.LeaderAddr == "" && healthData.Cluster.HAEnabled {
			common.AlarmCheckDown("vault_no_leader", "Vault cluster has no leader", false, "", "")
		} else if healthData.Cluster.LeaderAddr != "" {
			common.AlarmCheckUp("vault_leader_available", "Vault cluster leader is available", false)
		}
	}

	log.Debug().Bool("healthy", healthData.Cluster.Healthy).Str("reason", healthData.Cluster.HealthReason).Int("issues", len(healthData.Cluster.HealthIssues)).Msg("Cluster health assessment")

	return nil
}

// checkRaftClusterState checks Raft cluster state (requires appropriate permissions)
func checkRaftClusterState(healthData *VaultHealthData) error {
	resp, err := makeVaultAPIRequest("/v1/sys/storage/raft/autopilot/state")
	if err != nil {
		return fmt.Errorf("failed to get Raft state: %w", err)
	}
	defer resp.Body.Close()

	// Check if we got permission denied or other auth error
	if resp.StatusCode == 403 {
		log.Debug().Msg("Insufficient permissions to read Raft cluster state")
		return nil
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d for Raft state", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Raft state response: %w", err)
	}

	var raftResp RaftStateResponse
	if err := json.Unmarshal(body, &raftResp); err != nil {
		return fmt.Errorf("failed to parse Raft state response: %w", err)
	}

	// Update cluster information
	healthData.Cluster.Healthy = raftResp.Healthy
	healthData.Cluster.FailureTolerance = raftResp.FailureTolerance

	// Initialize Raft info
	if healthData.Storage.RaftInfo == nil {
		healthData.Storage.RaftInfo = &RaftInfo{}
	}
	healthData.Storage.RaftInfo.LeaderAddr = raftResp.Leader
	healthData.Storage.RaftInfo.FailureTolerance = raftResp.FailureTolerance
	healthData.Storage.RaftInfo.OptimisticFailureTol = raftResp.OptimisticFailureTolerance
	healthData.Storage.RaftInfo.Voters = raftResp.Voters

	// Convert Raft servers to Vault nodes
	healthData.Cluster.Nodes = make([]VaultNode, 0, len(raftResp.Servers))
	for _, server := range raftResp.Servers {
		node := VaultNode{
			ID:          server.ID,
			Address:     server.Address,
			Status:      server.Status,
			LastContact: server.LastContact,
			Version:     server.Version,
			Healthy:     server.Healthy,
			NodeType:    server.NodeType,
		}
		healthData.Cluster.Nodes = append(healthData.Cluster.Nodes, node)
	}

	log.Debug().Bool("healthy", raftResp.Healthy).Int("nodes", len(raftResp.Servers)).Str("leader", raftResp.Leader).Msg("Raft cluster state")

	return nil
}

// checkReplicationStatus checks Vault Enterprise replication status
func checkReplicationStatus(healthData *VaultHealthData) error {
	// Try performance replication status first
	resp, err := makeVaultAPIRequest("/v1/sys/replication/performance/status")
	if err != nil {
		return fmt.Errorf("failed to get replication status: %w", err)
	}
	defer resp.Body.Close()

	// If we get 404, it might be Community Edition
	if resp.StatusCode == 404 {
		healthData.Replication.Enabled = false
		healthData.Replication.Mode = "disabled"
		return nil
	}

	// Check for permission denied
	if resp.StatusCode == 403 {
		log.Debug().Msg("Insufficient permissions to read replication status")
		return nil
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d for replication status", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read replication response: %w", err)
	}

	var replResp struct {
		Data ReplicationStatusResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &replResp); err != nil {
		return fmt.Errorf("failed to parse replication response: %w", err)
	}

	// Update replication information
	healthData.Replication.Enabled = true
	healthData.Replication.Mode = replResp.Data.Mode
	healthData.Replication.Status = replResp.Data.State
	healthData.Replication.ClusterID = replResp.Data.ClusterID
	healthData.Replication.LastWAL = replResp.Data.LastWAL
	healthData.Replication.KnownSecondaries = replResp.Data.KnownSecondaries
	healthData.Replication.ConnectionState = replResp.Data.ConnectionState

	log.Debug().Str("mode", replResp.Data.Mode).Str("state", replResp.Data.State).Msg("Replication status")

	return nil
}

// parseVaultURL parses and validates a Vault URL
func parseVaultURL(address string) (*url.URL, error) {
	if address == "" {
		address = "https://127.0.0.1:8200"
	}

	// Add https:// if no scheme is provided
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}

	return url.Parse(address)
}

// checkRaftConfiguration checks Raft cluster configuration
func checkRaftConfiguration(healthData *VaultHealthData) error {
	resp, err := makeVaultAPIRequest("/v1/sys/storage/raft/configuration")
	if err != nil {
		return fmt.Errorf("failed to get Raft configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		log.Debug().Msg("Insufficient permissions to read Raft configuration")
		return nil
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d for Raft configuration", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Raft configuration response: %w", err)
	}

	var configResp struct {
		Data struct {
			Config struct {
				Servers []struct {
					NodeID      string `json:"node_id"`
					Address     string `json:"address"`
					Leader      bool   `json:"leader"`
					Voter       bool   `json:"voter"`
					ProtocolVer string `json:"protocol_version"`
				} `json:"servers"`
				Index uint64 `json:"index"`
			} `json:"config"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &configResp); err != nil {
		return fmt.Errorf("failed to parse Raft configuration response: %w", err)
	}

	// Count voters and non-voters
	voterCount := 0
	leaderCount := 0
	for _, server := range configResp.Data.Config.Servers {
		if server.Voter {
			voterCount++
		}
		if server.Leader {
			leaderCount++
		}
	}

	// Update cluster information with configuration details
	if healthData.Storage.RaftInfo == nil {
		healthData.Storage.RaftInfo = &RaftInfo{}
	}

	// Store configuration index for monitoring
	healthData.Storage.RaftInfo.CommittedIndex = int64(configResp.Data.Config.Index)

	// Log configuration status
	log.Debug().Int("total_servers", len(configResp.Data.Config.Servers)).Int("voters", voterCount).Int("leaders", leaderCount).Msg("Raft configuration")

	// Alert on configuration issues
	if leaderCount != 1 {
		common.AlarmCheckDown("vault_cluster_leadership",
			fmt.Sprintf("Vault cluster has %d leaders (expected 1)", leaderCount), false, "", "")
	} else {
		common.AlarmCheckUp("vault_cluster_leadership", "Vault cluster has proper leadership", false)
	}

	return nil
}

// checkClusterNodeHealth checks individual node health and captures detailed reasons
func checkClusterNodeHealth(healthData *VaultHealthData) error {
	if len(healthData.Cluster.Nodes) == 0 {
		return nil // No nodes to check
	}

	unhealthyNodes := 0
	partitionedNodes := 0

	for i, node := range healthData.Cluster.Nodes {
		// Clear previous issues
		healthData.Cluster.Nodes[i].Issues = []string{}
		healthData.Cluster.Nodes[i].HealthReason = ""
		healthData.Cluster.Nodes[i].LastSeen = ""

		// Check if node is healthy based on Vault's assessment
		if !node.Healthy {
			unhealthyNodes++
			healthData.Cluster.Nodes[i].Healthy = false
			healthData.Cluster.Nodes[i].HealthReason = "Node marked unhealthy by Vault autopilot"
			healthData.Cluster.Nodes[i].Issues = append(healthData.Cluster.Nodes[i].Issues, "Autopilot health check failed")
		} else {
			healthData.Cluster.Nodes[i].Healthy = true
			healthData.Cluster.Nodes[i].HealthReason = "Healthy"
		}

		// Check node status
		if node.Status != "alive" {
			if node.Status != "" {
				healthData.Cluster.Nodes[i].Issues = append(healthData.Cluster.Nodes[i].Issues, fmt.Sprintf("Node status: %s", node.Status))
				if healthData.Cluster.Nodes[i].HealthReason == "Healthy" {
					healthData.Cluster.Nodes[i].HealthReason = fmt.Sprintf("Node status is '%s'", node.Status)
				}
			}
		}

		// Check for potential network partitions (nodes with stale contact)
		if node.LastContact != "" && node.LastContact != "0s" {
			// Parse the duration and check if it's concerning
			if duration, err := time.ParseDuration(node.LastContact); err == nil {
				healthData.Cluster.Nodes[i].LastSeen = fmt.Sprintf("%s ago", node.LastContact)

				// Consider >30s as potentially concerning, >5m as partition
				if duration > 5*time.Minute {
					partitionedNodes++
					healthData.Cluster.Nodes[i].Issues = append(healthData.Cluster.Nodes[i].Issues, "Network partition suspected")
					if healthData.Cluster.Nodes[i].HealthReason == "Healthy" {
						healthData.Cluster.Nodes[i].HealthReason = "Network partition suspected"
					}
				} else if duration > 30*time.Second {
					healthData.Cluster.Nodes[i].Issues = append(healthData.Cluster.Nodes[i].Issues, "High network latency")
				}
			} else {
				healthData.Cluster.Nodes[i].LastSeen = node.LastContact
			}
		} else if node.LastContact == "0s" {
			healthData.Cluster.Nodes[i].LastSeen = "Leader (no contact time)"
		}

		// Check version consistency
		if healthData.VersionInfo.Version != "" && node.Version != "" && node.Version != healthData.VersionInfo.Version {
			healthData.Cluster.Nodes[i].Issues = append(healthData.Cluster.Nodes[i].Issues, fmt.Sprintf("Version mismatch: %s vs %s", node.Version, healthData.VersionInfo.Version))
		}

		// Check node type consistency for voters
		if node.NodeType == "voter" {
			// Voter nodes should generally be healthy for cluster stability
			if !healthData.Cluster.Nodes[i].Healthy {
				healthData.Cluster.Nodes[i].Issues = append(healthData.Cluster.Nodes[i].Issues, "Voter node is unhealthy")
			}
		}

		// Final health assessment - if there are issues but node was marked healthy, review
		if len(healthData.Cluster.Nodes[i].Issues) > 0 && healthData.Cluster.Nodes[i].Healthy {
			// Check if any issues are critical
			for _, issue := range healthData.Cluster.Nodes[i].Issues {
				if strings.Contains(issue, "partition") || strings.Contains(issue, "Voter node is unhealthy") {
					healthData.Cluster.Nodes[i].Healthy = false
					unhealthyNodes++
					break
				}
			}
		}

		// If no specific reason but node is unhealthy, provide generic reason
		if !healthData.Cluster.Nodes[i].Healthy && healthData.Cluster.Nodes[i].HealthReason == "" {
			healthData.Cluster.Nodes[i].HealthReason = "Node health check failed"
		}
	}

	// Update overall cluster health based on node health
	totalNodes := len(healthData.Cluster.Nodes)
	healthyNodes := totalNodes - unhealthyNodes

	// Consider cluster healthy if majority of nodes are healthy
	healthData.Cluster.Healthy = (healthyNodes > totalNodes/2)

	// Generate alerts for node health issues
	if unhealthyNodes > 0 {
		common.AlarmCheckDown("vault_cluster_node_health",
			fmt.Sprintf("Vault cluster has %d unhealthy nodes out of %d total", unhealthyNodes, totalNodes),
			false, "", "")
	} else {
		common.AlarmCheckUp("vault_cluster_node_health", "All Vault cluster nodes are healthy", false)
	}

	if partitionedNodes > 0 {
		common.AlarmCheckDown("vault_cluster_partitions",
			fmt.Sprintf("Detected %d potentially partitioned nodes", partitionedNodes),
			false, "", "")
	} else if len(healthData.Cluster.Nodes) > 1 {
		common.AlarmCheckUp("vault_cluster_partitions", "No network partitions detected", false)
	}

	log.Debug().Int("healthy", healthyNodes).Int("total", totalNodes).Int("partitioned", partitionedNodes).Msg("Cluster node health")
	return nil
}

// validateClusterQuorum ensures the cluster has proper quorum for operations
func validateClusterQuorum(healthData *VaultHealthData) {
	if !healthData.Cluster.HAEnabled {
		return // Single node, no quorum needed
	}

	totalNodes := len(healthData.Cluster.Nodes)
	if totalNodes == 0 {
		log.Debug().Msg("No cluster nodes information available for quorum validation")
		return
	}

	// Count healthy voter nodes
	healthyVoters := 0
	totalVoters := 0

	for _, node := range healthData.Cluster.Nodes {
		if node.NodeType == "voter" {
			totalVoters++
			if node.Healthy {
				healthyVoters++
			}
		}
	}

	// Calculate required quorum (majority)
	requiredQuorum := (totalVoters / 2) + 1
	hasQuorum := healthyVoters >= requiredQuorum

	// Update cluster health based on quorum
	if !hasQuorum {
		healthData.Cluster.Healthy = false
	}

	// Generate alerts for quorum issues
	if !hasQuorum {
		common.AlarmCheckDown("vault_cluster_quorum",
			fmt.Sprintf("Vault cluster lacks quorum: %d/%d healthy voters (need %d)",
				healthyVoters, totalVoters, requiredQuorum), false, "", "")
	} else {
		common.AlarmCheckUp("vault_cluster_quorum", "Vault cluster has sufficient quorum", false)
	}

	log.Debug().Int("healthy_voters", healthyVoters).Int("total_voters", totalVoters).Bool("has_quorum", hasQuorum).Int("required_quorum", requiredQuorum).Msg("Cluster quorum status")
}

// checkClusterPerformance monitors cluster performance metrics
func checkClusterPerformance(healthData *VaultHealthData) error {
	if !healthData.Cluster.HAEnabled {
		return nil // Single node, no cluster performance to check
	}

	startTime := time.Now()

	// Test cluster responsiveness with a lightweight operation
	resp, err := makeVaultAPIRequest("/v1/sys/health")
	if err != nil {
		return fmt.Errorf("failed to check cluster performance: %w", err)
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	// Check if response time is within acceptable limits
	maxResponseTime := 5 * time.Second // Default 5 seconds
	if VaultHealthConfig.Vault.Limits.Max_response_time != "" {
		if parsedTime, err := time.ParseDuration(VaultHealthConfig.Vault.Limits.Max_response_time); err == nil {
			maxResponseTime = parsedTime
		}
	}

	isResponseTimeOK := responseTime <= maxResponseTime

	// Log performance metrics
	log.Debug().Str("response_time", responseTime.String()).Str("max_response_time", maxResponseTime.String()).Msg("Cluster response time")

	// Generate alerts for performance issues
	if !isResponseTimeOK {
		common.AlarmCheckDown("vault_cluster_performance",
			fmt.Sprintf("Vault cluster response time (%v) exceeds limit (%v)", responseTime, maxResponseTime),
			false, "", "")
	} else {
		common.AlarmCheckUp("vault_cluster_performance", "Vault cluster performance is acceptable", false)
	}

	// Additional Raft-specific performance checks
	if healthData.Storage.Type == "raft" {
		if err := checkRaftPerformance(healthData); err != nil {
			log.Debug().Err(err).Msg("Failed to check Raft performance")
		}
	}

	return nil
}

// checkRaftPerformance checks Raft-specific performance metrics
func checkRaftPerformance(healthData *VaultHealthData) error {
	// Check Raft metrics endpoint
	resp, err := makeVaultAPIRequest("/v1/sys/metrics?format=prometheus")
	if err != nil {
		return fmt.Errorf("failed to get Raft metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		log.Debug().Msg("Insufficient permissions to read Raft metrics")
		return nil
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d for Raft metrics", resp.StatusCode)
	}

	// For now, just verify we can access metrics
	// In the future, this could parse Prometheus metrics for detailed analysis
	log.Debug().Msg("Successfully accessed Raft performance metrics")

	return nil
}

// checkVaultVersionUpdates checks for new Vault versions and sends alerts
func checkVaultVersionUpdates(healthData *VaultHealthData) error {
	if !VaultHealthConfig.Vault.Alerts.Version_updates {
		log.Debug().Msg("Version update alerts disabled in configuration")
		// Set default message when version updates are disabled
		healthData.VersionInfo.UpdateMessage = "Version checking disabled"
		healthData.VersionInfo.NeedsUpdate = false
		return nil
	}

	if healthData.VersionInfo.Version == "" {
		log.Debug().Msg("Cannot check version updates: current version unknown")
		return nil
	}

	currentVersion := healthData.VersionInfo.Version
	log.Debug().Str("current_version", currentVersion).Msg("Checking for updates to current Vault version")

	// Get latest version from HashiCorp releases API
	latestVersion, err := getLatestVaultVersion()
	if err != nil {
		log.Error().Err(err).Msg("Failed to check for Vault updates")
		return err
	}

	log.Debug().Str("latest_version", latestVersion).Msg("Latest Vault version available")

	// Compare versions
	if latestVersion != currentVersion {
		updateAvailable, err := isNewerVersion(latestVersion, currentVersion)
		if err != nil {
			log.Error().Err(err).Msg("Failed to compare versions")
			return err
		}

		if updateAvailable {
			// Update health data
			healthData.VersionInfo.NeedsUpdate = true
			healthData.VersionInfo.UpdateMessage = fmt.Sprintf("Update available: %s", latestVersion)

			// Send alarm about new version
			alarmMessage := fmt.Sprintf("New Vault version available: %s (current: %s)", latestVersion, currentVersion)
			common.AlarmCheckDown("vault_version_update", alarmMessage, false, "", "")

			log.Info().Str("current_version", currentVersion).Str("latest_version", latestVersion).Msg("Vault update available")
		} else {
			// Current version is newer or same (maybe dev/beta version)
			healthData.VersionInfo.NeedsUpdate = false
			healthData.VersionInfo.UpdateMessage = "Up-to-date"
			common.AlarmCheckUp("vault_version_update", "Vault version is up-to-date", false)
		}
	} else {
		// Versions are the same
		healthData.VersionInfo.NeedsUpdate = false
		healthData.VersionInfo.UpdateMessage = "Up-to-date"
		common.AlarmCheckUp("vault_version_update", "Vault version is up-to-date", false)
	}

	return nil
}

// getLatestVaultVersion fetches the latest Vault version from HashiCorp's releases API
func getLatestVaultVersion() (string, error) {
	apiURL := "https://api.releases.hashicorp.com/v1/releases/vault"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "MonoKit-VaultHealth/1.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch release information: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("releases API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var releases []struct {
		Version      string `json:"version"`
		IsPrerelease bool   `json:"is_prerelease"`
	}

	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("failed to parse releases JSON: %w", err)
	}

	// Find the latest stable release (not prerelease)
	for _, release := range releases {
		if !release.IsPrerelease && release.Version != "" {
			// Remove 'v' prefix if present
			version := strings.TrimPrefix(release.Version, "v")
			return version, nil
		}
	}

	return "", fmt.Errorf("no stable releases found")
}

// isNewerVersion compares two semantic version strings and returns true if newVer > currentVer
func isNewerVersion(newVer, currentVer string) (bool, error) {
	// Simple semantic version comparison
	// This handles basic cases like "1.15.0" vs "1.14.2"

	newParts, err := parseVersion(newVer)
	if err != nil {
		return false, fmt.Errorf("failed to parse new version %s: %w", newVer, err)
	}

	currentParts, err := parseVersion(currentVer)
	if err != nil {
		return false, fmt.Errorf("failed to parse current version %s: %w", currentVer, err)
	}

	// Compare major, minor, patch versions
	for i := 0; i < 3 && i < len(newParts) && i < len(currentParts); i++ {
		if newParts[i] > currentParts[i] {
			return true, nil
		} else if newParts[i] < currentParts[i] {
			return false, nil
		}
	}

	// Versions are equal
	return false, nil
}

// parseVersion parses a semantic version string into major, minor, patch integers
func parseVersion(version string) ([]int, error) {
	// Remove any 'v' prefix and clean the version string
	version = strings.TrimPrefix(version, "v")

	// Split by dots and parse each part
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}

	var versionParts []int
	for i, part := range parts {
		if i >= 3 {
			break // Only consider major.minor.patch
		}

		// Remove any non-numeric suffixes (like "-beta1", "+ent")
		numericPart := ""
		for _, char := range part {
			if char >= '0' && char <= '9' {
				numericPart += string(char)
			} else {
				break
			}
		}

		if numericPart == "" {
			return nil, fmt.Errorf("invalid version part: %s", part)
		}

		num, err := strconv.Atoi(numericPart)
		if err != nil {
			return nil, fmt.Errorf("failed to parse version part %s: %w", numericPart, err)
		}

		versionParts = append(versionParts, num)
	}

	// Ensure we have at least major.minor.patch (pad with zeros if needed)
	for len(versionParts) < 3 {
		versionParts = append(versionParts, 0)
	}

	return versionParts, nil
}
