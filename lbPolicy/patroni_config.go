package lbPolicy

import (
	"fmt"
	"strings"
	"time"
)

// PatroniAutoSwitchConfig represents the configuration for Patroni-based automatic switching
type PatroniAutoSwitchConfig struct {
	Enabled       bool             `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	CheckInterval time.Duration    `mapstructure:"check_interval" yaml:"check_interval" json:"check_interval"`
	Mappings      []PatroniMapping `mapstructure:"mappings" yaml:"mappings" json:"mappings"`
	DryRun        bool             `mapstructure:"dry_run" yaml:"dry_run" json:"dry_run"`
}

// PatroniMapping represents a mapping between a Patroni cluster and switch targets
type PatroniMapping struct {
	Cluster     string        `mapstructure:"cluster" yaml:"cluster" json:"cluster"`                // e.g., "test-pgsql"
	NodeMap     []string      `mapstructure:"nodemap" yaml:"nodemap" json:"nodemap"`                // e.g., ["test-pgsql-11:first_dc1", "test-pgsql-21:first_dc2"]
	SwitchTo    string        `mapstructure:"switch_to" yaml:"switch_to" json:"switch_to"`          // Default switch target e.g., "first_dc1"
	PatroniUrls []string      `mapstructure:"patroni_urls" yaml:"patroni_urls" json:"patroni_urls"` // e.g., ["http://pg1:8008", "http://pg2:8008"]
	Port        int           `mapstructure:"port" yaml:"port" json:"port"`                         // Default 8008
	Timeout     time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`                // Default 10s
}

// NodeMapping represents a parsed node to switch target mapping
type NodeMapping struct {
	NodeName string // e.g., "test-pgsql-11"
	SwitchTo string // e.g., "first_dc1"
}

// PatroniNodeStatus represents the status of a Patroni node
type PatroniNodeStatus struct {
	Name     string `json:"name"`
	Role     string `json:"role"`  // "leader", "replica", etc.
	State    string `json:"state"` // "running", "streaming", etc.
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Timeline int    `json:"timeline"`
	Lag      int    `json:"lag,omitempty"`
}

// PatroniClusterStatus represents the response from /cluster endpoint
type PatroniClusterStatus struct {
	Members []PatroniNodeStatus `json:"members"`
	Scope   string              `json:"scope"`
}

// GetDefaults returns default configuration values
func (config *PatroniAutoSwitchConfig) GetDefaults() *PatroniAutoSwitchConfig {
	return &PatroniAutoSwitchConfig{
		Enabled:       false,
		CheckInterval: 30 * time.Second,
		Mappings:      []PatroniMapping{},
		DryRun:        false,
	}
}

// Validate checks if the configuration is valid
func (config *PatroniAutoSwitchConfig) Validate() error {
	if !config.Enabled {
		return nil // No validation needed if disabled
	}

	if config.CheckInterval < 5*time.Second {
		return fmt.Errorf("check_interval must be at least 5 seconds")
	}

	if len(config.Mappings) == 0 {
		return fmt.Errorf("at least one mapping must be defined when enabled")
	}

	for i, mapping := range config.Mappings {
		if err := mapping.Validate(); err != nil {
			return fmt.Errorf("mapping %d: %v", i, err)
		}
	}

	return nil
}

// Validate checks if the mapping is valid
func (mapping *PatroniMapping) Validate() error {
	if mapping.Cluster == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	if len(mapping.NodeMap) == 0 {
		return fmt.Errorf("nodemap cannot be empty")
	}

	if len(mapping.PatroniUrls) == 0 {
		return fmt.Errorf("patroni_urls cannot be empty")
	}

	// Validate nodemap format
	for i, nodeMapEntry := range mapping.NodeMap {
		if !strings.Contains(nodeMapEntry, ":") {
			return fmt.Errorf("nodemap entry %d (%s) must be in format 'node:switch_target'", i, nodeMapEntry)
		}
	}

	// Set defaults
	if mapping.Port == 0 {
		mapping.Port = 8008
	}

	if mapping.Timeout == 0 {
		mapping.Timeout = 10 * time.Second
	}

	if mapping.SwitchTo == "" {
		// Use the first nodemap entry's switch target as default
		parts := strings.Split(mapping.NodeMap[0], ":")
		if len(parts) >= 2 {
			mapping.SwitchTo = parts[1]
		}
	}

	return nil
}

// ParseNodeMappings parses the nodemap into NodeMapping structs
func (mapping *PatroniMapping) ParseNodeMappings() []NodeMapping {
	var mappings []NodeMapping

	for _, nodeMapEntry := range mapping.NodeMap {
		parts := strings.Split(nodeMapEntry, ":")
		if len(parts) >= 2 {
			mappings = append(mappings, NodeMapping{
				NodeName: parts[0],
				SwitchTo: parts[1],
			})
		}
	}

	return mappings
}

// GetSwitchTargetForNode returns the switch target for a specific node
func (mapping *PatroniMapping) GetSwitchTargetForNode(nodeName string) string {
	nodeMappings := mapping.ParseNodeMappings()

	for _, nodeMapping := range nodeMappings {
		if nodeMapping.NodeName == nodeName {
			return nodeMapping.SwitchTo
		}
	}

	// Return default switch target if node not found in mapping
	return mapping.SwitchTo
}

// GetAllNodes returns all node names from the nodemap
func (mapping *PatroniMapping) GetAllNodes() []string {
	var nodes []string
	nodeMappings := mapping.ParseNodeMappings()

	for _, nodeMapping := range nodeMappings {
		nodes = append(nodes, nodeMapping.NodeName)
	}

	return nodes
}

// GetPrimaryNode returns the primary node from cluster status
func (status *PatroniClusterStatus) GetPrimaryNode() *PatroniNodeStatus {
	for _, member := range status.Members {
		if member.Role == "leader" || member.Role == "master" {
			return &member
		}
	}
	return nil
}

// GetNodeByName returns a node by its name
func (status *PatroniClusterStatus) GetNodeByName(name string) *PatroniNodeStatus {
	for _, member := range status.Members {
		if member.Name == name {
			return &member
		}
	}
	return nil
}

// GetHealthyNodes returns all healthy nodes
func (status *PatroniClusterStatus) GetHealthyNodes() []PatroniNodeStatus {
	var healthy []PatroniNodeStatus
	for _, member := range status.Members {
		if member.State == "running" || member.State == "streaming" {
			healthy = append(healthy, member)
		}
	}
	return healthy
}
