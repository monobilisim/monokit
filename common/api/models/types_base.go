//go:build !with_api

package models

import (
	"time"
)

// Host represents a monitored host
type Host struct {
	ID                  uint      `json:"id"`
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
}

// User represents a system user
type User struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Password    string `json:"-"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Groups      string `json:"groups"`
	Inventories string `json:"inventories"`
}

// Session represents a user session
type Session struct {
	ID        uint      `json:"id"`
	Token     string    `json:"token"`
	UserID    uint      `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Timeout   time.Time `json:"timeout"`
	User      User      `json:"user"`
}

// HostResponse represents a host response
type HostResponse struct {
	Name               string `json:"name"`
	Version            string `json:"version"`
	Status             string `json:"status"`
	Groups             string `json:"groups"`
	DisabledComponents string `json:"disabled_components"`
	WantsUpdateTo      string `json:"wants_update_to"`
	UpForDeletion      bool   `json:"up_for_deletion"`
}

// APIHost represents a host in the API
type APIHost struct {
	ID                  uint      `json:"id"`
	Name                string    `json:"name"`
	CpuCores            int       `json:"cpu_cores"`
	Ram                 string    `json:"ram"`
	MonokitVersion      string    `json:"monokit_version"`
	Os                  string    `json:"os"`
	Version             string    `json:"version"`
	Status              string    `json:"status"`
	Groups              string    `json:"groups"`
	DisabledComponents  string    `json:"disabled_components"`
	InstalledComponents string    `json:"installed_components"`
	IpAddress           string    `json:"ip_address"`
	WantsUpdateTo       string    `json:"wants_update_to"`
	UpForDeletion       bool      `json:"up_for_deletion"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Group struct is defined in models.go

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Groups   string `json:"groups"`
	} `json:"user"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Role     string `json:"role" binding:"required"`
	Groups   string `json:"groups"`
}

// UpdateMeRequest represents a request to update the current user
type UpdateMeRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// UserResponse represents a user response
type UserResponse struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Groups      string `json:"groups"`
	Inventories string `json:"inventories"`
}
