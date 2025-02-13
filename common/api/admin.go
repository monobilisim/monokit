package common

import (
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

		group := Group{Name: req.Name}
		if err := db.Create(&group).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
			return
		}

		c.JSON(http.StatusOK, group)
	}
}

// @Summary Delete a group
// @Description Delete an existing group
// @Tags admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Group name"
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
		var group Group
		if result := db.Where("name = ?", groupName).First(&group); result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}

		if err := db.Delete(&group).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully"})
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

		// Create new user
		err := CreateUser(req.Username, req.Password, req.Email, req.Role, req.Groups, db)
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
			user.HashedPassword = hashedPassword
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
	}
}
