//go:build with_api

package models

// Additional request types that were referenced in other packages

// AdminGroupResponse represents a group response for admin operations
type AdminGroupResponse struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Hosts []Host `json:"hosts"`
}

// UpdateUserGroupsRequest represents a request to update user groups
type UpdateUserGroupsRequest struct {
	Groups string `json:"groups" binding:"required"`
}

// CreateGroupRequest represents a request to create a new group
type CreateGroupRequest struct {
	Name string `json:"name" binding:"required"`
}
