// This file implements the uptime calculation for PostgreSQL
//
// It provides the following functions:
// - uptime(): Calculates the uptime of the PostgreSQL server
package pgsqlHealth

import (
	"fmt"

	"github.com/monobilisim/monokit/common"
)

// UptimeData holds PostgreSQL uptime information
type UptimeData struct {
	Uptime     string
	StartTime  string
	ActiveTime string
}

// uptime calculates the uptime of the PostgreSQL server
// by subtracting the start time of the postmaster from the current time
// and converting the result to days, hours, minutes, and seconds
func uptime() (*UptimeData, error) {
	var result int
	uptimeData := &UptimeData{}

	query := `SELECT EXTRACT(EPOCH FROM current_timestamp - pg_postmaster_start_time())::int AS uptime_seconds`
	err := Connection.QueryRow(query).Scan(&result)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		return nil, err
	}

	days := result / (24 * 3600)
	hours := (result % (24 * 3600)) / 3600
	minutes := (result % 3600) / 60
	seconds := result % 60

	uptimeString := fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	uptimeData.Uptime = uptimeString

	// Also get the start time of PostgreSQL
	var startTime string
	startTimeQuery := `SELECT to_char(pg_postmaster_start_time(), 'YYYY-MM-DD HH24:MI:SS')`
	err = Connection.QueryRow(startTimeQuery).Scan(&startTime)
	if err == nil {
		uptimeData.StartTime = startTime
	}

	return uptimeData, nil
}
