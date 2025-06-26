package lbPolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/monobilisim/monokit/common"
)

// PatroniMonitor handles Patroni-based automatic switching
type PatroniMonitor struct {
	config      PatroniAutoSwitchConfig
	client      *http.Client
	running     bool
	mutex       sync.RWMutex
	lastPrimary map[string]string // cluster -> primary node name
	stopChan    chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewPatroniMonitor creates a new Patroni monitor instance
func NewPatroniMonitor(config PatroniAutoSwitchConfig) *PatroniMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &PatroniMonitor{
		config:      config,
		client:      &http.Client{Timeout: 30 * time.Second},
		running:     false,
		lastPrimary: make(map[string]string),
		stopChan:    make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins monitoring Patroni clusters
func (pm *PatroniMonitor) Start() error {
	if err := pm.config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if !pm.config.Enabled {
		return fmt.Errorf("patroni auto-switch is disabled")
	}

	pm.mutex.Lock()
	if pm.running {
		pm.mutex.Unlock()
		return fmt.Errorf("monitor is already running")
	}
	pm.running = true
	pm.mutex.Unlock()

	common.LogInfo("Patroni monitor starting...")

	// Initialize last known primaries
	for _, mapping := range pm.config.Mappings {
		primary, err := pm.CheckClusterPrimary(mapping)
		if err != nil {
			common.LogWarn(fmt.Sprintf("Failed to get initial primary for cluster %s: %v", mapping.Cluster, err))
			continue
		}
		pm.lastPrimary[mapping.Cluster] = primary
		common.LogInfo(fmt.Sprintf("Initial primary for cluster %s: %s", mapping.Cluster, primary))
	}

	// Start monitoring in a goroutine
	go pm.monitorLoop()

	return nil
}

// Stop halts the monitoring process
func (pm *PatroniMonitor) Stop() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.running {
		return
	}

	common.LogInfo("Stopping Patroni monitor...")
	pm.running = false
	pm.cancel()
	close(pm.stopChan)
}

// monitorLoop is the main monitoring loop
func (pm *PatroniMonitor) monitorLoop() {
	ticker := time.NewTicker(pm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			common.LogInfo("Monitor loop stopped")
			return
		case <-ticker.C:
			pm.checkAllClusters()
		}
	}
}

// checkAllClusters checks all configured Patroni clusters
func (pm *PatroniMonitor) checkAllClusters() {
	for _, mapping := range pm.config.Mappings {
		if err := pm.checkSingleCluster(mapping); err != nil {
			common.LogError(fmt.Sprintf("Error checking cluster %s: %v", mapping.Cluster, err))
		}
	}
}

// checkSingleCluster checks a single Patroni cluster for changes
func (pm *PatroniMonitor) checkSingleCluster(mapping PatroniMapping) error {
	primary, err := pm.CheckClusterPrimary(mapping)
	if err != nil {
		return fmt.Errorf("failed to get primary for cluster %s: %w", mapping.Cluster, err)
	}

	lastPrimary, exists := pm.lastPrimary[mapping.Cluster]

	// Check if primary has changed
	if !exists || lastPrimary != primary {
		common.LogInfo(fmt.Sprintf("Primary change detected for cluster %s: %s -> %s",
			mapping.Cluster, lastPrimary, primary))

		// Get the switch target for this primary node
		switchTarget := mapping.GetSwitchTargetForNode(primary)

		if switchTarget == "" {
			common.LogWarn(fmt.Sprintf("No switch target found for primary %s in cluster %s",
				primary, mapping.Cluster))
			return nil
		}

		common.LogInfo(fmt.Sprintf("Triggering switch to %s for primary %s", switchTarget, primary))

		if pm.config.DryRun {
			common.LogInfo(fmt.Sprintf("DRY RUN: Would switch to %s", switchTarget))
		} else {
			if err := pm.performSwitch(switchTarget); err != nil {
				return fmt.Errorf("failed to perform switch: %w", err)
			}
		}

		// Update last known primary
		pm.lastPrimary[mapping.Cluster] = primary

		// Send alarm if configured
		pm.sendAlarm(mapping.Cluster, lastPrimary, primary, switchTarget)
	}

	return nil
}

// CheckClusterPrimary checks the primary node of a Patroni cluster concurrently (exported for main.go)
func (pm *PatroniMonitor) CheckClusterPrimary(mapping PatroniMapping) (string, error) {
	if len(mapping.PatroniUrls) == 0 {
		return "", fmt.Errorf("no Patroni URLs configured for cluster %s", mapping.Cluster)
	}

	// Create channels for results
	type result struct {
		primary string
		err     error
		url     string
	}

	results := make(chan result, len(mapping.PatroniUrls))
	ctx, cancel := context.WithTimeout(pm.ctx, mapping.Timeout)
	defer cancel()

	// Start concurrent checks
	for _, patroniUrl := range mapping.PatroniUrls {
		go func(url string) {
			primary, err := pm.checkSinglePatroniURL(ctx, url)
			results <- result{primary: primary, err: err, url: url}
		}(patroniUrl)
	}

	// Collect results - return first success
	var allErrors []string

	for i := 0; i < len(mapping.PatroniUrls); i++ {
		select {
		case res := <-results:
			if res.err == nil && res.primary != "" {
				common.LogDebug(fmt.Sprintf("Successfully got primary from %s: %s", res.url, res.primary))
				return res.primary, nil
			}
			allErrors = append(allErrors, fmt.Sprintf("%s: %v", res.url, res.err))
		case <-ctx.Done():
			return "", fmt.Errorf("timeout checking Patroni URLs for cluster %s", mapping.Cluster)
		}
	}

	return "", fmt.Errorf("failed to check all Patroni URLs for cluster %s: [%s]",
		mapping.Cluster, strings.Join(allErrors, "; "))
}

// checkSinglePatroniURL checks a single Patroni URL for primary node
func (pm *PatroniMonitor) checkSinglePatroniURL(ctx context.Context, patroniUrl string) (string, error) {
	url := fmt.Sprintf("%s/cluster", patroniUrl)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := pm.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var clusterStatus PatroniClusterStatus
	if err := json.NewDecoder(resp.Body).Decode(&clusterStatus); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	primary := clusterStatus.GetPrimaryNode()
	if primary == nil {
		return "", fmt.Errorf("no primary node found")
	}

	return primary.Name, nil
}

// performSwitch executes the switch using existing lbPolicy mechanism
func (pm *PatroniMonitor) performSwitch(switchTarget string) error {
	common.LogInfo(fmt.Sprintf("Performing switch to: %s", switchTarget))

	// Call the existing SwitchMain function
	SwitchMain(switchTarget)

	return nil
}

// sendAlarm sends a notification about the primary change
func (pm *PatroniMonitor) sendAlarm(cluster, oldPrimary, newPrimary, switchTarget string) {
	message := fmt.Sprintf("Patroni cluster %s primary changed: %s -> %s (switched to %s)",
		cluster, oldPrimary, newPrimary, switchTarget)

	// Log for traceability
	common.LogInfo(fmt.Sprintf("Sending alarm: %s", message))

	// Use the existing lbPolicy alarm helper which wraps common.Alarm
	AlarmCustom("switch", message)
}

// GetClusterStatus returns the full cluster status for a mapping using concurrent checks
func (pm *PatroniMonitor) GetClusterStatus(mapping PatroniMapping) (*PatroniClusterStatus, error) {
	if len(mapping.PatroniUrls) == 0 {
		return nil, fmt.Errorf("no Patroni URLs configured for cluster %s", mapping.Cluster)
	}

	// Create channels for results
	type statusResult struct {
		status *PatroniClusterStatus
		err    error
		url    string
	}

	results := make(chan statusResult, len(mapping.PatroniUrls))
	ctx, cancel := context.WithTimeout(pm.ctx, mapping.Timeout)
	defer cancel()

	// Start concurrent checks
	for _, patroniUrl := range mapping.PatroniUrls {
		go func(url string) {
			status, err := pm.getClusterStatusFromURL(ctx, url)
			results <- statusResult{status: status, err: err, url: url}
		}(patroniUrl)
	}

	// Collect results - return first success
	var allErrors []string

	for i := 0; i < len(mapping.PatroniUrls); i++ {
		select {
		case res := <-results:
			if res.err == nil && res.status != nil {
				common.LogDebug(fmt.Sprintf("Successfully got cluster status from %s", res.url))
				return res.status, nil
			}
			allErrors = append(allErrors, fmt.Sprintf("%s: %v", res.url, res.err))
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout getting cluster status for %s", mapping.Cluster)
		}
	}

	return nil, fmt.Errorf("failed to get cluster status from all Patroni URLs for %s: [%s]",
		mapping.Cluster, strings.Join(allErrors, "; "))
}

// getClusterStatusFromURL gets cluster status from a single URL
func (pm *PatroniMonitor) getClusterStatusFromURL(ctx context.Context, patroniUrl string) (*PatroniClusterStatus, error) {
	url := fmt.Sprintf("%s/cluster", patroniUrl)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := pm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var clusterStatus PatroniClusterStatus
	if err := json.NewDecoder(resp.Body).Decode(&clusterStatus); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &clusterStatus, nil
}

// IsRunning returns whether the monitor is currently running
func (pm *PatroniMonitor) IsRunning() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.running
}

// GetLastPrimary returns the last known primary for a cluster
func (pm *PatroniMonitor) GetLastPrimary(cluster string) (string, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	primary, exists := pm.lastPrimary[cluster]
	return primary, exists
}

// GetStats returns monitoring statistics
func (pm *PatroniMonitor) GetStats() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	return map[string]interface{}{
		"running":        pm.running,
		"clusters":       len(pm.config.Mappings),
		"check_interval": pm.config.CheckInterval.String(),
		"dry_run":        pm.config.DryRun,
		"last_primaries": pm.lastPrimary,
	}
}
