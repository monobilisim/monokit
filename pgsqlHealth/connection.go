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
	"os"
	"fmt"
	"errors"
	"strings"
	"gopkg.in/yaml.v3"
	"github.com/monobilisim/monokit/common"
	_ "github.com/lib/pq"
)


// getPatroniUrl reads the Patroni configuration file and returns the REST API
// connection address for the Patroni cluster
func getPatroniUrl() (string, error) {
	// Read the config file
	data, err := os.ReadFile("/etc/patroni/patroni.yml")
	if err != nil {
		common.LogError(fmt.Sprintf("couldn't read patroni config file: %v\n", err))
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
		common.LogError(fmt.Sprintf("couldn't unmarshal patroni config file: %v\n", err))
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
		psqlConn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable", host, port, user, password)
	} else {
		// Try to do UNIX auth
		psqlConn = "dbname=postgres sslmode=disable host=/var/run/postgresql"
	}

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