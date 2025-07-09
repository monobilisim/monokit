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
	"github.com/rs/zerolog/log"
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
			log.Error().Err(err).Msg("Failed to parse journal entry")
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
			log.Error().Err(err).Msg("Failed to save last sent timestamp")
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

	log.Debug().
		Str("component", "systemd_logs").
		Str("action", "push_to_api").
		Str("api_url", clientConf.URL).
		Str("hostname", hostName).
		Int("total_entries", len(entries)).
		Msg("Starting API push for systemd logs")

	successCount := 0
	failureCount := 0

	// Send each log entry individually
	for i, entry := range entries {
		// Skip empty messages
		if entry.Message == "" {
			log.Debug().
				Str("component", "systemd_logs").
				Str("action", "push_to_api").
				Int("entry_index", i).
				Msg("Skipping entry with empty message")
			continue
		}

		// Convert metadata map to JSON string
		metadataBytes, err := json.Marshal(entry.Metadata)
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "systemd_logs").
				Str("action", "marshal_metadata").
				Int("entry_index", i).
				Msg("Failed to marshal metadata for log entry")
			failureCount++
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
		if log.Debug().Enabled() {
			requestJson, _ := json.Marshal(logRequest)
			log.Debug().
				Str("component", "systemd_logs").
				Str("action", "push_to_api").
				Int("entry_index", i).
				RawJSON("request", requestJson).
				Msg("Sending log request to API")
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(logRequest)
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "systemd_logs").
				Str("action", "marshal_request").
				Int("entry_index", i).
				Msg("Failed to marshal log request")
			failureCount++
			continue
		}

		// Send log to API
		req, err := http.NewRequest("POST", clientConf.URL+"/api/v1/host/logs", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "systemd_logs").
				Str("action", "create_request").
				Int("entry_index", i).
				Msg("Failed to create HTTP request")
			failureCount++
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
			log.Error().
				Err(err).
				Str("component", "systemd_logs").
				Str("action", "send_request").
				Int("entry_index", i).
				Str("url", clientConf.URL+"/api/v1/host/logs").
				Msg("Failed to send HTTP request to API")
			failureCount++
			continue
		}

		// Check response status
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			// Read response body for more details
			respBody, _ := io.ReadAll(resp.Body)
			log.Error().
				Str("component", "systemd_logs").
				Str("action", "api_response").
				Int("entry_index", i).
				Int("status_code", resp.StatusCode).
				Str("response_body", string(respBody)).
				Str("log_level", logRequest.Level).
				Str("log_message", logRequest.Message).
				Msg("API rejected log entry")
			failureCount++
		} else {
			successCount++
		}

		resp.Body.Close()
	}

	log.Debug().
		Str("component", "systemd_logs").
		Str("action", "push_to_api").
		Int("total_entries", len(entries)).
		Int("success_count", successCount).
		Int("failure_count", failureCount).
		Float64("success_rate", float64(successCount)/float64(len(entries))*100).
		Msg("Completed pushing systemd logs to API")

	return nil
}

// SystemdLogs collects systemd logs and pushes them to the API
func SystemdLogs() {
	// Skip if systemd is not available
	if !hasSystemd() {
		return
	}

	// Skip if client configuration does not exist
	if !common.ConfExists("client") {
		return
	}

	startTime := time.Now()
	common.SplitSection("Systemd Logs")
	fmt.Println("Collecting systemd logs...")

	log.Debug().
		Str("component", "systemd_logs").
		Str("action", "start").
		Msg("Starting systemd log collection and submission")

	// Get the timestamp of the last sent log
	lastTimestamp, err := getLastSentTimestamp()
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "systemd_logs").
			Str("action", "get_last_timestamp").
			Dur("fallback_hours", 1*time.Hour).
			Msg("Failed to get last sent timestamp, using default fallback")
		// We continue with the default timestamp (1 hour ago)
	}

	log.Debug().
		Str("component", "systemd_logs").
		Str("action", "collect").
		Time("since", lastTimestamp).
		Int("max_entries", 1000).
		Msg("Collecting systemd logs since last timestamp")

	fmt.Printf("Collecting logs since %s\n", lastTimestamp.Format(time.RFC3339))

	// Collect logs since the last sent timestamp, max 1000 entries
	entries, err := collectSystemdLogs(lastTimestamp, 1000)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "systemd_logs").
			Str("action", "collect").
			Time("since", lastTimestamp).
			Msg("Failed to collect systemd logs")
		return
	}

	log.Debug().
		Str("component", "systemd_logs").
		Str("action", "collect").
		Int("entries_collected", len(entries)).
		Time("since", lastTimestamp).
		Dur("collection_duration", time.Since(startTime)).
		Msg("Systemd log collection completed")

	fmt.Printf("Collected %d systemd log entries\n", len(entries))

	// Push logs to API
	if len(entries) > 0 {
		pushStartTime := time.Now()
		err = pushLogsToAPI(entries)
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "systemd_logs").
				Str("action", "push_to_api").
				Int("entries_count", len(entries)).
				Msg("Failed to push systemd logs to API")
			return
		}

		log.Debug().
			Str("component", "systemd_logs").
			Str("action", "push_to_api").
			Int("entries_pushed", len(entries)).
			Dur("push_duration", time.Since(pushStartTime)).
			Dur("total_duration", time.Since(startTime)).
			Msg("Successfully pushed systemd logs to API")

		fmt.Println("Successfully pushed systemd logs to API")
	} else {
		log.Debug().
			Str("component", "systemd_logs").
			Str("action", "push_to_api").
			Msg("No systemd logs to push")
		fmt.Println("No systemd logs to push")
	}
}
