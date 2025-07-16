package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func getLogFilePath() string {
	// Use same logic as common/main.go InitZerolog
	if os.Geteuid() != 0 {
		// User mode
		xdgStateHome := os.Getenv("XDG_STATE_HOME")
		if xdgStateHome == "" {
			xdgStateHome = os.Getenv("HOME") + "/.local/state"
		}
		return filepath.Join(xdgStateHome, "monokit", "monokit.log")
	}
	// System mode
	return "/var/log/monokit.log"
}

func parseLogFile(filePath string) ([]LogEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry, err := parseLogEntry(line)
		if err != nil {
			continue // Skip invalid lines
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

func parseLogEntry(line string) (LogEntry, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return LogEntry{}, err
	}

	entry := LogEntry{Raw: raw}

	// Parse timestamp
	entry.Timestamp = parseTimestamp(raw["timestamp"])

	// Parse string fields
	fieldMap := map[string]*string{
		"level":     &entry.Level,
		"message":   &entry.Message,
		"component": &entry.Component,
		"version":   &entry.Version,
		"operation": &entry.Operation,
		"action":    &entry.Action,
		"type":      &entry.Type,
	}

	for key, field := range fieldMap {
		*field = getStringField(raw, key)
	}

	return entry, nil
}

func parseTimestamp(value interface{}) time.Time {
	tsStr, ok := value.(string)
	if !ok {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

func getStringField(raw map[string]interface{}, key string) string {
	value, ok := raw[key].(string)
	if !ok {
		return ""
	}
	return value
}
