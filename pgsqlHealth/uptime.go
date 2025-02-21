// This file implements the uptime calculation for PostgreSQL
//
// It provides the following functions:
// - uptime(): Calculates the uptime of the PostgreSQL server
package pgsqlHealth

import (
	"fmt"
	"github.com/monobilisim/monokit/common"
)

// uptime calculates the uptime of the PostgreSQL server
// by subtracting the start time of the postmaster from the current time
// and converting the result to days, hours, minutes, and seconds
func uptime() {
	var result int
	query := `SELECT EXTRACT(EPOCH FROM current_timestamp - pg_postmaster_start_time())::int AS uptime_seconds`
	err := Connection.QueryRow(query).Scan(&result)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		common.PrettyPrintStr("PostgreSQL uptime", false, "accessible")
		return
	}
	days := result / (24 * 3600)
	hours := (result % (24 * 3600)) / 3600
	minutes := (result % 3600) / 60
	seconds := result % 60

	common.PrettyPrintStr("PostgreSQL uptime", true, fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds))
} 