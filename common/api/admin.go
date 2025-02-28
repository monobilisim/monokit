//go:build with_api

package common

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// @Summary List all groups
// @Description Get list of all groups
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {array} Group
// @Failure 403 {object} ErrorResponse
// @Router /admin/groups [get]
func listGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		var groups []Group
		if err := db.Preload("Hosts").Find(&groups).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch groups"})
			return
		}
		c.JSON(http.StatusOK, groups)
	}
}

// @Summary Create new group
// @Description Create a new group
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param group body CreateGroupRequest true "Group information"
// @Success 200 {object} Group
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /admin/groups [post]
func createGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		var req struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if group already exists
		var existingGroup Group
		if result := db.Where("name = ?", req.Name).First(&existingGroup); result.Error == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Group already exists"})
			return
		}

		group := Group{
			Name:  req.Name,
			Users: []User{}, // Initialize with empty slice instead of "nil" string
		}
		if err := db.Create(&group).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
			return
		}

		c.JSON(http.StatusCreated, group)
	}
}

// @Summary Delete a group
// @Description Delete an existing group and optionally its hosts
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Group name"
// @Param withHosts query boolean false "Delete associated hosts"
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /admin/groups/{name} [delete]
func deleteGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		groupName := c.Param("name")
		withHosts := c.Query("withHosts") == "true"

		var group Group
		if result := db.Where("name = ?", groupName).First(&group); result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}

		// Get hosts in this group
		var hosts []Host
		if err := db.Find(&hosts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch hosts"})
			return
		}

		for _, host := range hosts {
			groups := strings.Split(host.Groups, ",")
			hasGroup := false
			var newGroups []string

			for _, g := range groups {
				g = strings.TrimSpace(g)
				if g == groupName {
					hasGroup = true
				} else {
					newGroups = append(newGroups, g)
				}
			}

			if hasGroup {
				if withHosts {
					// Delete host if it belongs to this group
					if err := db.Delete(&host).Error; err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete host"})
						return
					}
				} else {
					// Update host's groups
					if len(newGroups) == 0 {
						host.Groups = "nil"
					} else {
						host.Groups = strings.Join(newGroups, ",")
					}
					if err := db.Save(&host).Error; err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update host groups"})
						return
					}
				}
			}
		}

		// Update users that reference this group
		var users []User
		if err := db.Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}

		for _, user := range users {
			groups := strings.Split(user.Groups, ",")
			var newGroups []string
			for _, group := range groups {
				group = strings.TrimSpace(group)
				if group != groupName {
					newGroups = append(newGroups, group)
				}
			}
			if len(newGroups) == 0 {
				user.Groups = "nil"
			} else {
				user.Groups = strings.Join(newGroups, ",")
			}
			if err := db.Save(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user groups"})
				return
			}
		}

		// Finally delete the group
		if err := db.Delete(&group).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Group and associated resources deleted successfully"})
	}
}

// @Summary Add host to group
// @Description Add a host to a group
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Group name"
// @Param hostname path string true "Host name"
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /admin/groups/{name}/hosts/{hostname} [post]
func addHostToGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		groupName := c.Param("name")
		hostname := c.Param("hostname")

		var group Group
		if result := db.Where("name = ?", groupName).First(&group); result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}

		var host Host
		if result := db.Where("name = ?", hostname).First(&host); result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		// Add host to group
		if err := db.Model(&group).Association("Hosts").Append(&host); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add host to group"})
			return
		}

		// Update host's Groups field
		currentGroups := strings.Split(host.Groups, ",")
		if !slices.Contains(currentGroups, groupName) {
			if host.Groups == "" || host.Groups == "nil" {
				host.Groups = groupName
			} else {
				host.Groups = host.Groups + "," + groupName
			}
			if err := db.Save(&host).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update host groups"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Host added to group successfully"})
	}
}

// @Summary Remove host from group
// @Description Remove a host from a group
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Group name"
// @Param hostname path string true "Host name"
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /admin/groups/{name}/hosts/{hostname} [delete]
func removeHostFromGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		groupName := c.Param("name")
		hostname := c.Param("hostname")

		var group Group
		if result := db.Where("name = ?", groupName).First(&group); result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}

		var host Host
		if result := db.Where("name = ?", hostname).First(&host); result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		// Remove host from group
		if err := db.Model(&group).Association("Hosts").Delete(&host); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove host from group"})
			return
		}

		// Update host's Groups field
		currentGroups := strings.Split(host.Groups, ",")
		updatedGroups := make([]string, 0)
		for _, g := range currentGroups {
			if strings.TrimSpace(g) != groupName {
				updatedGroups = append(updatedGroups, strings.TrimSpace(g))
			}
		}

		if len(updatedGroups) > 0 {
			host.Groups = strings.Join(updatedGroups, ",")
		} else {
			host.Groups = "nil"
		}

		if err := db.Save(&host).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update host groups"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Host removed from group successfully"})
	}
}

// @Summary Update user groups
// @Description Update user's group memberships
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param groups body UpdateUserGroupsRequest true "Groups to assign"
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Router /admin/users/{username}/groups [put]
func updateUserGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		username := c.Param("username")
		var req UpdateUserGroupsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Update user groups
		result := db.Model(&User{}).Where("username = ?", username).Update("groups", req.Groups)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user groups"})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User groups updated successfully"})
	}
}

// @Summary Create new user
// @Description Create a new user (admin only)
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param user body RegisterRequest true "User registration info"
// @Success 201 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /admin/users [post]
func createUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if user already exists
		var existingUser User
		if result := db.Where("username = ?", req.Username).First(&existingUser); result.Error == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}

		// Create new user with inventory
		err := CreateUser(req.Username, req.Password, req.Email, req.Role, req.Groups, req.Inventory, db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
	}
}

// @Summary Delete user
// @Description Delete a user (cannot delete own account)
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /admin/users/{username} [delete]
func deleteUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, exists := c.Get("user")
		if !exists || currentUser.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		username := c.Param("username")

		// Prevent deleting own account
		if username == currentUser.(User).Username {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete your own account"})
			return
		}

		var userToDelete User
		if result := db.Where("username = ?", username).First(&userToDelete); result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		if err := db.Delete(&userToDelete).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
	}
}

// @Summary Update user details
// @Description Update any user's details (admin only)
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param user body UpdateUserRequest true "User details to update"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /admin/users/{username} [put]
func updateUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, exists := c.Get("user")
		if !exists || currentUser.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		username := c.Param("username")
		var req UpdateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var user User
		if err := db.Where("username = ?", username).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Check if new username is available (if being changed)
		if req.Username != "" && req.Username != user.Username {
			var existingUser User
			if result := db.Where("username = ?", req.Username).First(&existingUser); result.Error == nil {
				c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
				return
			}
			user.Username = req.Username
		}

		// Update other fields if provided
		if req.Password != "" {
			hashedPassword, err := HashPassword(req.Password)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			user.Password = hashedPassword
		}
		if req.Email != "" {
			user.Email = req.Email
		}
		if req.Role != "" {
			user.Role = req.Role
		}
		if req.Groups != "" {
			user.Groups = req.Groups
		}

		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
	}
}

// @Summary Get all users
// @Description Get list of all users (admin only)
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {array} UserResponse
// @Failure 403 {object} ErrorResponse
// @Router /admin/users [get]
func getAllUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		var users []User
		if err := db.Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}

		// Convert to response objects without sensitive data
		response := make([]UserResponse, len(users))
		for i, user := range users {
			response[i] = UserResponse{
				Username:    user.Username,
				Email:       user.Email,
				Role:        user.Role,
				Groups:      user.Groups,
				Inventories: user.Inventories,
			}
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Schedule host for deletion
// @Description Mark a host for deletion (admin only)
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param hostname path string true "Host name"
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /admin/hosts/{hostname} [delete]
func scheduleHostDeletion(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		hostname := c.Param("hostname")
		var host Host
		if err := db.Where("name = ?", hostname).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		// Update the upForDeletion flag
		host.UpForDeletion = true
		if err := db.Save(&host).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to schedule host for deletion"})
			return
		}

		// Update cache
		db.Find(&HostsList)

		c.JSON(http.StatusOK, gin.H{"message": "Host scheduled for deletion"})
	}
}

// @Summary Move host to inventory
// @Description Move a host to a different inventory (admin only)
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param hostname path string true "Host name"
// @Param inventory path string true "Target inventory name"
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /admin/hosts/{hostname}/move/{inventory} [post]
func moveHostToInventory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		hostname := c.Param("hostname")
		targetInventory := c.Param("inventory")

		// Check if target inventory exists
		var inventory Inventory
		if err := db.Where("name = ?", targetInventory).First(&inventory).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Target inventory not found"})
			return
		}

		// Find and update the host
		var host Host
		if err := db.Where("name = ?", hostname).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		host.Inventory = targetInventory
		if err := db.Save(&host).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to move host"})
			return
		}

		// Update the hosts list
		db.Find(&HostsList)

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Host %s moved to inventory %s", hostname, targetInventory)})
	}
}

// @Summary Get user by username
// @Description Get specific user information (admin only)
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 200 {object} UserResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /admin/users/{username} [get]
func getUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, exists := c.Get("user"); !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		username := c.Param("username")
		var user User
		if err := db.Where("username = ?", username).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, UserResponse{
			Username:    user.Username,
			Email:       user.Email,
			Role:        user.Role,
			Groups:      user.Groups,
			Inventories: user.Inventories,
		})
	}
}

func SetupAdminRoutes(r *gin.Engine, db *gorm.DB) {
	admin := r.Group("/api/v1/admin")
	admin.Use(AuthMiddleware(db))
	{
		admin.GET("/groups", listGroups(db))
		admin.POST("/groups", createGroup(db))
		admin.DELETE("/groups/:name", deleteGroup(db))
		admin.POST("/groups/:name/hosts/:hostname", addHostToGroup(db))
		admin.DELETE("/groups/:name/hosts/:hostname", removeHostFromGroup(db))
		admin.PUT("/users/:username/groups", updateUserGroups(db))
		admin.POST("/users", createUser(db))
		admin.DELETE("/users/:username", deleteUser(db))
		admin.PUT("/users/:username", updateUser(db))
		admin.GET("/users", getAllUsers(db))
		admin.DELETE("/hosts/:hostname", scheduleHostDeletion(db))
		admin.POST("/hosts/:hostname/move/:inventory", moveHostToInventory(db))
		admin.GET("/users/:username", getUser(db))
	}
}
