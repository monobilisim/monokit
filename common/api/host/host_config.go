//go:build with_api

package host

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"gorm.io/gorm"
)

// Type aliases for commonly used types from models package
type (
	HostFileConfig = models.HostFileConfig
)

// HandleGetHostConfig retrieves host configuration from the database.
func HandleGetHostConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host name is required"})
			return
		}

		var configs []HostFileConfig
		if err := db.Where("host_name = ?", name).Find(&configs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve host configurations"})
			return
		}

		// Convert array of configs to map of filename -> content
		configMap := make(map[string]string)
		for _, config := range configs {
			configMap[config.FileName] = config.Content
		}
		c.JSON(http.StatusOK, configMap)
	}
}

// HandlePostHostConfig creates or updates host configuration in the database.
func HandlePostHostConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host name is required"})
			return
		}

		var configMap map[string]string
		if err := c.ShouldBindJSON(&configMap); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		for fileName, content := range configMap {
			configData := HostFileConfig{
				HostName:  name,
				FileName:  fileName,
				Content:   content,
				UpdatedAt: time.Now(),
			}

			var existingConfig HostFileConfig
			if err := db.Where("host_name = ? AND file_name = ?", name, fileName).First(&existingConfig).Error; err == nil {
				// Update existing config
				configData.ID = existingConfig.ID // Preserve ID for update
				if err := db.Save(&configData).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update host configuration for file: " + fileName})
					return // Exit if update fails for any file
				}
			} else {
				configData.CreatedAt = time.Now()
				// Create new config
				if err := db.Create(&configData).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create host configuration for file: " + fileName})
					return // Exit if create fails for any file
				}
			}
		}

		c.JSON(http.StatusCreated, gin.H{"status": "created", "message": "configurations updated"})
	}
}

// HandlePutHostConfig updates host configuration in the database.
func HandlePutHostConfig(db *gorm.DB) gin.HandlerFunc {
	return HandlePostHostConfig(db) // Reuse Post handler for Put as per requirement
}

// HandleDeleteHostConfig deletes a specific host configuration file.
func HandleDeleteHostConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimSpace(c.Param("name"))
		filename := strings.TrimSpace(c.Param("filename"))

		if name == "" || filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host name and filename are required"})
			return
		}

		var config HostFileConfig
		if err := db.Where("host_name = ? AND file_name = ?", name, filename).First(&config).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "configuration not found for file: " + filename})
			return
		}

		if err := db.Delete(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete host configuration for file: " + filename})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "deleted", "message": "configuration file deleted: " + filename})
	}
}
