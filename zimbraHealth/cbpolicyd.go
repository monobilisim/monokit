//go:build linux && plugin

package zimbraHealth

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/process"

	// Database drivers
	_ "github.com/go-sql-driver/mysql"
)

// CheckCBPolicyd performs comprehensive checks for cbpolicyd service and database connectivity
func CheckCBPolicyd() CBPolicydInfo {
	info := CBPolicydInfo{
		CheckStatus: false, // Default to check failed
	}

	// Check if cbpolicyd service is running
	info.ServiceRunning = isCBPolicydRunning()

	// Check if configuration file exists
	configPath := "/opt/zimbra/conf/cbpolicyd.conf.in"
	if _, err := os.Stat(configPath); err == nil {
		info.ConfigExists = true

		// Parse configuration for database information
		dbConfig, err := parseCBPolicydConfig(configPath)
		if err != nil {
			info.Message = "Error parsing cbpolicyd configuration: " + err.Error()
			log.Error().Err(err).Str("config_path", configPath).Msg("Error parsing cbpolicyd configuration")
			return info
		}

		if dbConfig != nil {
			info.DatabaseConfigured = true
			info.DatabaseType = dbConfig.Type
			info.DatabaseHost = dbConfig.Host
			info.DatabaseName = dbConfig.Database

			// Test database connectivity
			info.DatabaseConnectable = testDatabaseConnection(dbConfig)
		} else {
			info.DatabaseConfigured = false
			info.Message = "No database configuration found in cbpolicyd.conf.in"
		}
	} else {
		info.ConfigExists = false
		info.Message = "cbpolicyd.conf.in not found at " + configPath
		log.Warn().Str("config_path", configPath).Msg("cbpolicyd configuration file not found")
	}

	info.CheckStatus = true

	// Generate comprehensive status message
	info.Message = generateCBPolicydStatusMessage(info)

	// Send alarms based on check results
	sendCBPolicydAlarms(info)

	return info
}

// isCBPolicydRunning checks if cbpolicyd service is running using gopsutil
func isCBPolicydRunning() bool {
	// CBPolicyd typically runs as: /usr/bin/perl /opt/zimbra/common/bin/cbpolicyd --config /opt/zimbra/conf/cbpolicyd.conf
	// We need to search through all processes since common.ProcGrep is specific to monokit

	// Get all processes using gopsutil
	procs, err := process.Processes()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get process list to check cbpolicyd")
		return false
	}

	for _, proc := range procs {
		// Get the command line for this process
		cmdline, err := proc.Cmdline()
		if err != nil {
			continue // Skip processes we can't read
		}

		// Check if this is a cbpolicyd process
		if strings.Contains(cmdline, "/opt/zimbra/common/bin/cbpolicyd") {
			log.Debug().Str("cmdline", cmdline).Msg("cbpolicyd process found running")
			return true
		}

		// Fallback: check for just "cbpolicyd" in case the path is different
		if strings.Contains(cmdline, "cbpolicyd") && !strings.Contains(cmdline, "grep") {
			log.Debug().Str("cmdline", cmdline).Msg("cbpolicyd process found running (fallback search)")
			return true
		}
	}

	log.Debug().Msg("cbpolicyd service not found running")
	return false
}

// parseCBPolicydConfig parses the cbpolicyd configuration file for database settings
func parseCBPolicydConfig(configPath string) (*DatabaseConfig, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config *DatabaseConfig
	scanner := bufio.NewScanner(file)
	inDatabaseSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for database section
		if line == "[database]" {
			inDatabaseSection = true
			config = &DatabaseConfig{}
			continue
		}

		// Check for other sections (exit database section)
		if strings.HasPrefix(line, "[") && line != "[database]" {
			inDatabaseSection = false
			continue
		}

		// Parse database configuration lines
		if inDatabaseSection && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "DSN":
				config.DSN = value
				parseDSN(config, value)
			case "Username":
				config.Username = value
			case "Password":
				config.Password = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return config, nil
}

// parseDSN parses the DSN string to extract database connection details
func parseDSN(config *DatabaseConfig, dsn string) {
	// Example DSN formats:
	// DBI:mysql:database=policyd_db;host=127.0.0.1;port=7306
	// DBI:SQLite:dbname=policyd.sqlite

	if strings.HasPrefix(dsn, "DBI:mysql:") {
		config.Type = "mysql"
		// Parse mysql DSN
		mysqlPart := strings.TrimPrefix(dsn, "DBI:mysql:")
		params := strings.Split(mysqlPart, ";")

		for _, param := range params {
			if strings.Contains(param, "=") {
				kv := strings.SplitN(param, "=", 2)
				if len(kv) == 2 {
					switch kv[0] {
					case "database":
						config.Database = kv[1]
					case "host":
						config.Host = kv[1]
					case "port":
						config.Port = kv[1]
					}
				}
			}
		}
	} else if strings.HasPrefix(dsn, "DBI:SQLite:") {
		config.Type = "sqlite"
		// Parse SQLite DSN
		sqlitePart := strings.TrimPrefix(dsn, "DBI:SQLite:")
		if strings.HasPrefix(sqlitePart, "dbname=") {
			config.Database = strings.TrimPrefix(sqlitePart, "dbname=")
		}
	}
}

// testDatabaseConnection tests connectivity to the configured database
func testDatabaseConnection(config *DatabaseConfig) bool {
	if config == nil {
		return false
	}

	var db *sql.DB
	var err error

	switch config.Type {
	case "mysql":
		// Build MySQL connection string
		var connStr string
		if config.Username != "" && config.Password != "" {
			connStr = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
				config.Username, config.Password, config.Host, config.Port, config.Database)
		} else {
			log.Warn().Msg("MySQL credentials not found in cbpolicyd configuration")
			return false
		}

		db, err = sql.Open("mysql", connStr)
		if err != nil {
			log.Error().Err(err).Msg("Failed to open MySQL connection for cbpolicyd")
			return false
		}
		defer db.Close()

		// Test the connection with a timeout
		db.SetConnMaxLifetime(5 * time.Second)
		err = db.Ping()
		if err != nil {
			log.Error().Err(err).Str("host", config.Host).Str("database", config.Database).Msg("Failed to ping MySQL database for cbpolicyd")
			return false
		}

		log.Debug().Str("host", config.Host).Str("database", config.Database).Msg("Successfully connected to MySQL database for cbpolicyd")
		return true

	case "sqlite":
		// Send alarm for SQLite usage
		log.Warn().Msg("cbpolicyd is using SQLite database which is not recommended")
		common.AlarmCheckDown("cbpolicyd_sqlite", "cbpolicyd is using SQLite database which is not recommended", false, "", "")

		// Return true since we're not actually testing the connection
		return true

	default:
		log.Warn().Str("type", config.Type).Msg("Unsupported database type for cbpolicyd")
		return false
	}
}

// generateCBPolicydStatusMessage creates a comprehensive status message
func generateCBPolicydStatusMessage(info CBPolicydInfo) string {
	var messages []string

	if !info.ServiceRunning {
		messages = append(messages, "Service not running")
	} else {
		messages = append(messages, "Service running")
	}

	if !info.ConfigExists {
		messages = append(messages, "Config file missing")
	} else if !info.DatabaseConfigured {
		messages = append(messages, "Database not configured")
	} else if !info.DatabaseConnectable {
		messages = append(messages, fmt.Sprintf("Database (%s) not accessible", info.DatabaseType))
	} else {
		messages = append(messages, fmt.Sprintf("Database (%s) accessible", info.DatabaseType))
	}

	return strings.Join(messages, ", ")
}

// sendCBPolicydAlarms sends appropriate alarms based on cbpolicyd check results
func sendCBPolicydAlarms(info CBPolicydInfo) {
	// Check service status
	if !info.ServiceRunning {
		log.Warn().Msg("cbpolicyd service is not running")
		common.AlarmCheckDown("cbpolicyd_service", "cbpolicyd service is not running", false, "", "")
	} else {
		common.AlarmCheckUp("cbpolicyd_service", "cbpolicyd service is running", false)
	}

	// Check configuration
	if !info.ConfigExists {
		log.Error().Msg("cbpolicyd configuration file not found")
		common.AlarmCheckDown("cbpolicyd_config", "cbpolicyd configuration file not found at /opt/zimbra/conf/cbpolicyd.conf.in", false, "", "")
	} else if !info.DatabaseConfigured {
		log.Warn().Msg("cbpolicyd database not configured")
		common.AlarmCheckDown("cbpolicyd_db_config", "cbpolicyd database configuration not found", false, "", "")
	} else {
		common.AlarmCheckUp("cbpolicyd_config", "cbpolicyd configuration file exists and database is configured", false)
	}

	// Check database connectivity
	if info.DatabaseConfigured {
		if !info.DatabaseConnectable {
			log.Error().Str("db_type", info.DatabaseType).Str("db_host", info.DatabaseHost).Str("db_name", info.DatabaseName).Msg("cbpolicyd database not accessible")
			common.AlarmCheckDown("cbpolicyd_db_conn", fmt.Sprintf("cbpolicyd database (%s) not accessible: %s@%s", info.DatabaseType, info.DatabaseName, info.DatabaseHost), false, "", "")
		} else {
			common.AlarmCheckUp("cbpolicyd_db_conn", fmt.Sprintf("cbpolicyd database (%s) is accessible", info.DatabaseType), false)
		}
	}
}
