package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

func formatLogs(entries []LogEntry, options OutputOptions) (string, error) {
	if len(options.GetFields) > 0 {
		return formatGetFields(entries, options.GetFields)
	}

	if options.Ugly {
		return formatRawJSON(entries)
	}
	return formatPretty(entries), nil
}

func formatGetFields(entries []LogEntry, fields []string) (string, error) {
	var result strings.Builder

	for _, entry := range entries {
		values := make([]string, 0, len(fields))

		for _, field := range fields {
			if value, exists := entry.Raw[field]; exists {
				values = append(values, fmt.Sprintf("%v", value))
			} else {
				// Field doesn't exist, add empty string
				values = append(values, "")
			}
		}

		result.WriteString(strings.Join(values, "\t"))
		result.WriteString("\n")
	}

	return result.String(), nil
}

func formatRawJSON(entries []LogEntry) (string, error) {
	var result strings.Builder
	for _, entry := range entries {
		jsonBytes, err := json.Marshal(entry.Raw)
		if err != nil {
			return "", err
		}
		result.WriteString(string(jsonBytes))
		result.WriteString("\n")
	}
	return result.String(), nil
}

func formatPretty(entries []LogEntry) string {
	var result strings.Builder

	// Create a console writer for pretty formatting
	consoleWriter := zerolog.ConsoleWriter{
		Out:        &result,
		TimeFormat: time.RFC3339,
		NoColor:    os.Getenv("NO_COLOR") == "true" || os.Getenv("NO_COLOR") == "1",
	}

	// Write each entry's raw JSON directly to the console writer
	for _, entry := range entries {
		jsonBytes, err := json.Marshal(entry.Raw)
		if err != nil {
			// Fallback to basic formatting if JSON marshal fails
			fmt.Fprintf(&result, "%s [%s] %s\n",
				entry.Timestamp.Format(time.RFC3339),
				strings.ToUpper(entry.Level),
				entry.Message)
			continue
		}

		// Write the JSON line to the console writer
		// ConsoleWriter will automatically parse and format it
		consoleWriter.Write(jsonBytes)
		consoleWriter.Write([]byte("\n"))
	}

	return result.String()
}
