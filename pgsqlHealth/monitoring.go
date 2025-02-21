// This file implements PostgreSQL monitoring functionality
//
// It provides functions to:
// - Write active connections to a log file
// - Check active connections against limits
// - Log active connections
//
// The main functions are:
// - writeActiveConnections(): Writes active connections to a log file
// - activeConnections(): Checks active connections against limits
// - runningQueries(): Checks running queries against limits
package pgsqlHealth

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
)

// writeActiveConnections queries and logs current active database connections
// to a rotating daily log file with details like PID, user, client address,
// query duration, and state
func writeActiveConnections() {
	query := `
		SELECT pid,usename, client_addr, now() - pg_stat_activity.query_start AS duration, query, state
		FROM pg_stat_activity
		WHERE state='active'
		ORDER BY duration DESC;
	`
	//run the query and write its output to a file
	rows, err := Connection.Query(query)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		return
	}
	defer rows.Close()

	dayOfWeek := time.Now().Weekday().String()
	logFileName := fmt.Sprintf("/var/log/monodb/pgsql-stat_activity-%s.log", dayOfWeek)
	logFile, err := os.Create(logFileName)
	if err != nil {
		common.LogError("Failed to create log file: " + err.Error())
	}
	defer logFile.Close()

	// Write query results to log file
	_, _ = fmt.Fprintf(logFile, "Query executed at: %v\n\n", time.Now())
	_, _ = fmt.Fprintf(logFile, "%-5s %-10s %-15s %-20s %-50s %-10s\n", "PID", "User", "Client Addr", "Duration", "Query", "State")
	_, _ = fmt.Fprintln(logFile, strings.Repeat("-", 120))

	for rows.Next() {
		var (
			pid        int
			usename    string
			clientAddr string
			duration   string
			query      string
			state      string
		)

		err := rows.Scan(&pid, &usename, &clientAddr, &duration, &query, &state)
		if err != nil {
			common.LogError("Failed to scan row: " + err.Error())
			continue
		}

		_, _ = fmt.Fprintf(logFile, "%-5d %-10s %-15s %-20s %-50s %-10s\n",
			pid, usename, clientAddr, duration, query[:40], state)
	}

}

// activeConnections checks the current number of active connections
// and compares it to the maximum allowed connections
func activeConnections() {
	var maxConn, used, increase int
	aboveLimitFile := common.TmpDir + "/last-connection-above-limit.txt"

	query := `
		SELECT max_conn, used FROM (SELECT COUNT(*) used
        FROM pg_stat_activity) t1, (SELECT setting::int max_conn FROM pg_settings WHERE name='max_connections') t2
	`
	err := Connection.QueryRow(query).Scan(&maxConn, &used)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		common.PrettyPrintStr("PostgreSQL active connections", false, "accessible")
		common.AlarmCheckDown("postgres_active_conn", "An error occurred while checking active connections: "+err.Error(), false, "", "")
		return
	} else {
		common.AlarmCheckUp("postgres_active_conn", "Active connections are now accessible", false)
	}

	usedPercent := (used * 100) / maxConn

	if _, err := os.Stat(aboveLimitFile); os.IsNotExist(err) {
		increase = 1
	} else {
		// set increase to the content of the file
		content, err := os.ReadFile(aboveLimitFile)
		if err != nil {
			common.LogError(fmt.Sprintf("Error reading file: %v\n", err))
			increase = 1
		}
		increase = int(content[0])
	}

	if usedPercent >= DbHealthConfig.Postgres.Limits.Conn_percent {
		if _, err := os.Stat(aboveLimitFile); os.IsNotExist(err) {
			writeActiveConnections()

		}
		common.PrettyPrintStr("Number of active connections", true, fmt.Sprintf("%d/%d and above %d", used, maxConn, DbHealthConfig.Postgres.Limits.Conn_percent))
		common.AlarmCheckDown("postgres_num_active_conn", fmt.Sprintf("Number of active connections: %d/%d and above %d", used, maxConn, DbHealthConfig.Postgres.Limits.Conn_percent), false, "", "")
		difference := (used - maxConn) / 10
		if difference > increase {
			writeActiveConnections()
			increase = difference + 1
		}
		fmt.Println("-----------------", increase)
		fmt.Println("-----------------", []byte{byte(increase)})
		err = os.WriteFile(aboveLimitFile, []byte(strconv.Itoa(increase)), 0644)
		if err != nil {
			common.LogError(fmt.Sprintf("Error writing file: %v\n", err))
		}
	} else {
		common.PrettyPrintStr("Number of active connections", true, fmt.Sprintf("%d/%d", used, maxConn))
		common.AlarmCheckUp("postgres_num_active_conn", fmt.Sprintf("Number of active connections is now: %d/%d", used, maxConn), false)
		if _, err := os.Stat(aboveLimitFile); err == nil {
			err := os.Remove(aboveLimitFile)
			if err != nil {
				common.LogError(fmt.Sprintf("Error deleting file: %v\n", err))
			}
		}
	}
}

// runningQueries checks the current number of running queries
// and compares it to the maximum allowed queries
func runningQueries() {
	query := `SELECT COUNT(*) AS active_queries_count FROM pg_stat_activity WHERE state = 'active';`

	var activeQueriesCount int
	err := Connection.QueryRow(query).Scan(&activeQueriesCount)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		common.PrettyPrintStr("Number of running queries", false, "accessible")
		common.AlarmCheckDown("postgres_running_queries", "An error occurred while checking running queries: "+err.Error(), false, "", "")
		return
	} else {
		common.AlarmCheckUp("postgres_running_queries", "Running queries are now accessible", false)
	}

	if activeQueriesCount > DbHealthConfig.Postgres.Limits.Query {
		common.PrettyPrintStr("Number of running queries", true, fmt.Sprintf("%d/%d", activeQueriesCount, DbHealthConfig.Postgres.Limits.Query))
		common.AlarmCheckDown("postgres_num_running_queries", fmt.Sprintf("Number of running queries: %d/%d", activeQueriesCount, DbHealthConfig.Postgres.Limits.Query), false, "", "")
	} else {
		common.PrettyPrintStr("Number of running queries", true, fmt.Sprintf("%d/%d", activeQueriesCount, DbHealthConfig.Postgres.Limits.Query))
		common.AlarmCheckUp("postgres_num_running_queries", fmt.Sprintf("Number of running queries is now: %d/%d", activeQueriesCount, DbHealthConfig.Postgres.Limits.Query), false)
	}
}
