package logs

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type cmdFlags struct {
	from        string
	to          string
	component   string
	operation   string
	action      string
	logType     string
	fromVersion string
	fields      []string // For arbitrary key=value filtering
	getFields   []string // For extracting specific field values
	ugly        bool
}

func NewLogsCmd() *cobra.Command {
	var flags cmdFlags

	var logsCmd = &cobra.Command{
		Use:   "logs",
		Short: "Parse and filter local monokit logs",
		Long: `Parse and filter local monokit logs with various filtering options.

All string-based filters support wildcards (* and ?) for pattern matching.
Use backslash (\) to escape wildcards when you want to match them literally.

Examples:
  monokit logs --from 2025-01-15 --to 2025-01-16
  monokit logs --component sshHealth --operation notification_and_save
  monokit logs --type error --ugly
  monokit logs --from-version 8.8.1 --action all_db_failed
  monokit logs --field status_code=500 --field hostname=test
  monokit logs --field user_mode=false --component sshNotifier
  monokit logs --get error --get hostname --type error
  monokit logs --get status_code --field component=sshNotifier

Wildcard Examples:
  monokit logs -c "ssh*"                    # All components starting with "ssh"
  monokit logs -o "*notification*"          # Operations containing "notification"
  monokit logs -F "hostname=server*"        # Hostnames starting with "server"
  monokit logs -F "error=*timeout*"         # Errors containing "timeout"
  monokit logs -F "path=\\*temp\\*"         # Paths containing literal "*temp*"`,
		Run: func(cmd *cobra.Command, args []string) {
			executeLogsCmd(flags)
		},
	}

	addFlags(logsCmd, &flags)
	return logsCmd
}

func addFlags(cmd *cobra.Command, flags *cmdFlags) {
	cmd.Flags().StringVarP(&flags.from, "from", "f", "", "Filter logs from this date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVarP(&flags.to, "to", "t", "", "Filter logs to this date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVarP(&flags.component, "component", "c", "", "Filter by component name")
	cmd.Flags().StringVarP(&flags.operation, "operation", "o", "", "Filter by operation name")
	cmd.Flags().StringVarP(&flags.action, "action", "a", "", "Filter by action name")
	cmd.Flags().StringVarP(&flags.logType, "type", "l", "", "Filter by log type/level (error, warn, info, debug, trace)")
	cmd.Flags().StringVarP(&flags.fromVersion, "from-version", "v", "", "Filter by exact version match")
	cmd.Flags().StringSliceVarP(&flags.fields, "field", "F", []string{}, "Filter by arbitrary field (key=value). Can be used multiple times.")
	cmd.Flags().StringSliceVarP(&flags.getFields, "get", "g", []string{}, "Extract specific field values. Can be used multiple times. Outputs tab-separated values.")
	cmd.Flags().BoolVarP(&flags.ugly, "ugly", "u", false, "Output raw JSON instead of pretty formatted logs")
}

func executeLogsCmd(flags cmdFlags) {
	// Parse time filters
	var fromTime, toTime *time.Time
	var err error

	if flags.from != "" {
		fromTime, err = parseTimeFilter(flags.from)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing from time: %v\n", err)
			os.Exit(1)
		}
	}

	if flags.to != "" {
		toTime, err = parseTimeFilter(flags.to)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing to time: %v\n", err)
			os.Exit(1)
		}
	}

	// Parse field filters
	fieldFilters := make(map[string]string)
	for _, field := range flags.fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Invalid field format: %s (expected key=value)\n", field)
			os.Exit(1)
		}
		fieldFilters[parts[0]] = parts[1]
	}

	// Create filter
	filter := LogFilter{
		From:        fromTime,
		To:          toTime,
		Component:   flags.component,
		Operation:   flags.operation,
		Action:      flags.action,
		Type:        flags.logType,
		FromVersion: flags.fromVersion,
		Fields:      fieldFilters,
	}

	// Get log file path
	logPath := getLogFilePath()

	// Parse log file
	entries, err := parseLogFile(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing log file: %v\n", err)
		os.Exit(1)
	}

	// Filter logs
	filteredEntries := filterLogs(entries, filter)

	// Format and output
	options := OutputOptions{
		Ugly:      flags.ugly,
		GetFields: flags.getFields,
	}
	output, err := formatLogs(filteredEntries, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting logs: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(output)
}
