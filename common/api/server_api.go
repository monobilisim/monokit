//go:build with_api

package common

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/monobilisim/monokit/docs"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// StartAPIServer starts the API server
func StartAPIServer(cmd *cobra.Command, args []string) error {
	r := gin.Default()
	db := setupDatabase()
	setupRoutes(r, db)
	SetupFrontend(r) // This will be a no-op if frontend is not included
	return r.Run(fmt.Sprintf(":%s", ServerConfig.Port))
}

func setupDatabase() *gorm.DB {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		ServerConfig.Postgres.Host,
		ServerConfig.Postgres.User,
		ServerConfig.Postgres.Password,
		ServerConfig.Postgres.Dbname,
		ServerConfig.Postgres.Port,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database")
	}

	// Auto migrate the schema in the correct order
	db.AutoMigrate(&Inventory{}) // First create Inventory table
	db.AutoMigrate(&Host{})      // Then Host table that references Inventory
	db.AutoMigrate(&User{})
	db.AutoMigrate(&HostKey{})
	db.AutoMigrate(&Session{})
	db.AutoMigrate(&Group{})

	// Create default inventory if it doesn't exist
	var defaultInventory Inventory
	if db.Where("name = ?", "default").First(&defaultInventory).Error == gorm.ErrRecordNotFound {
		db.Create(&Inventory{Name: "default"})
	}

	// Create initial admin user if no users exist
	if err := createInitialAdmin(db); err != nil {
		fmt.Printf("Warning: Failed to create initial admin user: %v\n", err)
	}

	// Load all hosts into memory
	db.Find(&HostsList)

	return db
}

func setupRoutes(r *gin.Engine, db *gorm.DB) {
	// Setup API routes first
	// Swagger route is already set up in server.go
	SetupAuthRoutes(r, db)
	r.POST("/api/v1/hosts", registerHost(db))
	SetupAdminRoutes(r, db)

	api := r.Group("/api/v1")
	api.Use(authMiddleware(db))
	{
		// Host management
		api.GET("/hosts", getAllHosts(db))
		api.GET("/hosts/:name", getHostByName())
		api.DELETE("/hosts/:name", deleteHost(db))
		api.PUT("/hosts/:name", updateHost(db))
		api.GET("/hosts/assigned", getAssignedHosts(db))
		api.POST("/hosts/:name/updateTo/:version", updateHostVersion(db))
		api.POST("/hosts/:name/enable/:service", enableComponent(db))
		api.POST("/hosts/:name/disable/:service", disableComponent(db))
		api.GET("/hosts/:name/:service", getComponentStatus())

		// Group management
		api.GET("/groups", getAllGroups(db))

		// Inventory management
		api.GET("/inventory", getAllInventories(db))
		api.POST("/inventory", createInventory(db))
		api.DELETE("/inventory/:name", deleteInventory(db))
	}
}

// Helper function to generate random token
func generateToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

// Authentication middleware
func authMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		var session Session
		if err := db.Preload("User").Where("token = ?", token).First(&session).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Check if session has expired
		if time.Now().After(session.Timeout) {
			db.Delete(&session)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
			c.Abort()
			return
		}

		c.Set("user", session.User)
		c.Next()
	}
}

// Admin authentication middleware
func adminAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		var session Session
		if err := db.Preload("User").Where("token = ?", token).First(&session).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Check if session has expired
		if time.Now().After(session.Timeout) {
			db.Delete(&session)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
			c.Abort()
			return
		}

		if session.User.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Set("user", session.User)
		c.Next()
	}
}

// Handler functions
func registerHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var host Host
		if err := c.ShouldBindJSON(&host); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if host.Inventory == "" {
			host.Inventory = "default"
		}

		var existingHost Host
		result := db.Where("name = ?", host.Name).First(&existingHost)
		if result.Error == nil {
			token := c.GetHeader("Authorization")
			if token == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required for existing host"})
				return
			}

			var hostKey HostKey
			if err := db.Where("host_name = ? AND token = ?", host.Name, token).First(&hostKey).Error; err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid host key"})
				return
			}

			if host.Inventory == "" {
				host.Inventory = existingHost.Inventory
			}

			host.ID = existingHost.ID
			host.UpForDeletion = existingHost.UpForDeletion
			if err := db.Model(&existingHost).Updates(&host).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update host"})
				return
			}
			db.Find(&HostsList)
			c.JSON(http.StatusOK, gin.H{"host": host})
			return
		}

		if err := db.Create(&host).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create host"})
			return
		}

		token := generateToken()
		hostKey := HostKey{
			Token:    token,
			HostName: host.Name,
		}

		if err := db.Create(&hostKey).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create host key"})
			return
		}

		db.Find(&HostsList)
		c.JSON(http.StatusCreated, gin.H{
			"host":   host,
			"apiKey": token,
		})
	}
}

// Get all hosts handler
func getAllHosts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		for i := range HostsList {
			syncHostGroups(db, &HostsList[i])
			db.Save(&HostsList[i])
		}

		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		currentUser := user.(User)
		var filteredHosts []Host

		if currentUser.Role == "admin" {
			filteredHosts = HostsList
		} else {
			for _, host := range HostsList {
				userInventories := strings.Split(currentUser.Inventories, ",")
				for _, inv := range userInventories {
					if host.Inventory == strings.TrimSpace(inv) {
						filteredHosts = append(filteredHosts, host)
						break
					}
				}
			}
		}

		for i := range filteredHosts {
			isOffline := time.Since(filteredHosts[i].UpdatedAt).Minutes() > 5
			if filteredHosts[i].UpForDeletion {
				if isOffline {
					if currentUser.Role == "admin" {
						db.Unscoped().Delete(&filteredHosts[i])
						continue
					}
				}
				filteredHosts[i].Status = "Scheduled for deletion"
			} else if isOffline {
				filteredHosts[i].Status = "Offline"
			}
		}

		c.JSON(http.StatusOK, filteredHosts)
	}
}

// Get host by name handler
func getHostByName() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		idx := slices.IndexFunc(HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		c.JSON(http.StatusOK, HostsList[idx])
	}
}

// Delete host handler
func deleteHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var host Host
		if err := db.Where("name = ?", name).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		if err := db.Delete(&host).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete host"})
			return
		}

		db.Find(&HostsList)
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

// Update host handler
func updateHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var host Host
		if err := db.Where("name = ?", name).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		var updates Host
		if err := c.ShouldBindJSON(&updates); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Model(&host).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update host"})
			return
		}

		db.Find(&HostsList)
		c.JSON(http.StatusOK, host)
	}
}

// Get assigned hosts handler
func getAssignedHosts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		currentUser := user.(User)
		var filteredHosts []Host

		for _, host := range HostsList {
			userInventories := strings.Split(currentUser.Inventories, ",")
			for _, inv := range userInventories {
				if host.Inventory == strings.TrimSpace(inv) {
					filteredHosts = append(filteredHosts, host)
					break
				}
			}
		}

		c.JSON(http.StatusOK, filteredHosts)
	}
}

// Group management handlers
func getAllGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var groups []string
		for _, host := range HostsList {
			if host.Groups != "nil" {
				hostGroups := strings.Split(host.Groups, ",")
				for _, group := range hostGroups {
					group = strings.TrimSpace(group)
					if !slices.Contains(groups, group) {
						groups = append(groups, group)
					}
				}
			}
		}
		c.JSON(http.StatusOK, groups)
	}
}

// Inventory management handlers
func getAllInventories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var inventories []Inventory
		if err := db.Find(&inventories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inventories"})
			return
		}
		c.JSON(http.StatusOK, inventories)
	}
}

func createInventory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var inventory Inventory
		if err := c.ShouldBindJSON(&inventory); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Create(&inventory).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create inventory"})
			return
		}

		c.JSON(http.StatusCreated, inventory)
	}
}

func deleteInventory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var inventory Inventory
		if err := db.Where("name = ?", name).First(&inventory).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Inventory not found"})
			return
		}

		if err := db.Delete(&inventory).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete inventory"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

// Helper function to sync host groups
func syncHostGroups(db *gorm.DB, host *Host) {
	if host.Groups == "" {
		host.Groups = "nil"
	}
}

// @Summary Update host version
// @Description Set the version that a host should update to
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Param version path string true "Version to update to"
// @Success 200 {object} map[string]string
// @Router /hosts/{name}/updateTo/{version} [post]
func updateHostVersion(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		version := c.Param("version")

		db.Find(&HostsList)

		idx := slices.IndexFunc(HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		HostsList[idx].WantsUpdateTo = version

		// Update the host in the pgsql database
		db.Model(&Host{}).Where("name = ?", name).Updates(&HostsList[idx])
	}
}

// @Summary Enable component
// @Description Enable a component on a host
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Param service path string true "Service name"
// @Success 200 {object} map[string]string
// @Router /hosts/{name}/enable/{service} [post]
func enableComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		service := c.Param("service")

		db.Find(&HostsList)

		idx := slices.IndexFunc(HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		host := HostsList[idx]
		var enabled bool

		disabledComponents := strings.Split(host.DisabledComponents, "::")

		for j := 0; j < len(disabledComponents); j++ {
			if disabledComponents[j] == service {
				disabledComponents = append(disabledComponents[:j], disabledComponents[j+1:]...)
				c.JSON(http.StatusOK, gin.H{"status": "enabled"})
				enabled = true
			}
		}

		host.DisabledComponents = strings.Join(disabledComponents, "::")

		if host.DisabledComponents == "" {
			host.DisabledComponents = "nil"
		}

		// Update the host in the pgsql database
		db.Model(&Host{}).Where("name = ?", name).Updates(&host)

		// Sync the hosts list
		db.Find(&HostsList)

		if enabled {
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "already enabled"})
	}
}

// @Summary Disable component
// @Description Disable a component on a host
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Param service path string true "Service name"
// @Success 200 {object} map[string]string
// @Router /hosts/{name}/disable/{service} [post]
func disableComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		service := c.Param("service")

		db.Find(&HostsList)

		idx := slices.IndexFunc(HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		host := HostsList[idx]

		disabledComponents := strings.Split(host.DisabledComponents, "::")

		for j := 0; j < len(disabledComponents); j++ {
			if disabledComponents[j] == service {
				c.JSON(http.StatusOK, gin.H{"status": "already disabled"})
				return
			}
		}

		disabledComponents = append(disabledComponents, service)

		host.DisabledComponents = strings.Join(disabledComponents, "::")

		// Update the host in the pgsql database
		db.Model(&Host{}).Where("name = ?", name).Updates(&host)

		// Sync the hosts list
		db.Find(&HostsList)

		c.JSON(http.StatusOK, gin.H{"status": "disabled"})
	}
}

// @Summary Get component status
// @Description Get the status of a component on a host
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Param service path string true "Service name"
// @Success 200 {object} map[string]string
// @Router /hosts/{name}/{service} [get]
func getComponentStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		service := c.Param("service")
		idx := slices.IndexFunc(HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		host := HostsList[idx]

		wantsUpdateTo := host.WantsUpdateTo
		disabledComponents := strings.Split(host.DisabledComponents, "::")
		for j := 0; j < len(disabledComponents); j++ {
			if disabledComponents[j] == service {
				c.JSON(http.StatusOK, gin.H{"status": "disabled", "wantsUpdateTo": wantsUpdateTo})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "enabled", "wantsUpdateTo": wantsUpdateTo})
	}
}
