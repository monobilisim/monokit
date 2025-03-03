package common

import "time"

// Host represents a host in the system
type Host struct {
	Name                string    `json:"name"`
	CpuCores            int       `json:"cpu_cores"`
	Ram                 string    `json:"ram"`
	MonokitVersion      string    `json:"monokit_version"`
	Os                  string    `json:"os"`
	DisabledComponents  string    `json:"disabled_components"`
	InstalledComponents string    `json:"installed_components"`
	IpAddress           string    `json:"ip_address"`
	Status              string    `json:"status"`
	UpdatedAt           time.Time `json:"updated_at"`
	CreatedAt           time.Time `json:"created_at"`
	WantsUpdateTo       string    `json:"wants_update_to"`
	Groups              string    `json:"groups"`
	UpForDeletion       bool      `json:"up_for_deletion"`
	Inventory           string    `json:"inventory"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`
}

// InventoryResponse represents an inventory response from the API
type InventoryResponse struct {
	Name  string `json:"name"`
	Hosts []Host `json:"hosts"`
}
