//go:build with_api

package models

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel represents the common fields for all models (for Swagger documentation)
type BaseModel struct {
	ID        uint       `json:"id" example:"1"`
	CreatedAt time.Time  `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt time.Time  `json:"updated_at" example:"2023-01-01T00:00:00Z"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" example:"2023-01-01T00:00:00Z"`
}

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
	Keycloak   KeycloakConfig   `mapstructure:"keycloak"`
	Awx        AwxConfig        `mapstructure:"awx"`
	Valkey     ValkeyConfig     `mapstructure:"valkey"`
	Cloudflare CloudflareConfig `mapstructure:"cloudflare"`
}

// AwxConfig represents AWX connection settings
type AwxConfig struct {
	Enabled   bool              `mapstructure:"enabled"`
	Url       string            `mapstructure:"url"`
	Username  string            `mapstructure:"username"`
	Password  string            `mapstructure:"password"`
	VerifySSL bool              `mapstructure:"verify_ssl"`
	Timeout   int               `mapstructure:"timeout"`
	HostIdMap map[string]string `mapstructure:"host_id_map"`
	// Map of template names to IDs
	TemplateIDs map[string]int `mapstructure:"template_ids"`
	// Map of workflow template names to IDs
	WorkflowTemplateIDs map[string]int `mapstructure:"workflow_template_ids"`
	// Default workflow template ID to use if none specified
	DefaultWorkflowTemplateID int `mapstructure:"default_workflow_template_id"`
	// Default inventory ID to use for AWX operations
	DefaultInventoryID int `mapstructure:"default_inventory_id"`
}

// CloudflareConfig represents Cloudflare API connection settings
type CloudflareConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	APIToken  string `mapstructure:"api_token"`  // Global API token for server-level operations
	APIKey    string `mapstructure:"api_key"`    // Legacy API key (optional)
	Email     string `mapstructure:"email"`      // Email for legacy API key authentication
	Timeout   int    `mapstructure:"timeout"`    // Request timeout in seconds
	VerifySSL bool   `mapstructure:"verify_ssl"` // Whether to verify SSL certificates
}

// Host represents a monitored host (now domain-scoped)
// @Description Host model for monitoring servers within a domain
type Host struct {
	gorm.Model          `swaggerignore:"true"`
	Name                string    `json:"name" gorm:"uniqueIndex:idx_host_name_domain" example:"web-server-01"` // Unique within domain
	DomainID            uint      `json:"domain_id" gorm:"uniqueIndex:idx_host_name_domain;index" example:"1"`
	Domain              Domain    `json:"domain,omitempty" swaggerignore:"true"`
	CpuCores            int       `json:"cpuCores" example:"8"`
	Ram                 string    `json:"ram" example:"16GB"`
	MonokitVersion      string    `json:"monokitVersion" example:"1.0.0"`
	Os                  string    `json:"os" example:"Ubuntu 22.04"`
	DisabledComponents  string    `json:"disabledComponents" example:"nil"`
	InstalledComponents string    `json:"installedComponents" example:"osHealth,mysqlHealth"`
	IpAddress           string    `json:"ipAddress"`
	Status              string    `json:"status"`
	AwxHostId           string    `json:"awxHostId"`
	AwxHostUrl          string    `json:"awxHostUrl"`
	UpdatedAt           time.Time `json:"updatedAt"`
	CreatedAt           time.Time `json:"createdAt"`
	WantsUpdateTo       string    `json:"wantsUpdateTo"`
	Groups              string    `json:"groups"`
	UpForDeletion       bool      `json:"upForDeletion"`
	AwxOnly             bool      `json:"awx_only"` // If true, this host is only in AWX and should not be shown in dashboard
}

// Domain represents a tenant domain in the multi-tenant system
// @Description Domain model for multi-tenant system
type Domain struct {
	gorm.Model        `swaggerignore:"true"`
	Name              string             `json:"name" gorm:"unique" example:"production"`
	Description       string             `json:"description" example:"Production environment domain"`
	Settings          string             `json:"settings" gorm:"type:text" example:"{\"theme\":\"dark\"}"` // JSON for domain-specific config
	Active            bool               `json:"active" gorm:"default:true" example:"true"`
	CloudflareDomains []CloudflareDomain `json:"cloudflare_domains,omitempty" gorm:"foreignKey:DomainID"`
}

// CloudflareDomain represents a Cloudflare domain configuration for a specific domain
// @Description Cloudflare domain configuration model
type CloudflareDomain struct {
	gorm.Model   `swaggerignore:"true"`
	DomainID     uint   `json:"domain_id" gorm:"index" example:"1"`
	Domain       Domain `json:"domain,omitempty" swaggerignore:"true"`
	ZoneName     string `json:"zone_name" gorm:"not null" example:"example.com"`  // The actual domain name in Cloudflare
	ZoneID       string `json:"zone_id" gorm:"not null" example:"abc123def456"`   // Cloudflare Zone ID
	APIToken     string `json:"api_token,omitempty" example:"your-api-token"`     // Domain-specific API token (optional, can use global)
	ProxyEnabled bool   `json:"proxy_enabled" gorm:"default:true" example:"true"` // Whether to proxy through Cloudflare
	Active       bool   `json:"active" gorm:"default:true" example:"true"`        // Whether this domain config is active
}

// DomainUser represents the many-to-many relationship between users and domains with roles
type DomainUser struct {
	ID       uint   `json:"id" gorm:"primarykey" example:"1"`
	DomainID uint   `json:"domain_id" gorm:"uniqueIndex:idx_domain_user;index" example:"1"`
	UserID   uint   `json:"user_id" gorm:"uniqueIndex:idx_domain_user;index" example:"1"`
	Role     string `json:"role" example:"domain_admin"` // "domain_admin", "domain_user"
	Domain   Domain `json:"domain,omitempty" swaggerignore:"true"`
	User     User   `json:"user,omitempty" swaggerignore:"true"`
}

// User represents a system user (now with domain associations)
type User struct {
	gorm.Model
	Username    string       `json:"username" gorm:"unique"`
	Password    string       `json:"-"`
	Email       string       `json:"email"`
	Role        string       `json:"role"` // "global_admin" for global access, or empty for domain-scoped users
	Groups      string       `json:"groups"`
	AuthMethod  string       `json:"auth_method"`
	DomainUsers []DomainUser `json:"domain_users" gorm:"foreignKey:UserID"` // Many-to-many with domains through DomainUser
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

// Group represents a group in the system (now domain-scoped)
// @Description Group model for organizing hosts and users within a domain
type Group struct {
	ID       uint   `json:"id" gorm:"primarykey"`
	Name     string `json:"name" gorm:"uniqueIndex:idx_group_name_domain"` // Unique within domain
	DomainID uint   `json:"domain_id" gorm:"uniqueIndex:idx_group_name_domain;index"`
	Domain   Domain `json:"domain" swaggerignore:"true"`
	Hosts    []Host `json:"hosts" gorm:"many2many:group_hosts;" swaggerignore:"true"`
	Users    []User `json:"users" gorm:"many2many:group_users;" swaggerignore:"true"`
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

// Domain-related request/response types
type CreateDomainRequest struct {
	Name              string                          `json:"name" binding:"required"`
	Description       string                          `json:"description"`
	Settings          string                          `json:"settings"`
	CloudflareDomains []CreateCloudflareDomainRequest `json:"cloudflare_domains,omitempty"` // Optional Cloudflare domains to create with the domain
}

type UpdateDomainRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Settings    string `json:"settings"`
	Active      *bool  `json:"active"`
}

type DomainResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Settings    string `json:"settings"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type AssignUserToDomainRequest struct {
	UserID uint   `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"` // "domain_admin" or "domain_user"
}

type UpdateDomainUserRoleRequest struct {
	Role string `json:"role" binding:"required"` // "domain_admin" or "domain_user"
}

type DomainUserResponse struct {
	ID       uint         `json:"id"`
	DomainID uint         `json:"domain_id"`
	UserID   uint         `json:"user_id"`
	Role     string       `json:"role"`
	User     UserResponse `json:"user"`
}

// Cloudflare-related request/response types
type CreateCloudflareDomainRequest struct {
	ZoneName     string `json:"zone_name" binding:"required"`
	ZoneID       string `json:"zone_id" binding:"required"`
	APIToken     string `json:"api_token"`
	ProxyEnabled *bool  `json:"proxy_enabled"`
}

type UpdateCloudflareDomainRequest struct {
	ZoneName     string `json:"zone_name"`
	ZoneID       string `json:"zone_id"`
	APIToken     string `json:"api_token"`
	ProxyEnabled *bool  `json:"proxy_enabled"`
	Active       *bool  `json:"active"`
}

type CloudflareDomainResponse struct {
	ID           uint   `json:"id"`
	DomainID     uint   `json:"domain_id"`
	ZoneName     string `json:"zone_name"`
	ZoneID       string `json:"zone_id"`
	ProxyEnabled bool   `json:"proxy_enabled"`
	Active       bool   `json:"active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
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
	Keycloak   KeycloakConfig   `mapstructure:"keycloak"`
	Awx        AwxConfig        `mapstructure:"awx"`
	Valkey     ValkeyConfig     `mapstructure:"valkey"`
	Cloudflare CloudflareConfig `mapstructure:"cloudflare"`
}

var HostsList []Host
