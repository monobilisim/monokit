package pgsqlHealth

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/monobilisim/monokit/common"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
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

func activeConnections() {
	var maxConn, used int
	query := `SELECT max_conn, used FROM (SELECT COUNT(*) used FROM pg_stat_activity) t1, (SELECT setting::int max_conn FROM pg_settings WHERE name='max_connections') t2`
	err := Connection.QueryRow(query).Scan(&maxConn, &used)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		common.PrettyPrintStr("PostgreSQL active connections", false, "accessible")
		return
	}
	common.PrettyPrintStr("PostgreSQL active connections", true, fmt.Sprintf("%d/%d", used, maxConn))
}
