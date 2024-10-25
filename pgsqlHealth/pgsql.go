package pgsqlHealth

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/monobilisim/monokit/common"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"strings"
	"time"
)

var Connection *sql.DB

func getPatroniUrl() (string, error) {
	// Read the file
	data, err := os.ReadFile("/etc/patroni/patroni.yml")
	if err != nil {
		common.LogError(fmt.Sprintf("couldn't read patroni config file: %v\n", err))
		return "", err
	}

	// Create a struct to hold the YAML data
	type patroniConf struct {
		Restapi struct {
			ConnectAddress string `yaml:"connect_address"`
		} `yaml:"restapi"`
	}
	var patroni patroniConf
	// Unmarshal the YAML data into the struct
	err = yaml.Unmarshal(data, &patroni)
	if err != nil {
		common.LogError(fmt.Sprintf("couldn't unmarshal patroni config file: %v\n", err))
		return "", err
	}

	return patroni.Restapi.ConnectAddress, nil
}

func Connect() error {
	pgPass := "/var/lib/postgresql/.pgpass"
	content, err := os.ReadFile(pgPass)
	if err != nil {
		common.LogError("Error reading file: " + err.Error())
		return err
	}

	// Split the content into lines
	lines := strings.Split(string(content), "\n")

	var host, port, user, password string

	// Find the line containing "localhost"
	for _, line := range lines {
		if strings.Contains(line, "localhost") {
			// Parse the line using colon (:) as a separator
			parts := strings.Split(strings.TrimSpace(line), ":")
			if len(parts) != 5 {
				common.LogError("Invalid .pgpass file format")
				return errors.New("invalid .pgpass file format")
			}

			host = parts[0]
			port = parts[1]
			user = parts[3]
			password = parts[4]

			break
		}
	}

	// connection string
	psqlConn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable", host, port, user, password)

	// open database
	db, err := sql.Open("postgres", psqlConn)
	if err != nil {
		common.LogError("Couldn't connect to postgresql: " + err.Error())
		return err
	}

	err = db.Ping()
	if err != nil {
		common.LogError("Couldn't ping postgresql: " + err.Error())
		return err
	}

	Connection = db
	return nil
}

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
		return
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
		common.PrettyPrintStr("Number of active connections", false, fmt.Sprintf("%d/%d and above %d", used, maxConn, DbHealthConfig.Postgres.Limits.Conn_percent))
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
		if _, err := os.Stat(aboveLimitFile); err == nil {
			err := os.Remove(aboveLimitFile)
			if err != nil {
				common.LogError(fmt.Sprintf("Error deleting file: %v\n", err))
			}
		}
	}
}
