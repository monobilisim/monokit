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
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	db "github.com/monobilisim/monokit/common/db"
	"github.com/rs/zerolog/log"
)

// ConnectionsData holds information about PostgreSQL connections
type ConnectionsData struct {
	Active    int
	Limit     int
	UsageRate float64
}

// QueryData holds information about a running PostgreSQL query
type QueryData struct {
	PID             int
	Username        string
	ClientAddr      string
	Duration        string
	DurationSeconds float64
	Query           string
	State           string
	Database        string
}

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
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "writeActiveConnections").Str("action", "query_execution_failed").Str("query", query).Msg("Error executing query")
		return
	}
	defer rows.Close()

	dayOfWeek := time.Now().Weekday().String()
	logFileName := fmt.Sprintf("/var/log/monodb/pgsql-stat_activity-%s.log", dayOfWeek)
	logFile, err := os.Create(logFileName)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "writeActiveConnections").Str("action", "create_log_file_failed").Str("log_file", logFileName).Msg("Failed to create log file")
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
			log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "writeActiveConnections").Str("action", "scan_row_failed").Msg("Failed to scan row")
			continue
		}

		_, _ = fmt.Fprintf(logFile, "%-5d %-10s %-15s %-20s %-50s %-10s\n",
			pid, usename, clientAddr, duration, query[:40], state)
	}

}

// activeConnections checks the current number of active connections
// and compares it to the maximum allowed connections
func activeConnections(dbConfig db.DbHealth) (*ConnectionsData, error) {
	var maxConn, used, increase int
	aboveLimitFile := common.TmpDir + "/last-connection-above-limit.txt"
	connectionsData := &ConnectionsData{}

	query := `
		SELECT max_conn, used FROM (SELECT COUNT(*) used
        FROM pg_stat_activity) t1, (SELECT setting::int max_conn FROM pg_settings WHERE name='max_connections') t2
	`
	err := Connection.QueryRow(query).Scan(&maxConn, &used)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "activeConnections").Str("action", "query_execution_failed").Str("query", query).Msg("Error executing query")
		common.AlarmCheckDown("postgres_active_conn", "An error occurred while checking active connections: "+err.Error(), false, "", "")
		return nil, err
	} else {
		common.AlarmCheckUp("postgres_active_conn", "Active connections are now accessible", false)
	}

	// Set connection data
	connectionsData.Active = used
	connectionsData.Limit = maxConn
	connectionsData.UsageRate = float64(used) / float64(maxConn)

	usedPercent := (used * 100) / maxConn

	if _, err := os.Stat(aboveLimitFile); os.IsNotExist(err) {
		increase = 1
	} else {
		// set increase to the content of the file
		content, err := os.ReadFile(aboveLimitFile)
		if err != nil {
			log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "activeConnections").Str("action", "read_file_failed").Str("file", aboveLimitFile).Msg("Error reading file")
			increase = 1
		}
		increase = int(content[0])
	}

	if usedPercent >= dbConfig.Postgres.Limits.Conn_percent {
		if _, err := os.Stat(aboveLimitFile); os.IsNotExist(err) {
			writeActiveConnections()
		}
		common.AlarmCheckDown("postgres_num_active_conn", fmt.Sprintf("Number of active connections: %d/%d and above %d", used, maxConn, dbConfig.Postgres.Limits.Conn_percent), false, "", "")
		difference := (used - maxConn) / 10
		if difference > increase {
			writeActiveConnections()
			increase = difference + 1
		}
		err = os.WriteFile(aboveLimitFile, []byte(strconv.Itoa(increase)), 0644)
		if err != nil {
			log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "activeConnections").Str("action", "write_file_failed").Str("file", aboveLimitFile).Msg("Error writing file")
		}
	} else {
		common.AlarmCheckUp("postgres_num_active_conn", fmt.Sprintf("Number of active connections is now: %d/%d", used, maxConn), false)
		if _, err := os.Stat(aboveLimitFile); err == nil {
			err := os.Remove(aboveLimitFile)
			if err != nil {
				log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "activeConnections").Str("action", "delete_file_failed").Str("file", aboveLimitFile).Msg("Error deleting file")
			}
		}
	}

	return connectionsData, nil
}

// runningQueries checks the current number of running queries
// and compares it to the maximum allowed queries
func runningQueries(dbConfig db.DbHealth) ([]QueryData, error) {
	// First get total count for the pretty print output
	countQuery := `SELECT COUNT(*) AS active_queries_count FROM pg_stat_activity WHERE state = 'active';`

	var activeQueriesCount int
	err := Connection.QueryRow(countQuery).Scan(&activeQueriesCount)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "runningQueries").Str("action", "query_execution_failed").Str("query", countQuery).Msg("Error executing query")
		// Already commented out console output
		common.AlarmCheckDown("postgres_running_queries", "An error occurred while checking running queries: "+err.Error(), false, "", "")
		return nil, err
	} else {
		common.AlarmCheckUp("postgres_running_queries", "Running queries are now accessible", false)
	}

	// Now get detailed information about running queries
	detailQuery := `
		SELECT pid, usename, datname, 
		       extract(epoch from (now() - query_start)) as duration_seconds,
		       to_char(now() - query_start, 'HH24:MI:SS') as duration, 
		       state, query, client_addr
		FROM pg_stat_activity 
		WHERE state = 'active' AND query != '<IDLE>' AND usename != 'postgres'
		ORDER BY duration_seconds DESC;
	`

	rows, err := Connection.Query(detailQuery)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "runningQueries").Str("action", "query_execution_failed").Str("query", detailQuery).Msg("Error executing query")
		return nil, err
	}
	defer rows.Close()

	queries := []QueryData{}
	for rows.Next() {
		var q QueryData
		var clientAddr sql.NullString
		var datname sql.NullString

		err := rows.Scan(
			&q.PID,
			&q.Username,
			&datname,
			&q.DurationSeconds,
			&q.Duration,
			&q.State,
			&q.Query,
			&clientAddr)

		if err != nil {
			log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "runningQueries").Str("action", "scan_row_failed").Msg("Failed to scan query row")
			continue
		}

		if datname.Valid {
			q.Database = datname.String
		} else {
			q.Database = "unknown"
		}

		if clientAddr.Valid {
			q.ClientAddr = clientAddr.String
		} else {
			q.ClientAddr = "local"
		}

		queries = append(queries, q)
	}

	// Neither of these blocks should be printing to the console anymore
	// We already commented these out, but let's make extra sure there's no output
	if activeQueriesCount > dbConfig.Postgres.Limits.Query {
		common.AlarmCheckDown("postgres_num_running_queries", fmt.Sprintf("Number of running queries: %d/%d", activeQueriesCount, dbConfig.Postgres.Limits.Query), false, "", "")
	} else {
		common.AlarmCheckUp("postgres_num_running_queries", fmt.Sprintf("Number of running queries is now: %d/%d", activeQueriesCount, dbConfig.Postgres.Limits.Query), false)
	}

	return queries, nil
}
