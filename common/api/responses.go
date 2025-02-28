//go:build with_api

package common

import "time"

// APIHost represents the host data received from the API
type APIHost struct {
	ID                  int       `json:"id"`
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

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// CreateGroupRequest represents a request to create a new group
type CreateGroupRequest struct {
	Name string `json:"name" binding:"required"`
}

// HostResponse represents a host response
type HostResponse struct {
	Name               string `json:"name"`
	Version            string `json:"version"`
	Status             string `json:"status"`
	Groups             string `json:"groups"`
	Inventory          string `json:"inventory"`
	DisabledComponents string `json:"disabled_components"`
	WantsUpdateTo      string `json:"wants_update_to"`
	UpForDeletion      bool   `json:"up_for_deletion"`
}

// GroupResponse represents a group response
type GroupResponse struct {
	ID    uint           `json:"id" example:"1"`
	Name  string         `json:"name" example:"developers"`
	Hosts []HostResponse `json:"hosts,omitempty"`
}

// UserResponse represents a user response
type UserResponse struct {
	ID          uint   `json:"id" example:"1"`
	Username    string `json:"username" example:"johndoe"`
	Email       string `json:"email" example:"john.doe@example.com"`
	Role        string `json:"role" example:"admin"`
	Groups      string `json:"groups" example:"developers,admins"`
	Inventories string `json:"inventories" example:"production"`
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
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Role      string `json:"role" binding:"required"`
	Groups    string `json:"groups"`
	Inventory string `json:"inventory"`
}

// UpdateUserGroupsRequest represents a request to update user groups
type UpdateUserGroupsRequest struct {
	Groups string `json:"groups" binding:"required"`
}

// UpdateMeRequest represents a request to update the current user
type UpdateMeRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Groups   string `json:"groups"`
}

// InventoryResponse represents an inventory response
type InventoryResponse struct {
	Name  string `json:"name"`
	Hosts []Host `json:"hosts"`
}

// CreateInventoryRequest represents a request to create a new inventory
type CreateInventoryRequest struct {
	Name string `json:"name" binding:"required"`
}
