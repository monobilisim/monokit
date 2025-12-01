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
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

var patroniHTTPClient *http.Client

type patroniConf struct {
	Name    string         `json:"name"`
	Restapi patroniRestAPI `yaml:"restapi"`
}

type patroniRestAPI struct {
	ConnectAddress string `yaml:"connect_address"`
	CertFile       string `yaml:"certfile"`
	KeyFile        string `yaml:"keyfile"`
	CAFile         string `yaml:"cafile"`
}

// getPatroniUrl reads the Patroni configuration file and returns the REST API
// connection address for the Patroni cluster
func getPatroniUrl() (string, error) {
	// Read the config file
	data, err := os.ReadFile("/etc/patroni/patroni.yml")
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "getPatroniUrl").Str("action", "read_patroni_config_failed").Msg("couldn't read patroni config file")
		return "", err
	}

	var patroni patroniConf
	// Unmarshal the YAML data into the struct
	err = yaml.Unmarshal(data, &patroni)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "getPatroniUrl").Str("action", "unmarshal_patroni_config_failed").Msg("couldn't unmarshal patroni config file")
		return "", err
	}
	nodeName = patroni.Name

	connectAddress := strings.TrimSpace(patroni.Restapi.ConnectAddress)
	if connectAddress == "" {
		return "", errors.New("patroni restapi connect_address is empty")
	}

	tlsConfigured := patroni.Restapi.CertFile != "" || patroni.Restapi.KeyFile != "" || patroni.Restapi.CAFile != ""
	hasScheme := strings.HasPrefix(connectAddress, "http://") || strings.HasPrefix(connectAddress, "https://")

	if !hasScheme {
		if tlsConfigured {
			connectAddress = "https://" + connectAddress
		} else {
			connectAddress = "http://" + connectAddress
		}
	}

	u, err := url.Parse(connectAddress)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid patroni connect_address: %s", connectAddress)
	}

	// Keep only scheme://host[:port] portion to avoid double slashes when adding paths
	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	if u.Path != "" && u.Path != "/" {
		baseURL = fmt.Sprintf("%s%s", baseURL, strings.TrimSuffix(u.Path, "/"))
	}

	client, err := buildPatroniHTTPClient(patroni.Restapi, u.Hostname(), tlsConfigured)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "getPatroniUrl").Str("action", "build_patroni_http_client_failed").Msg("couldn't build patroni http client with provided certificates")
		return "", err
	}
	patroniHTTPClient = client

	return baseURL, nil
}

// Connect establishes a connection to the PostgreSQL database
// It first checks if the .pgpass file exists and uses it to connect
func Connect() error {
	pgPass := "/var/lib/postgresql/.pgpass"
	var useSocket bool
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
		useSocket = true
	}

	// open database
	db, err := sql.Open("postgres", psqlConn)
	if err != nil {
		log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Connect").Str("action", "connect_to_postgresql_failed").Msg("Couldn't connect to postgresql")
		return err
	}

	err = db.Ping()
	if err != nil {
		if useSocket {
			psqlConn := "dbname=postgres sslmode=disable host=/data/postgresql/16"
			db, err = sql.Open("postgres", psqlConn)
			if err != nil {
				log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Connect").Str("action", "connect_to_postgresql_failed").Msg("Couldn't connect to postgresql")
				return err
			}
			err = db.Ping()
			if err != nil {
				log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Connect").Str("action", "ping_postgresql_failed").Msg("Couldn't ping postgresql")
				return err
			}
		} else {
			log.Error().Err(err).Str("component", "pgsqlHealth").Str("operation", "Connect").Str("action", "ping_postgresql_failed").Msg("Couldn't ping postgresql")
			return err
		}
	}

	Connection = db
	return nil
}

func buildPatroniHTTPClient(restapi patroniRestAPI, serverName string, tlsConfigured bool) (*http.Client, error) {
	if !tlsConfigured {
		return &http.Client{Timeout: 10 * time.Second}, nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if serverName != "" {
		tlsConfig.ServerName = serverName
	}

	if restapi.CAFile != "" {
		caData, err := os.ReadFile(restapi.CAFile)
		if err != nil {
			return nil, fmt.Errorf("couldn't read patroni CA file: %w", err)
		}

		rootCAs, err := x509.SystemCertPool()
		if err != nil || rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		if ok := rootCAs.AppendCertsFromPEM(caData); !ok {
			return nil, fmt.Errorf("failed to append CA certificates from %s", restapi.CAFile)
		}

		tlsConfig.RootCAs = rootCAs
	}

	if restapi.CertFile != "" && restapi.KeyFile != "" {
		clientCert, err := tls.LoadX509KeyPair(restapi.CertFile, restapi.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("couldn't load patroni client certificate or key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}, nil
}

func getPatroniHTTPClient() *http.Client {
	if patroniHTTPClient != nil {
		return patroniHTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}
