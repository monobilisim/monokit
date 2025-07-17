//go:build with_api

package models

import (
	"time"

	"gorm.io/gorm"
)

// KeycloakConfig represents Keycloak SSO configuration
type KeycloakConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	URL              string `mapstructure:"url"`
	Realm            string `mapstructure:"realm"`
	ClientID         string `mapstructure:"client_id"`
	ClientSecret     string `mapstructure:"client_secret"`
	DisableLocalAuth bool   `mapstructure:"disable_local_auth"`
	AdminRole        string `mapstructure:"admin_role"`
	DefaultRole      string `mapstructure:"default_role"`
	DefaultInventory string `mapstructure:"default_inventory"`
	DefaultGroups    string `mapstructure:"default_groups"`
}

// ValkeyConfig represents Valkey cache configuration
type ValkeyConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Address      string `mapstructure:"address"`
	Password     string `mapstructure:"password"`
	Database     int    `mapstructure:"database"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxConnAge   int    `mapstructure:"max_conn_age"`  // in seconds
	IdleTimeout  int    `mapstructure:"idle_timeout"`  // in seconds
	ConnTimeout  int    `mapstructure:"conn_timeout"`  // in seconds
	ReadTimeout  int    `mapstructure:"read_timeout"`  // in seconds
	WriteTimeout int    `mapstructure:"write_timeout"` // in seconds
	DefaultTTL   int    `mapstructure:"default_ttl"`   // in seconds
	SessionTTL   int    `mapstructure:"session_ttl"`   // in seconds
	HealthTTL    int    `mapstructure:"health_ttl"`    // in seconds
	HostTTL      int    `mapstructure:"host_ttl"`      // in seconds
	KeyPrefix    string `mapstructure:"key_prefix"`
}

// Server represents the server configuration
type Server struct {
	Port     string `mapstructure:"port"`
	Postgres struct {
		Host     string `mapstructure:"host"`
		Port     string `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Dbname   string `mapstructure:"dbname"`
	} `mapstructure:"postgres"`
	Keycloak KeycloakConfig `mapstructure:"keycloak"`
	Awx      AwxConfig      `mapstructure:"awx"`
	Valkey   ValkeyConfig   `mapstructure:"valkey"`
}

// AwxConfig represents AWX connection settings
type AwxConfig struct {
	Enabled            bool              `mapstructure:"enabled"`
	Url                string            `mapstructure:"url"`
	Username           string            `mapstructure:"username"`
	Password           string            `mapstructure:"password"`
	VerifySSL          bool              `mapstructure:"verify_ssl"`
	Timeout            int               `mapstructure:"timeout"`
	HostIdMap          map[string]string `mapstructure:"host_id_map"`
	DefaultInventoryID int               `mapstructure:"default_inventory_id"`
	// Map of template names to IDs
	TemplateIDs map[string]int `mapstructure:"template_ids"`
	// Map of workflow template names to IDs
	WorkflowTemplateIDs map[string]int `mapstructure:"workflow_template_ids"`
	// Default workflow template ID to use if none specified
	DefaultWorkflowTemplateID int `mapstructure:"default_workflow_template_id"`
}

// Host represents a monitored host
type Host struct {
	gorm.Model
	Name                string    `json:"name" gorm:"uniqueIndex"`
	CpuCores            int       `json:"cpuCores"`
	Ram                 string    `json:"ram"`
	MonokitVersion      string    `json:"monokitVersion"`
	Os                  string    `json:"os"`
	DisabledComponents  string    `json:"disabledComponents"`
	InstalledComponents string    `json:"installedComponents"`
	IpAddress           string    `json:"ipAddress"`
	Status              string    `json:"status"`
	AwxHostId           string    `json:"awxHostId"`
	AwxHostUrl          string    `json:"awxHostUrl"`
	UpdatedAt           time.Time `json:"updatedAt"`
	CreatedAt           time.Time `json:"createdAt"`
	WantsUpdateTo       string    `json:"wantsUpdateTo"`
	Groups              string    `json:"groups"`
	UpForDeletion       bool      `json:"upForDeletion"`
	Inventory           string    `json:"inventory"`
	AwxOnly             bool      `json:"awx_only"` // If true, this host is only in AWX and should not be shown in dashboard
}

// Inventory represents a collection of hosts
type Inventory struct {
	ID    uint   `json:"id" gorm:"primarykey"`
	Name  string `json:"name" gorm:"unique"`
	Hosts []Host `json:"hosts" gorm:"foreignKey:Inventory;references:Name"`
}

// User represents a system user
type User struct {
	gorm.Model
	Username    string `json:"username" gorm:"unique"`
	Password    string `json:"-"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Groups      string `json:"groups"`
	Inventories string `json:"inventories"`
	AuthMethod  string `json:"auth_method"`
}

// HostKey represents an API key for a host
type HostKey struct {
	ID       uint   `json:"id" gorm:"primarykey"`
	Token    string `json:"token"`
	HostName string `json:"host_name"`
}

// Session represents a user session
type Session struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Token     string    `json:"token"`
	UserID    uint      `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Timeout   time.Time `json:"timeout"`
	User      User      `json:"user"`
}

// Group represents a group in the system
type Group struct {
	ID    uint   `json:"id" gorm:"primarykey"`
	Name  string `json:"name" gorm:"unique"`
	Hosts []Host `json:"hosts" gorm:"many2many:group_hosts;"`
	Users []User `json:"users" gorm:"many2many:group_users;"`
}

// HostFileConfig model
type HostFileConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	HostName  string    `gorm:"index" json:"host_name"`
	FileName  string    `json:"file_name"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HostLog represents a log entry from a host
type HostLog struct {
	gorm.Model
	HostName  string    `json:"host_name" gorm:"index"`
	Level     string    `json:"level" gorm:"index"`     // info, warning, error, critical
	Component string    `json:"component" gorm:"index"` // system, application, service name, etc.
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp" gorm:"index"`
	Metadata  string    `json:"metadata"` // JSON string for additional data
	Type      string    `json:"type"`
}

// HostHealthData stores the latest health check JSON output for a specific tool on a host.
type HostHealthData struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	HostName    string    `gorm:"uniqueIndex:idx_host_tool;not null" json:"host_name"` // Foreign key to Host.Name
	ToolName    string    `gorm:"uniqueIndex:idx_host_tool;not null" json:"tool_name"` // e.g., "osHealth", "mysqlHealth"
	DataJSON    string    `gorm:"type:jsonb;not null" json:"data_json"`                // The actual JSON output from the health tool
	LastUpdated time.Time `gorm:"autoUpdateTime" json:"last_updated"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName specifies the database table name for the HostHealthData model.
func (HostHealthData) TableName() string {
	return "host_health_data"
}

// Global variables
var ServerConfig struct {
	Port     string `mapstructure:"port"`
	Postgres struct {
		Host     string `mapstructure:"host"`
		Port     string `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Dbname   string `mapstructure:"dbname"`
	} `mapstructure:"postgres"`
	Keycloak KeycloakConfig `mapstructure:"keycloak"`
	Awx      AwxConfig      `mapstructure:"awx"`
	Valkey   ValkeyConfig   `mapstructure:"valkey"`
}

var HostsList []Host
