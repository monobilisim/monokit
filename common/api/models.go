	package common

import (
	"time"

	"gorm.io/gorm"
)

// @Description Error response
type ErrorResponse struct {
	Error string `json:"error" example:"Admin access required"`
}

// @Description Group model
type CreateGroupRequest struct {
	Name string `json:"name" binding:"required" example:"developers"`
}

// @Description Host response model
type HostResponse struct {
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
	Inventory           string    `json:"inventory"`
}

// @Description Group response model
type GroupResponse struct {
	ID    uint           `json:"id" example:"1"`
	Name  string         `json:"name" example:"developers"`
	Hosts []HostResponse `json:"hosts,omitempty"`
}

// @Description User response model
type UserResponse struct {
	ID          uint   `json:"id" example:"1"`
	Username    string `json:"username" example:"johndoe"`
	Email       string `json:"email" example:"john.doe@example.com"`
	Role        string `json:"role" example:"admin"`
	Groups      string `json:"groups" example:"developers,admins"`
	Inventories string `json:"inventories" example:"production"`
}

// @Description Login request
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"johndoe"`
	Password string `json:"password" binding:"required" example:"secretpassword123"`
}

// @Description Login response
type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  struct {
		Username string `json:"username" example:"johndoe"`
		Email    string `json:"email" example:"john.doe@example.com"`
		Role     string `json:"role" example:"admin"`
		Groups   string `json:"groups" example:"developers,admins"`
	} `json:"user"`
}

// @Description Register request
type RegisterRequest struct {
	Username  string `json:"username" binding:"required" example:"johndoe"`
	Password  string `json:"password" binding:"required" example:"secretpassword123"`
	Email     string `json:"email" binding:"required" example:"john.doe@example.com"`
	Role      string `json:"role" binding:"required" example:"user"`
	Groups    string `json:"groups" example:"developers"`
	Inventory string `json:"inventory" example:"production"`
}

// @Description Update user groups request
type UpdateUserGroupsRequest struct {
	Groups string `json:"groups" binding:"required" example:"developers,production"`
}

// @Description Update own user details request
type UpdateMeRequest struct {
	Username string `json:"username,omitempty" example:"johndoe"`
	Password string `json:"password,omitempty" example:"newpassword123"`
	Email    string `json:"email,omitempty" example:"john.doe@example.com"`
}

// @Description Update user request (admin)
type UpdateUserRequest struct {
	Username string `json:"username,omitempty" example:"johndoe"`
	Password string `json:"password,omitempty" example:"newpassword123"`
	Email    string `json:"email,omitempty" example:"john.doe@example.com"`
	Role     string `json:"role,omitempty" example:"admin"`
	Groups   string `json:"groups,omitempty" example:"developers,admins"`
}

// @Description Inventory response model
type InventoryResponse struct {
	Name  string `json:"name" example:"production"`
	Hosts int    `json:"hosts" example:"5"`
}

// @Description Create inventory request
type CreateInventoryRequest struct {
	Name string `json:"name" binding:"required" example:"production"`
}

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

// Add this with the other model definitions
type HostKey struct {
	gorm.Model
	Token    string `json:"token"`
	HostName string `json:"hostName" gorm:"unique"`
}

// Add this helper function
func generateToken() string {
	return GenerateRandomString(32)
}
