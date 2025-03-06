//go:build with_api

package common

import (
	"time"

	"gorm.io/gorm"
)

// Server represents the server configuration
type Server struct {
	Port     string
	Postgres struct {
		Host     string
		Port     string
		User     string
		Password string
		Dbname   string
	}
}

// Host represents a monitored host
type Host struct {
	gorm.Model
	Name                string    `json:"name"`
	CpuCores            int       `json:"cpuCores"`
	Ram                 string    `json:"ram"`
	MonokitVersion      string    `json:"monokitVersion"`
	Os                  string    `json:"os"`
	DisabledComponents  string    `json:"disabledComponents"`
	InstalledComponents string    `json:"installedComponents"`
	IpAddress           string    `json:"ipAddress"`
	Status              string    `json:"status"`
	UpdatedAt           time.Time `json:"updatedAt"`
	CreatedAt           time.Time `json:"createdAt"`
	WantsUpdateTo       string    `json:"wantsUpdateTo"`
	Groups              string    `json:"groups"`
	UpForDeletion       bool      `json:"upForDeletion"`
	Inventory           string    `json:"inventory"`
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
}

var HostsList []Host
