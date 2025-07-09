// This file implements PostgreSQL connection functionality
//
// It provides functions to:
// - Connect to the PostgreSQL database
// - Get the Patroni REST API URL
// - Read the Patroni configuration file
//
// The main functions are:
// - getPatroniUrl(): Reads the Patroni configuration file and returns the REST API
// connection address for the Patroni cluster
// - Connect(): Establishes a connection to the PostgreSQL database
package pgsqlHealth

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// getPatroniUrl reads the Patroni configuration file and returns the REST API
// connection address for the Patroni cluster
func getPatroniUrl() (string, error) {
	// Read the config file
	data, err := os.ReadFile("/etc/patroni/patroni.yml")
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "getPatroniUrl").Str("action", "read_patroni_config_failed").Msg("couldn't read patroni config file")
		return "", err
	}

	// Create a struct to hold the YAML data
	type patroniConf struct {
		Name    string `json:"name"`
		Restapi struct {
			ConnectAddress string `yaml:"connect_address"`
		} `yaml:"restapi"`
	}
	var patroni patroniConf
	// Unmarshal the YAML data into the struct
	err = yaml.Unmarshal(data, &patroni)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "getPatroniUrl").Str("action", "unmarshal_patroni_config_failed").Msg("couldn't unmarshal patroni config file")
		return "", err
	}
	nodeName = patroni.Name

	return patroni.Restapi.ConnectAddress, nil
}

// Connect establishes a connection to the PostgreSQL database
// It first checks if the .pgpass file exists and uses it to connect
func Connect() error {
	pgPass := "/var/lib/postgresql/.pgpass"
	var psqlConn string
	if _, err := os.Stat(pgPass); err == nil {
		content, err := os.ReadFile(pgPass)
		if err != nil {
			log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Connect").Str("action", "read_pgpass_failed").Msg("Error reading file")
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
					log.Error().Str("component", "pgsqlHealth").Str("operation", "Connect").Str("action", "invalid_pgpass_format").Msg("Invalid .pgpass file format")
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
		psqlConn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable", host, port, user, password)
	} else {
		// Try to do UNIX auth
		psqlConn = "dbname=postgres sslmode=disable host=/var/run/postgresql"
	}

	// open database
	db, err := sql.Open("postgres", psqlConn)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Connect").Str("action", "connect_to_postgresql_failed").Msg("Couldn't connect to postgresql")
		return err
	}

	err = db.Ping()
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Connect").Str("action", "ping_postgresql_failed").Msg("Couldn't ping postgresql")
		return err
	}

	Connection = db
	return nil
}
