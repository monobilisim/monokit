// This file implements systemd log collection functionality
//
// It provides functions to:
// - Collect systemd logs from journald
// - Push logs to the API endpoint
//
// The main function is:
// - SystemdLogs(): Collects systemd logs and pushes them to the API

//go:build linux

package osHealth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
)

// SystemdLogEntry represents a systemd journal log entry
type SystemdLogEntry struct {
	Message   string            `json:"message"`
	Timestamp string            `json:"timestamp"`
	Priority  int               `json:"priority"`
	Metadata  map[string]string `json:"metadata"`
}

// hasSystemd checks if the system has systemd
func hasSystemd() bool {
	// Check if we're on Linux
	if runtime.GOOS != "linux" {
		return false
	}

	// Check for systemd paths
	systemdPaths := []string{
		"/run/systemd/system",
		"/sys/fs/cgroup/systemd",
	}

	for _, path := range systemdPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// getLastSentTimestamp reads the timestamp of the last sent log entry
func getLastSentTimestamp() (time.Time, error) {
	// Set default timestamp to 1 hour ago
	defaultTime := time.Now().Add(-1 * time.Hour)

	// Define the timestamp file path
	timestampFile := filepath.Join(common.TmpDir, "systemd_last_timestamp")

	// Check if the file exists
	if _, err := os.Stat(timestampFile); os.IsNotExist(err) {
		return defaultTime, nil
	}

	// Read the timestamp
	data, err := os.ReadFile(timestampFile)
	if err != nil {
		return defaultTime, err
	}

	// Parse the timestamp
	timestamp, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return defaultTime, err
	}

	return timestamp, nil
}

// saveLastSentTimestamp saves the timestamp of the last sent log entry
func saveLastSentTimestamp(timestamp time.Time) error {
	// Define the timestamp file path
	timestampFile := filepath.Join(common.TmpDir, "systemd_last_timestamp")

	// Write the timestamp
	return os.WriteFile(timestampFile, []byte(timestamp.Format(time.RFC3339)), 0644)
}

// collectSystemdLogs collects systemd logs from journald using journalctl command
func collectSystemdLogs(since time.Time, maxEntries int) ([]SystemdLogEntry, error) {
	// Format the since parameter for journalctl
	sincePeriod := since.Format("2006-01-02 15:04:05")

	// Execute journalctl command to get logs in JSON format
	cmd := exec.Command("journalctl", "-o", "json", "--no-pager", "--since", sincePeriod, "-n", fmt.Sprintf("%d", maxEntries))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute journalctl: %w", err)
	}

	// Split output into lines, each line is a JSON object
	lines := strings.Split(string(output), "\n")

	var entries []SystemdLogEntry
	var newestTimestamp time.Time
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Parse JSON entry
		var rawEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rawEntry); err != nil {
			common.LogError("Failed to parse journal entry: " + err.Error())
			continue
		}

		// Extract timestamp
		var timestamp string
		var entryTime time.Time
		if ts, ok := rawEntry["__REALTIME_TIMESTAMP"].(string); ok {
			// Convert microseconds to time
			if microSec, err := strconv.ParseInt(ts, 10, 64); err == nil {
				entryTime = time.Unix(0, microSec*1000)
				timestamp = entryTime.Format(time.RFC3339)

				// Track the newest timestamp
				if entryTime.After(newestTimestamp) {
					newestTimestamp = entryTime
				}
			} else {
				entryTime = time.Now()
				timestamp = entryTime.Format(time.RFC3339)
			}
		} else {
			entryTime = time.Now()
			timestamp = entryTime.Format(time.RFC3339)
		}

		// Extract priority
		priority := 6 // Default to INFO
		if pri, ok := rawEntry["PRIORITY"].(string); ok {
			if p, err := strconv.Atoi(pri); err == nil {
				priority = p
			}
		}

		// Extract message
		message := ""
		if msg, ok := rawEntry["MESSAGE"].(string); ok {
			message = msg
		}

		// Build metadata
		metadata := make(map[string]string)
		for k, v := range rawEntry {
			// Skip MESSAGE as it's already used
			if k != "MESSAGE" && k != "__REALTIME_TIMESTAMP" && k != "PRIORITY" {
				// Convert all values to string
				metadata[k] = fmt.Sprintf("%v", v)
			}
		}

		// Add entry to list
		entries = append(entries, SystemdLogEntry{
			Message:   message,
			Timestamp: timestamp,
			Priority:  priority,
			Metadata:  metadata,
		})
	}

	// If we found entries and have a valid newest timestamp
	if len(entries) > 0 && !newestTimestamp.IsZero() {
		// Add a small buffer (1 second) to avoid missing logs at the exact same timestamp
		newestTimestamp = newestTimestamp.Add(1 * time.Second)

		// Save the newest timestamp for next time
		err = saveLastSentTimestamp(newestTimestamp)
		if err != nil {
			common.LogError("Failed to save last sent timestamp: " + err.Error())
		}
	}

	return entries, nil
}

// convertPriorityToLevel converts a systemd priority level to a log level string
// Valid levels according to API are: "info", "warning", "error", "critical"
func convertPriorityToLevel(priority int) string {
	switch priority {
	case 0, 1:
		return "critical" // EMERG, ALERT -> critical
	case 2, 3:
		return "error" // CRIT, ERR -> error
	case 4:
		return "warning" // WARNING -> warning
	case 5, 6, 7:
		return "info" // NOTICE, INFO, DEBUG -> info
	default:
		return "info" // Default to info
	}
}

// pushLogsToAPI pushes the systemd logs to the API
func pushLogsToAPI(entries []SystemdLogEntry) error {
	// Skip if client configuration doesn't exist
	if !common.ConfExists("client") {
		return fmt.Errorf("client configuration doesn't exist")
	}

	// Load client configuration
	var clientConf struct {
		URL string
	}
	common.ConfInit("client", &clientConf)

	// Skip if URL is not configured
	if clientConf.URL == "" {
		return fmt.Errorf("client URL is not configured")
	}

	// Get hostname from global config
	hostName := common.Config.Identifier

	// Skip if hostname is not configured
	if hostName == "" {
		return fmt.Errorf("hostname is not configured")
	}

	// Try to read the host key
	hostToken := ""
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", hostName)
	if hostKey, err := os.ReadFile(keyPath); err == nil {
		hostToken = string(hostKey)
	} else {
		return fmt.Errorf("host key is not available: %w", err)
	}

	// Send each log entry individually
	for _, entry := range entries {
		// Skip empty messages
		if entry.Message == "" {
			common.LogDebug("Skipping empty message")
			continue
		}

		// Convert metadata map to JSON string
		metadataBytes, err := json.Marshal(entry.Metadata)
		if err != nil {
			common.LogError("Failed to marshal metadata: " + err.Error())
			continue
		}

		// Build API log request
		logRequest := struct {
			Level     string `json:"level"`
			Component string `json:"component"`
			Message   string `json:"message"`
			Timestamp string `json:"timestamp"`
			Metadata  string `json:"metadata"`
			Type      string `json:"type"`
		}{
			Level:     convertPriorityToLevel(entry.Priority),
			Component: "systemd",
			Message:   entry.Message,
			Timestamp: entry.Timestamp,
			Metadata:  string(metadataBytes),
			Type:      "systemd",
		}

		// Debug log the request being sent
		requestJson, _ := json.Marshal(logRequest)
		common.LogDebug("Sending log request: " + string(requestJson))

		// Marshal to JSON
		jsonData, err := json.Marshal(logRequest)
		if err != nil {
			common.LogError("Failed to marshal log request: " + err.Error())
			continue
		}

		// Send log to API
		req, err := http.NewRequest("POST", clientConf.URL+"/api/v1/host/logs", bytes.NewBuffer(jsonData))
		if err != nil {
			common.LogError("Failed to create request: " + err.Error())
			continue
		}

		// Add headers
		req.Header.Set("Authorization", hostToken)
		req.Header.Set("Content-Type", "application/json")
		common.AddUserAgent(req)

		// Send request
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			common.LogError("Failed to send request: " + err.Error())
			continue
		}

		// Check response status
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			// Read response body for more details
			respBody, _ := io.ReadAll(resp.Body)
			common.LogError(fmt.Sprintf("Failed to push log: status code %d - %s", resp.StatusCode, string(respBody)))
		}

		resp.Body.Close()
	}

	return nil
}

// SystemdLogs collects systemd logs and pushes them to the API
func SystemdLogs() {
	// Skip if systemd is not available
	if !hasSystemd() {
		common.LogInfo("Systemd not available, skipping systemd logs collection")
		return
	}

	// Skip if client configuration does not exist
	if !common.ConfExists("client") {
		common.LogInfo("Client configuration not found, skipping systemd logs collection")
		return
	}

	common.SplitSection("Systemd Logs")
	common.LogInfo("Collecting systemd logs...")

	// Get the timestamp of the last sent log
	lastTimestamp, err := getLastSentTimestamp()
	if err != nil {
		common.LogError("Failed to get last sent timestamp, using default: " + err.Error())
		// We continue with the default timestamp (1 hour ago)
	}

	common.LogInfo(fmt.Sprintf("Collecting logs since %s", lastTimestamp.Format(time.RFC3339)))

	// Collect logs since the last sent timestamp, max 1000 entries
	entries, err := collectSystemdLogs(lastTimestamp, 1000)
	if err != nil {
		common.LogError("Failed to collect systemd logs: " + err.Error())
		return
	}

	common.LogInfo(fmt.Sprintf("Collected %d systemd log entries", len(entries)))

	// Push logs to API
	if len(entries) > 0 {
		err = pushLogsToAPI(entries)
		if err != nil {
			common.LogError("Failed to push systemd logs to API: " + err.Error())
			return
		}
		common.LogInfo("Successfully pushed systemd logs to API")
	} else {
		common.LogInfo("No systemd logs to push")
	}
}
