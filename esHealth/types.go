package esHealth

import "time"

// Config holds the configuration for esHealth, loaded from es.yml
// This var will be initialized in main.go by common.ConfInit
var Config struct {
	Api_url  string `mapstructure:"es_health_api_url"`
	User     string `mapstructure:"es_health_user,omitempty"`
	Password string `mapstructure:"es_health_password,omitempty"`
}

// ClusterHealthAPIResponse mirrors the relevant parts of Elasticsearch's /_cluster/health response
type ClusterHealthAPIResponse struct {
	ClusterName                 string  `json:"cluster_name"`
	Status                      string  `json:"status"` // green, yellow, red
	TimedOut                    bool    `json:"timed_out"`
	NumberOfNodes               int     `json:"number_of_nodes"`
	NumberOfDataNodes           int     `json:"number_of_data_nodes"`
	ActivePrimaryShards         int     `json:"active_primary_shards"`
	ActiveShards                int     `json:"active_shards"`
	RelocatingShards            int     `json:"relocating_shards"`
	InitializingShards          int     `json:"initializing_shards"`
	UnassignedShards            int     `json:"unassigned_shards"`
	DelayedUnassignedShards     int     `json:"delayed_unassigned_shards"`
	NumberOfPendingTasks        int     `json:"number_of_pending_tasks"`
	NumberOfInFlightFetch       int     `json:"number_of_in_flight_fetch"`
	TaskMaxWaitingInQueueMillis int     `json:"task_max_waiting_in_queue_millis"`
	ActiveShardsPercentAsNumber float64 `json:"active_shards_percent_as_number"`
}

// ShardAllocationAPIResponse represents the response from the /_cluster/allocation/explain API
// This is used when there are unassigned shards.
type ShardAllocationAPIResponse struct {
	Index               string             `json:"index"`
	Shard               int                `json:"shard"`
	Primary             bool               `json:"primary"`
	CurrentState        string             `json:"current_state"`
	UnassignedInfo      *UnassignedInfoAPI `json:"unassigned_info,omitempty"`
	CanAllocate         string             `json:"can_allocate"` // "no", "yes", "throttle"
	AllocateExplanation string             `json:"allocate_explanation"`
}

// UnassignedInfoAPI contains information about why a shard is unassigned
type UnassignedInfoAPI struct {
	Reason               string    `json:"reason"`
	At                   time.Time `json:"at"`
	LastAllocationStatus string    `json:"last_allocation_status,omitempty"`
	Details              string    `json:"details,omitempty"`
}

// --- UI Data Structures ---

// EsHealthData holds all collected Elasticsearch health information for UI rendering
type EsHealthData struct {
	ClusterName string
	Status      string // green, yellow, red
	NodeStats   NodeStatsInfo
	ShardStats  ShardStatsInfo
	Allocation  *AllocationInfo // Pointer because it might be nil if no issues or not checked
	LastChecked string
	Error       string // To store any general error messages during data collection
}

// NodeStatsInfo holds information about cluster nodes
type NodeStatsInfo struct {
	TotalDataNodes int
	TotalNodes     int
}

// ShardStatsInfo holds information about cluster shards
type ShardStatsInfo struct {
	ActivePrimary int
	Active        int
	Relocating    int
	Initializing  int
	Unassigned    int
	ActivePercent float64
}

// AllocationInfo holds information about shard allocation issues
type AllocationInfo struct {
	CanAllocate      string // "no", "yes", "throttle", or "OK" if no issues
	Explanation      string
	Index            string // Relevant if a specific shard is problematic
	Shard            int    // Relevant if a specific shard is problematic
	Primary          bool   // Relevant if a specific shard is problematic
	CurrentState     string // Relevant if a specific shard is problematic
	UnassignedReason string
	UnassignedAt     string
	IsProblematic    bool // True if CanAllocate is "no" or "throttle" or UnassignedInfo is present
}
