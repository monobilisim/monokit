//go:build linux

package vaultHealth

import "time"

// VaultHealthProvider implements the health.Provider interface
type VaultHealthProvider struct{}

// Name returns the name of the provider
func (p *VaultHealthProvider) Name() string {
	return "vaultHealth"
}

// Collect gathers Vault health data.
// The 'hostname' parameter is ignored for vaultHealth as it collects local data.
func (p *VaultHealthProvider) Collect(_ string) (interface{}, error) {
	return collectVaultHealthData()
}

// VaultConfig holds configuration for Vault health checks
type VaultConfig struct {
	Vault struct {
		Address string `yaml:"address"`
		Token   string `yaml:"token"`
		Tls     struct {
			Verify    bool   `yaml:"verify"`
			Ca_cert   string `yaml:"ca_cert"`
			Cert_file string `yaml:"cert_file"`
			Key_file  string `yaml:"key_file"`
		} `yaml:"tls"`
		Limits struct {
			Max_response_time     string `yaml:"max_response_time"`
			Health_check_interval string `yaml:"health_check_interval"`
		} `yaml:"limits"`
		Alerts struct {
			Sealed_vault    bool `yaml:"sealed_vault"`
			Leader_changes  bool `yaml:"leader_changes"`
			Version_updates bool `yaml:"version_updates"`
		} `yaml:"alerts"`
		ClusterChecks struct {
			Enabled             bool `yaml:"enabled"`             // Enable detailed cluster checks
			Check_configuration bool `yaml:"check_configuration"` // Check Raft configuration
			Check_node_health   bool `yaml:"check_node_health"`   // Check individual node health
			Check_quorum        bool `yaml:"check_quorum"`        // Validate cluster quorum
			Check_performance   bool `yaml:"check_performance"`   // Monitor cluster performance
			Check_metrics       bool `yaml:"check_metrics"`       // Access Prometheus metrics
		} `yaml:"cluster_checks"`
	} `yaml:"vault"`
	Alarm struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"alarm"`
}

// VaultHealthData represents comprehensive Vault health information
type VaultHealthData struct {
	Version     string          `json:"version"`
	LastChecked string          `json:"last_checked"`
	Connection  ConnectionInfo  `json:"connection"`
	Service     ServiceInfo     `json:"service"`
	Cluster     ClusterInfo     `json:"cluster"`
	Seal        SealInfo        `json:"seal"`
	Replication ReplicationInfo `json:"replication"`
	Storage     StorageInfo     `json:"storage"`
	VersionInfo VersionInfo     `json:"version_info"`
}

// ConnectionInfo contains Vault API connection information
type ConnectionInfo struct {
	Address    string `json:"address"`
	Connected  bool   `json:"connected"`
	Error      string `json:"error,omitempty"`
	TLSEnabled bool   `json:"tls_enabled"`
	Healthy    bool   `json:"healthy"`
}

// ServiceInfo contains Vault service status information
type ServiceInfo struct {
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	Status    string `json:"status"`
	Enabled   bool   `json:"enabled"`
}

// ClusterInfo contains Vault cluster status information
type ClusterInfo struct {
	HAEnabled        bool        `json:"ha_enabled"`
	IsLeader         bool        `json:"is_leader"`
	LeaderAddr       string      `json:"leader_addr"`
	Mode             string      `json:"mode"` // "active", "standby", "performance_standby"
	Nodes            []VaultNode `json:"nodes"`
	Healthy          bool        `json:"healthy"`
	ClusterName      string      `json:"cluster_name"`
	ClusterID        string      `json:"cluster_id"`
	FailureTolerance int         `json:"failure_tolerance"`
	HealthReason     string      `json:"health_reason,omitempty"` // Why the cluster is healthy/unhealthy
	HealthIssues     []string    `json:"health_issues,omitempty"` // List of specific health issues
}

// VaultNode represents a Vault cluster node
type VaultNode struct {
	ID           string    `json:"id"`
	Address      string    `json:"address"`
	Status       string    `json:"status"`
	LastContact  string    `json:"last_contact"`
	Version      string    `json:"version"`
	Healthy      bool      `json:"healthy"`
	NodeType     string    `json:"node_type"` // "voter", "non-voter"
	ActiveSince  time.Time `json:"active_since,omitempty"`
	HealthReason string    `json:"health_reason,omitempty"` // Why the node is unhealthy
	Issues       []string  `json:"issues,omitempty"`        // List of specific issues
	LastSeen     string    `json:"last_seen,omitempty"`     // Human-readable last contact time
}

// SealInfo contains Vault seal status information
type SealInfo struct {
	Sealed      bool   `json:"sealed"`
	SealType    string `json:"seal_type"`
	Threshold   int    `json:"threshold"`
	Shares      int    `json:"shares"`
	Initialized bool   `json:"initialized"`
	Progress    int    `json:"progress"`
	Nonce       string `json:"nonce,omitempty"`
	Migration   bool   `json:"migration"`
}

// ReplicationInfo contains Vault replication status (Enterprise only)
type ReplicationInfo struct {
	Enabled          bool     `json:"enabled"`
	Mode             string   `json:"mode"` // "primary", "secondary", "disabled"
	Status           string   `json:"status"`
	ClusterID        string   `json:"cluster_id"`
	DRMode           string   `json:"dr_mode"`
	PerformanceMode  string   `json:"performance_mode"`
	LastWAL          int64    `json:"last_wal,omitempty"`
	KnownSecondaries []string `json:"known_secondaries,omitempty"`
	ConnectionState  string   `json:"connection_state,omitempty"`
}

// StorageInfo contains Vault storage backend information
type StorageInfo struct {
	Type     string    `json:"type"` // "raft", "consul", "file", etc.
	RaftInfo *RaftInfo `json:"raft_info,omitempty"`
}

// RaftInfo contains Raft-specific storage information
type RaftInfo struct {
	LeaderAddr           string   `json:"leader_addr"`
	AppliedIndex         int64    `json:"applied_index"`
	CommittedIndex       int64    `json:"committed_index"`
	FailureTolerance     int      `json:"failure_tolerance"`
	OptimisticFailureTol int      `json:"optimistic_failure_tolerance"`
	Voters               []string `json:"voters"`
}

// VersionInfo contains Vault version information
type VersionInfo struct {
	Version       string `json:"version"`
	BuildDate     string `json:"build_date"`
	NeedsUpdate   bool   `json:"needs_update"`
	UpdateMessage string `json:"update_message"`
}

// API Response structures for Vault endpoints

// HealthResponse represents /sys/health API response
type HealthResponse struct {
	Initialized                bool   `json:"initialized"`
	Sealed                     bool   `json:"sealed"`
	Standby                    bool   `json:"standby"`
	PerformanceStandby         bool   `json:"performance_standby"`
	ReplicationPerformanceMode string `json:"replication_performance_mode"`
	ReplicationDRMode          string `json:"replication_dr_mode"`
	ServerTimeUTC              int64  `json:"server_time_utc"`
	Version                    string `json:"version"`
	ClusterName                string `json:"cluster_name"`
	ClusterID                  string `json:"cluster_id"`
	HAConnection               bool   `json:"ha_connection_healthy"`
}

// LeaderResponse represents /sys/leader API response
type LeaderResponse struct {
	HAEnabled                       bool   `json:"ha_enabled"`
	IsSelf                          bool   `json:"is_self"`
	LeaderAddress                   string `json:"leader_address"`
	LeaderClusterAddress            string `json:"leader_cluster_address"`
	PerformanceStandby              bool   `json:"performance_standby"`
	PerformanceStandbyLastRemoteWAL int64  `json:"performance_standby_last_remote_wal"`
	ActiveTime                      string `json:"active_time"`
	RaftCommittedIndex              int64  `json:"raft_committed_index"`
	RaftAppliedIndex                int64  `json:"raft_applied_index"`
}

// SealStatusResponse represents /sys/seal-status API response
type SealStatusResponse struct {
	Type         string `json:"type"`
	Initialized  bool   `json:"initialized"`
	Sealed       bool   `json:"sealed"`
	T            int    `json:"t"`
	N            int    `json:"n"`
	Progress     int    `json:"progress"`
	Nonce        string `json:"nonce"`
	Version      string `json:"version"`
	BuildDate    string `json:"build_date"`
	Migration    bool   `json:"migration"`
	ClusterName  string `json:"cluster_name"`
	ClusterID    string `json:"cluster_id"`
	RecoverySeal bool   `json:"recovery_seal"`
	StorageType  string `json:"storage_type"`
}

// RaftStateResponse represents /sys/storage/raft/autopilot/state API response
type RaftStateResponse struct {
	Healthy                    bool                  `json:"healthy"`
	FailureTolerance           int                   `json:"failure_tolerance"`
	OptimisticFailureTolerance int                   `json:"optimistic_failure_tolerance"`
	Leader                     string                `json:"leader"`
	Voters                     []string              `json:"voters"`
	Servers                    map[string]RaftServer `json:"servers"`
}

// RaftServer represents a server in the Raft cluster state
type RaftServer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	NodeStatus  string `json:"node_status"`
	Healthy     bool   `json:"healthy"`
	LastContact string `json:"last_contact"`
	LastTerm    int64  `json:"last_term"`
	LastIndex   int64  `json:"last_index"`
	Status      string `json:"status"`
	Version     string `json:"version"`
	NodeType    string `json:"node_type"`
	StableSince string `json:"stable_since"`
}

// ReplicationStatusResponse represents /sys/replication/performance/status API response
type ReplicationStatusResponse struct {
	Mode                     string            `json:"mode"`
	ClusterID                string            `json:"cluster_id"`
	State                    string            `json:"state"`
	LastWAL                  int64             `json:"last_wal"`
	LastRemoteWAL            int64             `json:"last_remote_wal"`
	ConnectionState          string            `json:"connection_state"`
	KnownSecondaries         []string          `json:"known_secondaries"`
	Primaries                []ReplicationNode `json:"primaries"`
	Secondaries              []ReplicationNode `json:"secondaries"`
	KnownPrimaryClusterAddrs []string          `json:"known_primary_cluster_addrs"`
	PrimaryClusterAddr       string            `json:"primary_cluster_addr"`
	SecondaryID              string            `json:"secondary_id"`
	SSCTGenerationCounter    int               `json:"ssct_generation_counter"`
}

// ReplicationNode represents a node in replication status
type ReplicationNode struct {
	APIAddress                    string `json:"api_address"`
	ClusterAddress                string `json:"cluster_address"`
	ConnectionStatus              string `json:"connection_status"`
	LastHeartbeat                 string `json:"last_heartbeat"`
	NodeID                        string `json:"node_id"`
	ReplicationPrimaryCanaryAgeMS string `json:"replication_primary_canary_age_ms"`
}
