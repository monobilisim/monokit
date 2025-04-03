package mysqlHealth

import "database/sql"

// Global database connection
var Connection *sql.DB

// DbConfig represents the database configuration
type DbConfig struct {
	Mysql MysqlConfig
}

// MysqlConfig represents MySQL specific configuration
type MysqlConfig struct {
	Process_limit int
	Cluster       ClusterConfig
}

// ClusterConfig represents MySQL cluster configuration
type ClusterConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Size             int    `yaml:"size"`
	Check_table_day  string `yaml:"check_table_day"`
	Check_table_hour string `yaml:"check_table_hour"`
}

// DbHealthConfig holds the global configuration instance
var DbHealthConfig DbConfig
