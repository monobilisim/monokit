//go:build with_api

package common

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	_ "github.com/monobilisim/monokit/docs"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// @title Monokit API
// @version 1.0
// @description Monokit API Server
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.monobilisim.com.tr
// @contact.email info@monobilisim.com.tr

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:9989
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

// @tag.name Logs
// @tag.description Operations related to logs

// APILogRequest represents a log entry submission request
type APILogRequest struct {
	Level     string `json:"level" binding:"required" example:"info" enums:"info,warning,error,critical"`
	Component string `json:"component" binding:"required" example:"system"`
	Message   string `json:"message" binding:"required" example:"System started successfully"`
	Timestamp string `json:"timestamp" example:"2023-01-01T12:00:00Z"`
	Metadata  string `json:"metadata" example:"{\"version\":\"1.2.3\"}"`
	Type      string `json:"type" example:"monokit"`
}

// APILogEntry represents a log entry in the database and response
type APILogEntry struct {
	ID        uint   `json:"id" example:"1"`
	HostName  string `json:"host_name" example:"server1"`
	Level     string `json:"level" example:"info"`
	Component string `json:"component" example:"system"`
	Message   string `json:"message" example:"System started successfully"`
	Timestamp string `json:"timestamp" example:"2023-01-01T12:00:00Z"`
	Metadata  string `json:"metadata" example:"{\"version\":\"1.2.3\"}"`
	Type      string `json:"type" example:"monokit"`
	CreatedAt string `json:"created_at" example:"2023-01-01T12:00:01Z"`
	UpdatedAt string `json:"updated_at" example:"2023-01-01T12:00:01Z"`
}

// APILogSearchRequest represents a log search request
type APILogSearchRequest struct {
	HostName    string `json:"host_name" example:"server1"`
	Level       string `json:"level" example:"error"`
	Component   string `json:"component" example:"database"`
	MessageText string `json:"message_text" example:"connection"`
	Type        string `json:"type" example:"monokit"`
	StartTime   string `json:"start_time" example:"2023-01-01T00:00:00Z"`
	EndTime     string `json:"end_time" example:"2023-01-31T23:59:59Z"`
	Page        int    `json:"page" example:"1"`
	PageSize    int    `json:"page_size" example:"100"`
}

// APILogPagination represents pagination information for log responses
type APILogPagination struct {
	Total    int64 `json:"total" example:"150"`
	Page     int   `json:"page" example:"1"`
	PageSize int   `json:"page_size" example:"100"`
	Pages    int64 `json:"pages" example:"2"`
}

// APILogsResponse represents a paginated list of logs
type APILogsResponse struct {
	Logs       []APILogEntry    `json:"logs"`
	Pagination APILogPagination `json:"pagination"`
}

// APIHostLogsResponse represents a paginated list of logs for a specific host
type APIHostLogsResponse struct {
	HostName   string           `json:"hostname" example:"server1"`
	Logs       []APILogEntry    `json:"logs"`
	Pagination APILogPagination `json:"pagination"`
}

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
	db.AutoMigrate(&HostLog{})

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
	// Setup Keycloak routes if enabled
	if ServerConfig.Keycloak.Enabled {
		SetupKeycloakRoutes(r, db)
	}
	r.POST("/api/v1/hosts", registerHost(db))
	SetupAdminRoutes(r, db)

	api := r.Group("/api/v1")
	// Apply Keycloak middleware first if enabled, then fall back to standard auth
	if ServerConfig.Keycloak.Enabled {
		api.Use(KeycloakAuthMiddleware(db))
	}
	api.Use(authMiddleware(db))
	{
		// Host management
		api.GET("/hosts/assigned", getAssignedHosts(db))
		api.GET("/hosts", getAllHosts(db))
		api.GET("/hosts/:name", getHostByName())
		api.DELETE("/hosts/:name", deleteHost(db))
		api.PUT("/hosts/:name", updateHost(db))
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

		// Log management - ensure these endpoints use the same auth chain
		api.GET("/logs", getAllLogs(db))
		api.GET("/logs/:hostname", getHostLogs(db))
		api.POST("/logs/search", searchLogs(db))
		api.DELETE("/logs/:id", deleteLog(db))
	}

	// Host-specific API that uses host token authentication
	hostApi := r.Group("/api/v1/host")
	hostApi.Use(hostAuthMiddleware(db))
	{
		// Allow hosts to submit their logs
		hostApi.POST("/logs", submitHostLog(db))
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
		// Check if user is already set in context by Keycloak middleware
		if user, exists := c.Get("user"); exists {
			// User is already authenticated by Keycloak
			fmt.Printf("User already authenticated by Keycloak: %v for path: %s\n",
				user.(User).Username, c.Request.URL.Path)
			c.Next()
			return
		}

		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		fmt.Printf("Processing request with token for path: %s\n", c.Request.URL.Path)

		// Extract token if it's a Bearer token (remove "Bearer " prefix)
		tokenValue := token
		if strings.HasPrefix(token, "Bearer ") {
			tokenValue = strings.TrimPrefix(token, "Bearer ")
			fmt.Printf("Found Bearer token, extracted value for path: %s\n", c.Request.URL.Path)

			// For Bearer tokens, if Keycloak is enabled, we should try to validate as Keycloak token first
			if ServerConfig.Keycloak.Enabled {
				// Attempt Keycloak authentication
				authAttempt := attemptKeycloakAuth(tokenValue, db, c)
				if authAttempt {
					// Successfully authenticated with Keycloak
					fmt.Println("Successfully authenticated with Keycloak via authMiddleware")
					c.Next()
					return
				}

				// If Keycloak auth failed and local auth is disabled
				if ServerConfig.Keycloak.DisableLocalAuth {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Keycloak authentication required"})
					c.Abort()
					return
				}
			}
		}

		// If Keycloak is enabled and local auth is disabled, reject non-JWT tokens
		if ServerConfig.Keycloak.Enabled && ServerConfig.Keycloak.DisableLocalAuth {
			// Try to validate as JWT before rejecting
			_, err := jwt.Parse(tokenValue, func(token *jwt.Token) (interface{}, error) {
				// We're just checking if it's a valid JWT format, not validating signature here
				return nil, fmt.Errorf("just checking format")
			})

			if err != nil && !strings.Contains(err.Error(), "just checking format") {
				// Not a valid JWT format and local auth is disabled
				fmt.Printf("Not a valid JWT token and local auth is disabled for path: %s\n", c.Request.URL.Path)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Keycloak authentication required"})
				c.Abort()
				return
			}
		}

		// Standard session-based auth - try with the raw token value
		var session Session
		if err := db.Preload("User").Where("token = ?", tokenValue).First(&session).Error; err != nil {
			// Also try with the full token including "Bearer " if applicable
			if strings.HasPrefix(token, "Bearer ") && err != nil {
				if err := db.Preload("User").Where("token = ?", token).First(&session).Error; err != nil {
					fmt.Printf("Invalid token for path: %s - %v\n", c.Request.URL.Path, err)
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
					c.Abort()
					return
				}
			} else {
				fmt.Printf("Invalid token for path: %s - %v\n", c.Request.URL.Path, err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				c.Abort()
				return
			}
		}

		// Check if session has expired
		if time.Now().After(session.Timeout) {
			db.Delete(&session)
			fmt.Printf("Token expired for path: %s\n", c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
			c.Abort()
			return
		}

		fmt.Printf("Authenticated session user: %s for path: %s\n",
			session.User.Username, c.Request.URL.Path)
		c.Set("user", session.User)
		c.Next()
	}
}

// Host authentication middleware
func hostAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		var hostKey HostKey
		if err := db.Where("token = ?", token).First(&hostKey).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid host token"})
			c.Abort()
			return
		}

		// Set the host name in the context for use in handlers
		c.Set("hostname", hostKey.HostName)
		c.Next()
	}
}

// Admin authentication middleware
func adminAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is already set in context by Keycloak middleware
		if user, exists := c.Get("user"); exists {
			// User is already authenticated, check if admin
			currentUser := user.(User)
			if currentUser.Role != "admin" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
				c.Abort()
				return
			}

			// User is admin, proceed
			c.Next()
			return
		}

		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Check if token is a Bearer token (Keycloak)
		if strings.HasPrefix(token, "Bearer ") {
			// If we get here and Keycloak is enabled, the token was invalid or user isn't set
			// We should already have checked this in the KeycloakAuthMiddleware
			if ServerConfig.Keycloak.Enabled && ServerConfig.Keycloak.DisableLocalAuth {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Keycloak token"})
				c.Abort()
				return
			}
			// Fall through to standard auth below if local auth is allowed
		}

		// If Keycloak is enabled and local auth is disabled, reject non-Bearer tokens
		if ServerConfig.Keycloak.Enabled && ServerConfig.Keycloak.DisableLocalAuth {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Keycloak authentication required"})
			c.Abort()
			return
		}

		// Standard session-based auth
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
		} else {
			// Check if the inventory exists, if not create it
			var inventory Inventory
			if err := db.Where("name = ?", host.Inventory).First(&inventory).Error; err != nil {
				// Create the inventory if it doesn't exist
				newInventory := Inventory{Name: host.Inventory}
				if err := db.Create(&newInventory).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create inventory"})
					return
				}
			}
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

// @Summary Delete log entry
// @Description Delete a log entry by its ID
// @Tags Logs
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Log ID"
// @Success 200 {object} map[string]string "Log deleted successfully"
// @Failure 400 {object} map[string]string "Invalid log id"
// @Failure 404 {object} map[string]string "Log not found"
// @Failure 500 {object} map[string]string "Failed to delete log entry"
func deleteLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log id"})
			return
		}
		var log HostLog
		if err := db.First(&log, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Log not found"})
			return
		}
		if err := db.Delete(&log).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log entry"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
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

		for _, host := range HostsList {
			if host.Name == name {
				components := strings.Split(host.DisabledComponents, "::")
				isDisabled := slices.Contains(components, service)
				c.JSON(http.StatusOK, gin.H{
					"name":     name,
					"service":  service,
					"disabled": isDisabled,
				})
				return
			}
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
	}
}

// @Summary Submit host log
// @Description Submit a log entry from a host
// @Tags logs
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param log body APILogRequest true "Log entry"
// @Success 201 {object} map[string]interface{} "Log entry saved response"
// @Failure 400 {object} map[string]string "Bad request error"
// @Failure 401 {object} map[string]string "Unauthorized error"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /host/logs [post]
func submitHostLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get hostname from context
		hostname, exists := c.Get("hostname")
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Hostname not found in context"})
			return
		}

		// Parse log data from request
		var logRequest APILogRequest
		if err := c.ShouldBindJSON(&logRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Use current time if timestamp not provided
		timestamp := logRequest.Timestamp
		if timestamp == "" {
			timestamp = time.Now().Format(time.RFC3339)
		}

		// Parse the timestamp string into time.Time
		parsedTime, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			// If parsing fails, use current time
			parsedTime = time.Now()
		}

		// Create entry
		logType := logRequest.Type

		if logType == "" {
			logType = "monokit"
		}

		log := HostLog{
			HostName:  hostname.(string),
			Level:     logRequest.Level,
			Component: logRequest.Component,
			Message:   logRequest.Message,
			Timestamp: parsedTime,
			Metadata:  logRequest.Metadata,
			Type:      logType,
		}

		// Count total logs
		var total int64
		if err := db.Model(&HostLog{}).Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count logs"})
			return
		}

		// If we have more than 10000 logs, delete the oldest ones
		if total >= 10000 {
			// Calculate how many logs to delete
			toDelete := total - 9999 // This ensures we'll have 9999 logs after deletion, allowing the new one to be the 10000th

			// Find the IDs of the oldest logs we need to delete
			var oldestLogs []HostLog
			if err := db.Order("timestamp asc").Limit(int(toDelete)).Find(&oldestLogs).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find oldest logs"})
				return
			}

			// Extract IDs
			var ids []uint
			for _, log := range oldestLogs {
				ids = append(ids, log.ID)
			}

			// Delete the oldest logs
			if err := db.Delete(&HostLog{}, ids).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete old logs"})
				return
			}
		}

		// Save to database
		if err := db.Create(&log).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save log entry"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "Log entry saved successfully"})
	}
}

// @Summary Get all logs
// @Description Retrieve all logs with pagination
// @Tags Logs
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param page_size query int false "Page size (default: 100, max: 1000)"
// @Success 200 {object} APILogsResponse "Paginated logs response"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /logs [get]
func getAllLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse pagination parameters
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "100")

		// Convert to integers
		pageInt, err := strconv.Atoi(pageStr)
		if err != nil || pageInt < 1 {
			pageInt = 1
		}

		pageSizeInt, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSizeInt < 1 {
			pageSizeInt = 100
		}

		// Limit page size to prevent excessive queries
		if pageSizeInt > 1000 {
			pageSizeInt = 1000
		}

		// Calculate offset
		offset := (pageInt - 1) * pageSizeInt

		// Count total logs
		var total int64
		db.Model(&HostLog{}).Count(&total)

		// Get logs with pagination
		var logs []HostLog
		if err := db.Order("timestamp desc").Offset(offset).Limit(pageSizeInt).Find(&logs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
			return
		}

		// Calculate total pages
		totalPages := (total + int64(pageSizeInt) - 1) / int64(pageSizeInt)

		// Convert to response format
		var logEntries []APILogEntry
		for _, log := range logs {
			logEntries = append(logEntries, APILogEntry{
				ID:        log.ID,
				HostName:  log.HostName,
				Level:     log.Level,
				Component: log.Component,
				Message:   log.Message,
				Timestamp: log.Timestamp.Format(time.RFC3339),
				Metadata:  log.Metadata,
				Type:      log.Type,
				CreatedAt: log.CreatedAt.Format(time.RFC3339),
				UpdatedAt: log.UpdatedAt.Format(time.RFC3339),
			})
		}

		// Return paginated response
		c.JSON(http.StatusOK, APILogsResponse{
			Logs: logEntries,
			Pagination: APILogPagination{
				Total:    total,
				Page:     pageInt,
				PageSize: pageSizeInt,
				Pages:    totalPages,
			},
		})
	}
}

// @Summary Get logs for a specific host
// @Description Retrieve logs for a specific host with pagination
// @Tags Logs
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param hostname path string true "Hostname"
// @Param page query int false "Page number (default: 1)"
// @Param page_size query int false "Page size (default: 100, max: 1000)"
// @Success 200 {object} APIHostLogsResponse "Paginated host logs response"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /logs/{hostname} [get]
func getHostLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get hostname from path parameter
		hostname := c.Param("hostname")
		if hostname == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Hostname is required"})
			return
		}

		// Parse pagination parameters
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "100")

		// Convert to integers
		pageInt, err := strconv.Atoi(pageStr)
		if err != nil || pageInt < 1 {
			pageInt = 1
		}

		pageSizeInt, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSizeInt < 1 {
			pageSizeInt = 100
		}

		// Limit page size to prevent excessive queries
		if pageSizeInt > 1000 {
			pageSizeInt = 1000
		}

		// Calculate offset
		offset := (pageInt - 1) * pageSizeInt

		// Count total logs for this host
		var total int64
		db.Model(&HostLog{}).Where("host_name = ?", hostname).Count(&total)

		// Get logs with pagination
		var logs []HostLog
		if err := db.Where("host_name = ?", hostname).Order("timestamp desc").Offset(offset).Limit(pageSizeInt).Find(&logs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
			return
		}

		// Calculate total pages
		totalPages := (total + int64(pageSizeInt) - 1) / int64(pageSizeInt)

		// Convert to response format
		var logEntries []APILogEntry
		for _, log := range logs {
			logEntries = append(logEntries, APILogEntry{
				ID:        log.ID,
				HostName:  log.HostName,
				Level:     log.Level,
				Component: log.Component,
				Message:   log.Message,
				Timestamp: log.Timestamp.Format(time.RFC3339),
				Metadata:  log.Metadata,
				Type:      log.Type,
				CreatedAt: log.CreatedAt.Format(time.RFC3339),
				UpdatedAt: log.UpdatedAt.Format(time.RFC3339),
			})
		}

		// Return paginated response
		c.JSON(http.StatusOK, APIHostLogsResponse{
			HostName: hostname,
			Logs:     logEntries,
			Pagination: APILogPagination{
				Total:    total,
				Page:     pageInt,
				PageSize: pageSizeInt,
				Pages:    totalPages,
			},
		})
	}
}

// @Summary Search logs
// @Description Search logs with various filters
// @Tags Logs
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param search body APILogSearchRequest true "Search parameters"
// @Success 200 {object} APILogsResponse "Paginated logs response"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /logs/search [post]
func searchLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse search parameters from request
		var searchRequest APILogSearchRequest
		if err := c.ShouldBindJSON(&searchRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Set default pagination values if not provided
		if searchRequest.Page < 1 {
			searchRequest.Page = 1
		}
		if searchRequest.PageSize < 1 {
			searchRequest.PageSize = 100
		}
		// Limit page size to prevent excessive queries
		if searchRequest.PageSize > 1000 {
			searchRequest.PageSize = 1000
		}

		// Calculate offset
		offset := (searchRequest.Page - 1) * searchRequest.PageSize

		// Build query with filters
		query := db.Model(&HostLog{})

		// Apply filters
		if searchRequest.HostName != "" {
			query = query.Where("LOWER(host_name) = LOWER(?)", searchRequest.HostName)
		}
		if searchRequest.Level != "" {
			query = query.Where("level = ?", searchRequest.Level)
		}
		if searchRequest.Component != "" {
			query = query.Where("component = ?", searchRequest.Component)
		}
		if searchRequest.MessageText != "" {
			query = query.Where("message LIKE ?", "%"+searchRequest.MessageText+"%")
		}
		if searchRequest.Type != "" {
			query = query.Where("type = ?", searchRequest.Type)
		}
		if searchRequest.StartTime != "" {
			startTime, err := time.Parse(time.RFC3339, searchRequest.StartTime)
			if err == nil {
				query = query.Where("timestamp >= ?", startTime)
			}
		}
		if searchRequest.EndTime != "" {
			endTime, err := time.Parse(time.RFC3339, searchRequest.EndTime)
			if err == nil {
				query = query.Where("timestamp <= ?", endTime)
			}
		}

		// Count total matching logs
		var total int64
		query.Count(&total)

		// Get logs with pagination
		var logs []HostLog
		if err := query.Order("timestamp desc").Offset(offset).Limit(searchRequest.PageSize).Find(&logs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
			return
		}

		// Calculate total pages
		totalPages := (total + int64(searchRequest.PageSize) - 1) / int64(searchRequest.PageSize)

		// Convert to response format
		var logEntries []APILogEntry
		for _, log := range logs {
			logEntries = append(logEntries, APILogEntry{
				ID:        log.ID,
				HostName:  log.HostName,
				Level:     log.Level,
				Component: log.Component,
				Message:   log.Message,
				Timestamp: log.Timestamp.Format(time.RFC3339),
				Metadata:  log.Metadata,
				Type:      log.Type,
				CreatedAt: log.CreatedAt.Format(time.RFC3339),
				UpdatedAt: log.UpdatedAt.Format(time.RFC3339),
			})
		}

		// Return paginated response
		c.JSON(http.StatusOK, APILogsResponse{
			Logs: logEntries,
			Pagination: APILogPagination{
				Total:    total,
				Page:     searchRequest.Page,
				PageSize: searchRequest.PageSize,
				Pages:    totalPages,
			},
		})
	}
}
