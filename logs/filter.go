package logs

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

var timeFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func (f *LogFilter) matches(entry LogEntry) bool {
	checks := []func() bool{
		func() bool { return f.From == nil || !entry.Timestamp.Before(*f.From) },
		func() bool { return f.To == nil || !entry.Timestamp.After(*f.To) },
		func() bool { return f.Component == "" || wildcardMatch(entry.Component, f.Component) },
		func() bool { return f.Operation == "" || wildcardMatch(entry.Operation, f.Operation) },
		func() bool { return f.Action == "" || wildcardMatch(entry.Action, f.Action) },
		func() bool { return f.Type == "" || wildcardMatch(entry.Level, f.Type) },
		func() bool { return f.FromVersion == "" || wildcardMatch(entry.Version, f.FromVersion) },
		func() bool { return f.matchesArbitraryFields(entry) },
	}

	for _, check := range checks {
		if !check() {
			return false
		}
	}
	return true
}

func (f *LogFilter) matchesArbitraryFields(entry LogEntry) bool {
	if f.Fields == nil {
		return true
	}

	for key, expectedValue := range f.Fields {
		if rawValue, exists := entry.Raw[key]; exists {
			// Convert the raw value to string for comparison
			actualValue := fmt.Sprintf("%v", rawValue)
			if !wildcardMatch(actualValue, expectedValue) {
				return false
			}
		} else {
			// If the field doesn't exist in the entry, it doesn't match
			return false
		}
	}
	return true
}

// wildcardMatch performs glob pattern matching with escape support
func wildcardMatch(text, pattern string) bool {
	if pattern == "" {
		return true
	}

	// Handle escaped wildcards by temporarily replacing them
	escapedPattern := escapeWildcards(pattern)

	// Use filepath.Match for glob pattern matching
	matched, err := filepath.Match(escapedPattern, text)
	if err != nil {
		// If pattern is invalid, fall back to exact match
		return text == pattern
	}

	return matched
}

// escapeWildcards handles escaped wildcards by temporarily replacing them
func escapeWildcards(pattern string) string {
	var result strings.Builder
	escaped := false

	for i := 0; i < len(pattern); i++ {
		char := pattern[i]

		if escaped {
			// If escaped, treat the character literally
			result.WriteByte(char)
			escaped = false
		} else if char == '\\' {
			// Start escape sequence
			escaped = true
		} else {
			// Regular character
			result.WriteByte(char)
		}
	}

	return result.String()
}

func parseTimeFilter(timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}

	for _, format := range timeFormats {
		if parsed, err := time.Parse(format, timeStr); err == nil {
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("invalid time format: %s", timeStr)
}

func filterLogs(entries []LogEntry, filter LogFilter) []LogEntry {
	var filtered []LogEntry
	for _, entry := range entries {
		if filter.matches(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
