package common

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupAdminRoutes(r *gin.Engine, db *gorm.DB) {
	admin := r.Group("/api/v1/admin")
	admin.Use(AuthMiddleware(db))
	{
		// Groups management
		admin.GET("/groups", func(c *gin.Context) {
			// Check if user is admin
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
		})

		admin.POST("/groups", func(c *gin.Context) {
			// Check if user is admin
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
		})

		admin.DELETE("/groups/:name", func(c *gin.Context) {
			// Check if user is admin
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
		})

		admin.POST("/groups/:name/hosts/:hostname", func(c *gin.Context) {
			// Check if user is admin
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
		})
	}
}
