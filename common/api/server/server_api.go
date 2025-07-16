//go:build with_api

package server

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/monobilisim/monokit/common/api/admin"
	"github.com/monobilisim/monokit/common/api/auth"
	"github.com/monobilisim/monokit/common/api/host"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/monobilisim/monokit/common/health"
	_ "github.com/monobilisim/monokit/docs"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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

	monokitHostname, err := os.Hostname()
	if err != nil {
		// Log the error but continue, as health fallback might not be critical for all setups
		log.Error().
			Str("component", "api").
			Str("operation", "get_hostname").
			Err(err).
			Msg("Failed to get Monokit server hostname. Health check fallback for self may not work as expected.")
		monokitHostname = "" // Set to empty or a placeholder if preferred
	}

	setupRoutes(r, db, monokitHostname)
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
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // Ensure table names are not pluralized by GORM
		},
	})
	if err != nil {
		panic("Failed to connect to database")
	}

	// Begin transaction for table creation
	tx := db.Begin()
	if tx.Error != nil {
		panic(fmt.Sprintf("Failed to begin transaction: %v", tx.Error))
	}

	// Auto migrate the rest of the schema in the correct order, using the transaction
	allModels := []interface{}{
		&APILogEntry{}, &Inventory{}, &Host{}, &User{}, &HostKey{},
		&Session{}, &Group{}, &HostLog{}, &HostFileConfig{}, &HostHealthData{},
	}
	if err := tx.AutoMigrate(allModels...); err != nil {
		tx.Rollback() // Rollback on error
		panic(fmt.Sprintf("Failed to migrate schema: %v", err))
	}

	// Add AwxOnly column to hosts table if it doesn't exist, using the transaction
	// This helps with backward compatibility for existing databases
	if err := tx.Exec("ALTER TABLE hosts ADD COLUMN IF NOT EXISTS awx_only boolean DEFAULT false").Error; err != nil {
		// Log warning but don't necessarily rollback, as it's a non-critical compatibility addition
		fmt.Printf("Warning: Failed to add awx_only column: %v\n", err)
	}

	// Add indexes for host_logs table to improve query performance, using the transaction
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_host_logs_deleted_at_timestamp ON host_logs (deleted_at, timestamp)").Error; err != nil {
		fmt.Printf("Warning: Failed to create index idx_host_logs_deleted_at_timestamp on host_logs: %v\n", err)
	}

	// Add index for timestamp alone for faster sorting, using the transaction
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_host_logs_timestamp ON host_logs (timestamp)").Error; err != nil {
		fmt.Printf("Warning: Failed to create index idx_host_logs_timestamp on host_logs timestamp: %v\n", err)
	}

	// Add index for the id column to speed up "WHERE id IN (...)" queries, using the transaction
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_host_logs_id ON host_logs (id)").Error; err != nil {
		fmt.Printf("Warning: Failed to create index idx_host_logs_id on host_logs id: %v\n", err)
	}

	// Verify the host_file_configs table exists and has the correct structure, using the transaction
	if err := tx.Exec("SELECT * FROM host_file_configs LIMIT 0").Error; err != nil {
		tx.Rollback() // Rollback if a critical table verification fails
		panic(fmt.Sprintf("Failed to verify host_file_configs table: %v", err))
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		panic(fmt.Sprintf("Failed to commit transaction: %v", err))
	}

	// Create default inventory if it doesn't exist
	var defaultInventory Inventory
	if db.Where("name = ?", "default").First(&defaultInventory).Error == gorm.ErrRecordNotFound {
		db.Create(&Inventory{Name: "default"})
	}

	// Create initial admin user if no users exist
	if err := auth.CreateInitialAdmin(db); err != nil {
		fmt.Printf("Warning: Failed to create initial admin user: %v\n", err)
	}

	// Load all hosts into memory
	db.Find(&models.HostsList)

	// Note: fixDuplicateHosts is now called from ServerMain in server.go

	return db
}

func setupRoutes(r *gin.Engine, db *gorm.DB, monokitHostname string) {
	// Setup API routes first
	// Swagger route is already set up in server.go
	auth.SetupAuthRoutes(r, db)
	// Setup Keycloak routes if enabled
	if ServerConfig.Keycloak.Enabled {
		auth.SetupKeycloakRoutes(r, db)
	}
	r.POST("/api/v1/hosts", RegisterHost(db))
	admin.SetupAdminRoutes(r, db)

	api := r.Group("/api/v1")
	// Apply Keycloak middleware first if enabled, then fall back to standard auth
	if ServerConfig.Keycloak.Enabled {
		api.Use(auth.KeycloakAuthMiddleware(db))
	}
	api.Use(AuthMiddleware(db))
	{
		// Host management
		api.GET("/hosts/assigned", GetAssignedHosts(db))
		api.GET("/hosts", GetAllHosts(db))
		api.GET("/hosts/:name", GetHostByName())
		api.DELETE("/hosts/:name", DeleteHost(db))
		api.DELETE("/hosts/:name/force", ForceDeleteHost(db))
		api.PUT("/hosts/:name", UpdateHost(db))
		api.GET("/hosts/:name/awx-jobs", GetHostAwxJobs(db))
		api.GET("/hosts/:name/awx-jobs/:jobID/logs", GetHostAwxJobLogs(db))
		api.GET("/hosts/:name/awx-job-templates", GetAwxJobTemplates(db))
		api.GET("/hosts/:name/awx-job-templates/:templateID", GetAwxJobTemplateDetails(db))
		api.POST("/hosts/:name/awx-jobs/execute", ExecuteAwxJob(db))
		api.POST("/hosts/:name/awx-workflow-jobs/execute", ExecuteAwxWorkflowJob(db))
		api.POST("/hosts/awx", CreateAwxHost(db))
		api.DELETE("/hosts/awx/:id", DeleteAwxHost(db))
		api.GET("/awx/jobs/:jobID", GetAwxJobStatus(db))
		api.GET("/awx/job-templates", GetAwxTemplatesGlobal(db))
		api.GET("/awx/workflow-templates", GetAwxWorkflowTemplatesGlobal(db))

		// Config endpoints - using handlers from host_config.go
		api.GET("/hosts/:name/config", host.HandleGetHostConfig(db))                 // GET config - get all configs for a host
		api.POST("/hosts/:name/config", host.HandlePostHostConfig(db))               // POST config - create or update host configs
		api.PUT("/hosts/:name/config", host.HandlePutHostConfig(db))                 // PUT config - update host configs (same as POST)
		api.DELETE("/hosts/:name/config/:filename", host.HandleDeleteHostConfig(db)) // DELETE config - delete a specific config file

		api.POST("/hosts/:name/updateTo/:version", UpdateHostVersion(db))
		api.POST("/hosts/:name/enable/:service", EnableComponent(db))
		api.POST("/hosts/:name/disable/:service", DisableComponent(db))
		api.GET("/hosts/:name/status/:service", GetComponentStatus())

		// Add direct component status route (for compatibility with frontend)
		api.GET("/hosts/:name/:component", GetComponentStatus())

		// Group management
		api.GET("/groups", GetAllGroups(db))

		// Inventory management
		api.GET("/inventory", GetAllInventories(db))
		api.POST("/inventory", CreateInventory(db))
		api.DELETE("/inventory/:name", DeleteInventory(db))

		// Log management - ensure these endpoints use the same auth chain
		api.GET("/logs", GetAllLogs(db))
		api.GET("/logs/hourly", GetHourlyLogStats(db))
		api.GET("/logs/:hostname", GetHostLogs(db))
		api.POST("/logs/search", SearchLogs(db))
		api.DELETE("/logs/:id", DeleteLog(db))

		// Health endpoints
		api.GET("/health/tools", GetHealthTools(db))
		api.GET("/hosts/:name/health", GetHostHealth(db, monokitHostname))
		api.GET("/hosts/:name/health/:tool", GetHostToolHealth(db, monokitHostname))
	}

	// Host-specific API that uses host token authentication
	hostApi := r.Group("/api/v1/host")
	hostApi.Use(HostAuthMiddleware(db))
	{
		// Config endpoints - with self-host auto-detection
		hostApi.GET("/config", host.HandleGetHostConfig(db))
		hostApi.PUT("/config", host.HandlePutHostConfig(db))

		// Status endpoints - make the parameter name more explicit
		hostApi.GET("/status/:service", GetComponentStatus()) // Changed from "/:service" to "/status/:service"

		// Allow hosts to submit their logs
		hostApi.POST("/logs", SubmitHostLog(db))
		// Allow hosts to submit their health data
		hostApi.POST("/health/:tool", PostHostHealth(db))
	}
}

// Helper function to generate random token
func GenerateToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

// Authentication middleware
func AuthMiddleware(db *gorm.DB) gin.HandlerFunc {
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
				authAttempt := auth.AttemptKeycloakAuth(tokenValue, db, c)
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
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("Not a valid JWT token and local auth is disabled for path: %s", c.Request.URL.Path),
					"title": "Authentication Error",
				})
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
func HostAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
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

		// Verify that the associated host still exists and is not soft-deleted
		var host Host
		if err := db.Where("name = ?", hostKey.HostName).First(&host).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Associated host not found or deleted"})
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
func RegisterHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Printf("Processing host registration request\n")

		// Parse host data from request body
		var host Host
		if err := c.ShouldBindJSON(&host); err != nil {
			fmt.Printf("Error parsing host registration body: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fmt.Printf("Registering host: %s, IP: %s, Inventory: %s\n",
			host.Name, host.IpAddress, host.Inventory)

		// Set default inventory if not provided
		if host.Inventory == "" {
			host.Inventory = "default"
			fmt.Printf("Using default inventory\n")
		} else {
			// Check if the inventory exists, if not create it
			var inventory Inventory
			if err := db.Where("name = ?", host.Inventory).First(&inventory).Error; err != nil {
				fmt.Printf("Creating new inventory: %s\n", host.Inventory)
				// Create the inventory if it doesn't exist
				newInventory := Inventory{Name: host.Inventory}
				if err := db.Create(&newInventory).Error; err != nil {
					fmt.Printf("Error creating inventory: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create inventory"})
					return
				}
			}
		}

		// Check if host already exists
		var existingHost Host
		result := db.Where("name = ?", host.Name).First(&existingHost)
		if result.Error == nil {
			fmt.Printf("Host already exists: %s (ID=%d)\n", existingHost.Name, existingHost.ID)

			// Verify authentication for existing host
			token := c.GetHeader("Authorization")
			if token == "" {
				fmt.Printf("Missing authentication token for existing host\n")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required for existing host"})
				return
			}

			// Verify host key
			var hostKey HostKey
			if err := db.Where("host_name = ? AND token = ?", host.Name, token).First(&hostKey).Error; err != nil {
				fmt.Printf("Invalid host key: %v\n", err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid host key"})
				return
			}
			fmt.Printf("Host key validation successful\n")

			// Preserve existing inventory if none specified
			if host.Inventory == "" {
				host.Inventory = existingHost.Inventory
			}

			// Preserve existing ID and deletion status
			host.ID = existingHost.ID
			host.UpForDeletion = existingHost.UpForDeletion

			// Update host
			if err := db.Model(&existingHost).Updates(&host).Error; err != nil {
				fmt.Printf("Error updating host: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update host"})
				return
			}

			fmt.Printf("Host updated successfully: %s (ID=%d)\n", host.Name, host.ID)

			// Refresh hosts list
			if err := db.Find(&models.HostsList).Error; err != nil {
				fmt.Printf("Warning: Error refreshing hosts list: %v\n", err)
			}

			c.JSON(http.StatusOK, gin.H{"host": host})
			return
		}

		// Create new host
		fmt.Printf("Creating new host: %s\n", host.Name)
		if err := db.Create(&host).Error; err != nil {
			fmt.Printf("Error creating host: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create host"})
			return
		}
		fmt.Printf("Host created successfully: %s (ID=%d)\n", host.Name, host.ID)

		// Generate and store API key for host
		token := GenerateToken()
		hostKey := HostKey{
			Token:    token,
			HostName: host.Name,
		}

		if err := db.Create(&hostKey).Error; err != nil {
			fmt.Printf("Error creating host key: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create host key"})
			return
		}
		fmt.Printf("Host key created successfully\n")

		// Refresh hosts list
		if err := db.Find(&models.HostsList).Error; err != nil {
			fmt.Printf("Warning: Error refreshing hosts list: %v\n", err)
		}

		c.JSON(http.StatusCreated, gin.H{
			"host":   host,
			"apiKey": token,
		})
	}
}

// Get all hosts handler
func GetAllHosts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		for i := range models.HostsList {
			syncHostGroups(db, &models.HostsList[i])
			db.Save(&models.HostsList[i])
		}

		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		currentUser := user.(User)
		var filteredHosts []Host

		// First filter out AWX-only hosts that should not be shown in dashboard
		var visibleHosts []Host
		for _, host := range models.HostsList {
			if !host.AwxOnly {
				visibleHosts = append(visibleHosts, host)
			}
		}

		if currentUser.Role == "admin" {
			filteredHosts = visibleHosts
		} else {
			for _, host := range visibleHosts {
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

			// If host is scheduled for deletion AND offline for 5 minutes, delete it
			if filteredHosts[i].UpForDeletion && isOffline {
				fmt.Printf("Deleting host '%s' (ID=%d) - scheduled for deletion and offline for 5+ minutes\n",
					filteredHosts[i].Name, filteredHosts[i].ID)

				// Use Unscoped().Delete to permanently remove the host
				db.Unscoped().Delete(&filteredHosts[i])
				continue
			}

			// Update status for remaining hosts
			if filteredHosts[i].UpForDeletion {
				filteredHosts[i].Status = "Scheduled for deletion"
			} else if isOffline {
				filteredHosts[i].Status = "Offline"
			}
		}

		c.JSON(http.StatusOK, filteredHosts)
	}
}

// Get host by name handler
func GetHostByName() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		fmt.Printf("Looking up host by name: %s\n", name)

		idx := slices.IndexFunc(models.HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			fmt.Printf("Host not found: %s\n", name)
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		fmt.Printf("Found host: %s (ID=%d)\n", models.HostsList[idx].Name, models.HostsList[idx].ID)
		c.JSON(http.StatusOK, models.HostsList[idx])
	}
}

// Delete host handler
func DeleteHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		fmt.Printf("Attempting to delete host: %s\n", name)

		var host Host
		if err := db.Where("name = ?", name).First(&host).Error; err != nil {
			fmt.Printf("Error finding host for deletion: %v\n", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		fmt.Printf("Found host for deletion: %s (ID=%d)\n", host.Name, host.ID)

		if err := db.Delete(&host).Error; err != nil {
			fmt.Printf("Error deleting host: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete host"})
			return
		}

		fmt.Printf("Host deleted successfully: %s (ID=%d)\n", host.Name, host.ID)

		// Refresh the hosts list
		db.Find(&models.HostsList)
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

// Create a host in AWX
func CreateAwxHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Parse request body
		var requestData struct {
			Name      string                 `json:"name" binding:"required"`
			IpAddress string                 `json:"ip_address" binding:"required"`
			ExtraVars map[string]interface{} `json:"extra_vars"`
			AwxOnly   bool                   `json:"awx_only"`
		}

		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		// Create a new HTTP client with timeout
		client := &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// AWX API endpoint for hosts
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/hosts/", awxURL)

		// Check if inventory ID is available in config
		if ServerConfig.Awx.DefaultInventoryID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "No default inventory ID configured. Please set 'default_inventory_id' in the server configuration.",
				"details": "The AWX API requires an inventory ID for creating hosts.",
			})
			return
		}

		// Prepare variables for AWX API
		variables := map[string]interface{}{
			"ansible_host": requestData.IpAddress,
		}

		// Add any extra variables if provided
		if len(requestData.ExtraVars) > 0 {
			for k, v := range requestData.ExtraVars {
				variables[k] = v
			}
		}

		// Convert variables to YAML string
		variablesYaml, err := yaml.Marshal(variables)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to format variables: " + err.Error()})
			return
		}

		// Prepare payload for AWX API
		payload := map[string]interface{}{
			"name":        requestData.Name,
			"variables":   string(variablesYaml),
			"enabled":     true,
			"instance_id": "",
			"inventory":   ServerConfig.Awx.DefaultInventoryID, // Inventory is required by AWX API
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal payload: " + err.Error()})
			return
		}

		// Create the request
		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
			return
		}

		// Set basic auth and headers
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
		req.Header.Set("Content-Type", "application/json")

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			// Read error response for debugging
			errorBody, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				// Handle error reading the body, though unlikely
				errorMsg := fmt.Sprintf("AWX API returned status: %d, but failed to read error body: %v", resp.StatusCode, readErr)
				fmt.Printf("AWX API error: %s\n", errorMsg)
				// Return the original status code even if body reading failed
				c.JSON(resp.StatusCode, gin.H{"error": errorMsg})
				return
			}

			// Log the error
			fmt.Printf("AWX API error: Status %d - %s\n", resp.StatusCode, string(errorBody))

			// Try to unmarshal the error body as JSON
			var awxErrorResponse interface{}
			err := json.Unmarshal(errorBody, &awxErrorResponse)
			if err == nil {
				// If unmarshalling is successful, return the JSON error object
				c.JSON(resp.StatusCode, awxErrorResponse)
			} else {
				// If unmarshalling fails, return the raw error string
				c.JSON(resp.StatusCode, gin.H{"error": string(errorBody)})
			}
			return
		}

		// Parse response
		var awxHostResponse map[string]interface{}
		respBody, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(respBody, &awxHostResponse); err != nil {
			errorMsg := fmt.Sprintf("Failed to decode response: %s. Raw response: %s", err.Error(), string(respBody))
			fmt.Printf("AWX response parsing error: %s\n", errorMsg)
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
			return
		}

		// Extract AWX host ID
		awxHostID, ok := awxHostResponse["id"].(float64)
		if !ok {
			log.Error().
				Str("component", "api").
				Str("operation", "extract_awx_host_id").
				Msg("Warning: Couldn't extract AWX host ID from response")
		}

		// We always need to create a minimal entry in the local database
		// This is necessary for the API to find the host for AWX job execution
		log.Debug().
			Str("component", "api").
			Str("operation", "create_local_database_entry").
			Str("host", requestData.Name).
			Msg("Creating local database entry for host")

		// Create the host in the local database
		localHost := Host{
			Name:      requestData.Name,
			IpAddress: requestData.IpAddress,
			Os:        "Unknown", // This can be updated later
			Status:    "Pending",
			Inventory: "default",                         // Use default inventory
			Groups:    "",                                // Can be populated later
			AwxHostId: fmt.Sprintf("%d", int(awxHostID)), // Store the AWX host ID as string
		}

		// If this is AWX-only, mark it as hidden from dashboard listings
		if requestData.AwxOnly {
			localHost.AwxOnly = true // This field needs to be added to the Host struct
		}

		// Check if host already exists
		var existingHost Host
		result := db.Where("name = ?", localHost.Name).First(&existingHost)
		if result.Error == nil {
			// Host already exists, update it
			fmt.Printf("Host already exists in local DB, updating: %s (ID=%d)\n",
				existingHost.Name, existingHost.ID)

			// Update existing host with new AWX data
			existingHost.IpAddress = localHost.IpAddress
			existingHost.AwxHostId = localHost.AwxHostId
			existingHost.Status = "Pending"
			if requestData.AwxOnly {
				existingHost.AwxOnly = true
			}

			if err := db.Save(&existingHost).Error; err != nil {
				fmt.Printf("Error updating existing host: %v\n", err)
				// Don't return error, continue with AWX host creation success
			}
		} else {
			// Create new host
			if err := db.Create(&localHost).Error; err != nil {
				fmt.Printf("Error creating local host entry: %v\n", err)
				// Don't return error, still return success for AWX host creation
			} else {
				fmt.Printf("Created local host entry: %s (ID=%d)\n", localHost.Name, localHost.ID)

				// Generate an API key for this host
				token := GenerateToken()
				hostKey := HostKey{
					Token:    token,
					HostName: localHost.Name,
				}

				if err := db.Create(&hostKey).Error; err != nil {
					fmt.Printf("Error creating host key: %v\n", err)
				} else {
					fmt.Printf("Created host key for %s\n", localHost.Name)

					// Include the API key in the response
					awxHostResponse["apiKey"] = token
				}
			}
		}

		// Refresh the hosts list
		if err := db.Find(&models.HostsList).Error; err != nil {
			fmt.Printf("Warning: Error refreshing hosts list: %v\n", err)
		}

		// Add local host information to response
		awxHostResponse["localHostRegistered"] = true
		if requestData.AwxOnly {
			awxHostResponse["awxOnly"] = true
		}

		c.JSON(http.StatusCreated, awxHostResponse)
	}
}

// Delete a host from AWX
func DeleteAwxHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Get host ID from URL
		hostID := c.Param("id")
		fmt.Printf("Deleting AWX host with ID: %s\n", hostID)

		// First, find if there's a local host with this AWX host ID
		var localHost Host
		result := db.Where("awx_host_id = ?", hostID).First(&localHost)

		// If first query fails, try as string comparison since AwxHostId is a string field
		if result.Error != nil {
			result = db.Where("awx_host_id = ?", fmt.Sprintf("%s", hostID)).First(&localHost)
		}
		if result.Error == nil {
			fmt.Printf("Found matching local host: %s (ID=%d)\n", localHost.Name, localHost.ID)

			// Delete the host keys associated with this host
			if err := db.Where("host_name = ?", localHost.Name).Delete(&HostKey{}).Error; err != nil {
				fmt.Printf("Warning: Error deleting host keys: %v\n", err)
			}

			// Delete host config files
			if err := db.Where("host_name = ?", localHost.Name).Delete(&HostFileConfig{}).Error; err != nil {
				fmt.Printf("Warning: Error deleting host config files: %v\n", err)
			}

			// Delete the local host entry
			if err := db.Unscoped().Delete(&localHost).Error; err != nil {
				fmt.Printf("Error deleting local host: %v\n", err)
			} else {
				fmt.Printf("Deleted local host: %s\n", localHost.Name)
			}

			// Refresh hosts list
			if err := db.Find(&models.HostsList).Error; err != nil {
				fmt.Printf("Warning: Error refreshing hosts list: %v\n", err)
			}
		} else {
			fmt.Printf("No matching local host found for AWX host ID: %s\n", hostID)
		}

		// Create a new HTTP client with timeout
		client := &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// AWX API endpoint for deleting host
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/hosts/%s/", awxURL, hostID)
		fmt.Printf("Calling AWX API to delete host: %s\n", apiURL)

		// Create the request
		req, err := http.NewRequest("DELETE", apiURL, nil)
		if err != nil {
			fmt.Printf("Error creating DELETE request: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
			return
		}

		// Set basic auth and headers
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error executing DELETE request: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Check response status
		fmt.Printf("AWX API DELETE response status: %d\n", resp.StatusCode)
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			// Read error response for debugging
			errorBody, _ := io.ReadAll(resp.Body)
			errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
			fmt.Printf("AWX API error: %s\n", errorMsg)
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "deleted",
			"details": map[string]interface{}{
				"awx_host_id":        hostID,
				"local_host_deleted": result.Error == nil,
			},
		})
	}
}

// Get all AWX job templates without requiring a host
func GetAwxTemplatesGlobal(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Create a new HTTP client with timeout and custom transport for better error handling
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !ServerConfig.Awx.VerifySSL,
			},
			// Add timeout settings for better diagnostics
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		}

		client := &http.Client{
			Timeout:   time.Duration(ServerConfig.Awx.Timeout) * time.Second,
			Transport: transport,
		}

		// AWX API endpoint for job templates
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/job_templates/", awxURL)

		// Initialize variables for collecting all templates
		var allResults []interface{}
		nextURL := apiURL

		// Follow pagination until there are no more pages
		for nextURL != "" {
			// Create the request
			req, err := http.NewRequest("GET", nextURL, nil)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
				return
			}

			// Set basic auth and headers
			req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
			req.Header.Set("Content-Type", "application/json")

			// Execute the request
			resp, err := client.Do(req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
				return
			}

			// Check response status
			if resp.StatusCode != http.StatusOK {
				// Read error response for debugging
				errorBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
				fmt.Printf("AWX API error: %s\n", errorMsg)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
				return
			}

			// Parse the JSON response
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response: " + err.Error()})
				return
			}
			resp.Body.Close()

			// Add the results from this page to our collection
			if results, ok := result["results"].([]interface{}); ok {
				allResults = append(allResults, results...)
				fmt.Printf("Added %d templates from page, total now: %d\n", len(results), len(allResults))
			}

			// Determine if there's a next page
			nextURL = ""
			if next, ok := result["next"].(string); ok && next != "" {
				if strings.HasPrefix(next, "/") {
					// Make sure we're using the base URL without any trailing slashes
					baseAwxURL := strings.TrimRight(awxURL, "/")
					nextURL = baseAwxURL + next
					fmt.Printf("Following pagination to next URL: %s\n", nextURL)
				} else if strings.HasPrefix(next, "http") {
					nextURL = next
					fmt.Printf("Following pagination to absolute next URL: %s\n", nextURL)
				} else {
					// Handle other URL formats or log the unexpected format
					fmt.Printf("Unexpected next URL format: %s\n", next)
				}
			}
		}

		// Return all collected results
		result := map[string]interface{}{
			"count":   len(allResults),
			"results": allResults,
		}
		c.JSON(http.StatusOK, result)
	}
}

// Execute an AWX Workflow Job
func ExecuteAwxWorkflowJob(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "AWX integration is not enabled",
				"title": "AWX Not Enabled",
				"code":  "AWX_NOT_ENABLED",
			})
			return
		}

		hostname := c.Param("name")
		fmt.Printf("Executing AWX workflow job for host: %s\n", hostname)

		// Parse request body with more lenient binding
		var requestData struct {
			WorkflowTemplateID   int                    `json:"workflow_template_id"`
			WorkflowTemplateName string                 `json:"workflow_template_name"`
			ExtraVars            map[string]interface{} `json:"extra_vars"`
			Format               string                 `json:"format"` // e.g. "yaml" or "json"
			InventoryID          int                    `json:"inventory_id"`
		}

		// Read the raw request body for debugging
		rawBody, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody)) // Reset the body

		fmt.Printf("Raw workflow request body: %s\n", string(rawBody))

		if err := c.ShouldBindJSON(&requestData); err != nil {
			fmt.Printf("Error parsing workflow request body: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid request: %v", err),
				"title": "Invalid Request Format",
				"code":  "INVALID_REQUEST_FORMAT",
			})
			return
		}

		// Log received data for debugging
		fmt.Printf("Parsed workflow request: template_id=%d, template_name=%s, format=%s, inventory_id=%d\n",
			requestData.WorkflowTemplateID, requestData.WorkflowTemplateName, requestData.Format, requestData.InventoryID)

		// Use template ID directly if provided or look up by name
		if requestData.WorkflowTemplateID <= 0 && requestData.WorkflowTemplateName != "" {
			// Check if we have a workflow template ID for this name in the configuration
			if ServerConfig.Awx.WorkflowTemplateIDs != nil {
				if id, exists := ServerConfig.Awx.WorkflowTemplateIDs[requestData.WorkflowTemplateName]; exists {
					fmt.Printf("Found workflow template ID %d for name '%s' in configuration\n", id, requestData.WorkflowTemplateName)
					requestData.WorkflowTemplateID = id
				} else {
					// Try some common alternative names or patterns
					templateName := strings.ToLower(strings.TrimSpace(requestData.WorkflowTemplateName))

					if strings.Contains(templateName, "setup") || strings.Contains(templateName, "install") {
						// Check for setup/install workflow templates
						for name, id := range ServerConfig.Awx.WorkflowTemplateIDs {
							if strings.Contains(strings.ToLower(name), "setup") ||
								strings.Contains(strings.ToLower(name), "install") {
								fmt.Printf("Using workflow template ID %d for '%s'\n", id, name)
								requestData.WorkflowTemplateID = id
								break
							}
						}
					}
				}
			}

			// If we still don't have a template ID, report an error with available options
			if requestData.WorkflowTemplateID <= 0 {
				fmt.Printf("No workflow template ID found for name: %s\n", requestData.WorkflowTemplateName)

				// Provide a helpful message listing available templates
				var availableTemplates []string
				if ServerConfig.Awx.WorkflowTemplateIDs != nil {
					for name := range ServerConfig.Awx.WorkflowTemplateIDs {
						availableTemplates = append(availableTemplates, name)
					}
				}

				c.JSON(http.StatusBadRequest, gin.H{
					"error":               fmt.Sprintf("No workflow template ID configured for '%s'", requestData.WorkflowTemplateName),
					"detail":              "Please configure the workflow template ID in server.yml or provide a valid workflow_template_id directly",
					"available_templates": availableTemplates,
					"title":               "Workflow Template Not Found",
					"code":                "WORKFLOW_TEMPLATE_NOT_FOUND",
				})
				return
			}
		}

		// If no specific template was provided and we have a default workflow template
		if requestData.WorkflowTemplateID <= 0 && ServerConfig.Awx.DefaultWorkflowTemplateID > 0 {
			fmt.Printf("Using default workflow template ID: %d\n", ServerConfig.Awx.DefaultWorkflowTemplateID)
			requestData.WorkflowTemplateID = ServerConfig.Awx.DefaultWorkflowTemplateID
		}

		// Validate we have a template ID at this point
		if requestData.WorkflowTemplateID <= 0 {
			fmt.Printf("No workflow template ID or name provided\n")
			c.JSON(http.StatusBadRequest, gin.H{
				"error":  "No workflow_template_id or workflow_template_name provided",
				"detail": "Please provide either workflow_template_id or workflow_template_name in the request",
				"title":  "Missing Template Information",
				"code":   "MISSING_TEMPLATE_INFO",
			})
			return
		}

		fmt.Printf("Will execute workflow template_id=%d for host %s\n", requestData.WorkflowTemplateID, hostname)

		// Check host existence
		var host Host
		if err := db.Where("name = ?", hostname).First(&host).Error; err != nil {
			fmt.Printf("Error finding host %s: %v\n", hostname, err)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Host not found",
				"title": "Host Not Found",
				"code":  "HOST_NOT_FOUND",
			})
			return
		}
		fmt.Printf("Found host: %s (ID=%d)\n", host.Name, host.ID)

		// Create a new HTTP client with timeout
		client := &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: !ServerConfig.Awx.VerifySSL,
				},
			},
		}

		// Create AWX API URL for workflow job templates
		awxURL := strings.TrimRight(ServerConfig.Awx.Url, "/")
		apiURL := fmt.Sprintf("%s/api/v2/workflow_job_templates/%d/launch/", awxURL, requestData.WorkflowTemplateID)

		// Determine inventory ID - use provided one or default from config
		inventoryID := requestData.InventoryID
		if inventoryID <= 0 {
			inventoryID = ServerConfig.Awx.DefaultInventoryID
			fmt.Printf("Using default inventory ID from config: %d\n", inventoryID)
		}

		if inventoryID <= 0 {
			// Handle this case more gracefully
			fmt.Printf("ERROR: No inventory ID provided or configured. This is required for AWX API calls.\n")

			// Return a more specific error that the frontend can handle
			c.JSON(http.StatusBadRequest, gin.H{
				"error":  "No inventory ID provided or configured",
				"detail": "Please set 'default_inventory_id' in the server configuration or provide an inventory_id in the request.",
				"title":  "Missing Inventory ID",
				"code":   "MISSING_INVENTORY_ID",
			})
			return
		}

		fmt.Printf("Executing AWX workflow job template ID %d for host %s with inventory ID %d\n",
			requestData.WorkflowTemplateID, hostname, inventoryID)

		// Find the host in database to get AWX host ID if available
		if host.AwxHostId != "" {
			fmt.Printf("Found host %s (AWX ID: %s) in database\n", hostname, host.AwxHostId)
		} else if host.IpAddress != "" {
			// Try to find or create AWX host if we have IP but no AWX ID
			fmt.Printf("Host %s not registered in AWX, will create it first\n", hostname)

			awxHostId, err := EnsureHostInAwx(db, host)
			if err != nil {
				fmt.Printf("Error ensuring host in AWX: %v\n", err)
				// Continue anyway as the job might still work with just hostname
			} else if awxHostId != "" {
				fmt.Printf("Successfully registered host in AWX with ID: %s\n", awxHostId)
				// Update the host record with AWX ID
				host.AwxHostId = awxHostId
				if err := db.Save(&host).Error; err != nil {
					fmt.Printf("Warning: Failed to update host with AWX ID: %v\n", err)
				}
			}
		}

		// Prepare payload for AWX API - keep it minimal for workflows
		// For workflow templates, we typically only need the inventory
		payload := map[string]interface{}{
			"inventory": inventoryID,
		}

		// Check if we need to add a limit based on workflow template configuration
		workflowSupportsLimit := false
		// You would ideally check this from AWX API but for now we can use a hardcoded list or config
		if workflowSupportsLimit {
			if host.Name != "" {
				payload["limit"] = host.Name
				fmt.Printf("Using limit: %s\n", host.Name)
			}
		}

		// If extra_vars are explicitly allowed for this template, include them
		// This would typically be stored in a config or determined by querying the AWX API
		isExtraVarsEnabled := false // Default to false for safety

		// Check if this specific template has extra_vars enabled
		if requestData.WorkflowTemplateID == 95 {
			// Template ID 95 is known not to support extra_vars at launch
			isExtraVarsEnabled = false
			fmt.Printf("Template ID 95 does not support extra_vars at launch\n")
		}

		// Only add extra_vars if the template supports them
		if isExtraVarsEnabled && requestData.ExtraVars != nil && len(requestData.ExtraVars) > 0 {
			fmt.Printf("Processing extra_vars for workflow: %+v\n", requestData.ExtraVars)
			var extraVarsStr string

			// Check if format is YAML, otherwise use JSON (default)
			format := strings.ToLower(requestData.Format)
			if format == "yaml" || format == "yml" {
				// Convert to YAML string
				extraVarsYAML, err := yaml.Marshal(requestData.ExtraVars)
				if err != nil {
					fmt.Printf("Error encoding extra_vars to YAML: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode extra_vars to YAML: " + err.Error()})
					return
				}
				extraVarsStr = string(extraVarsYAML)
				fmt.Printf("Converted workflow extra_vars to YAML: %s\n", extraVarsStr)
			} else {
				// Convert to JSON string (default for backward compatibility)
				extraVarsJSON, err := json.Marshal(requestData.ExtraVars)
				if err != nil {
					fmt.Printf("Error encoding extra_vars to JSON: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode extra_vars to JSON: " + err.Error()})
					return
				}
				extraVarsStr = string(extraVarsJSON)
				fmt.Printf("Converted workflow extra_vars to JSON: %s\n", extraVarsStr)
			}

			payload["extra_vars"] = extraVarsStr
			fmt.Printf("Added extra_vars to workflow payload\n")
		} else {
			// Don't include extra_vars at all if not supported
			fmt.Printf("Skipping extra_vars for workflow template %d as they're not enabled\n", requestData.WorkflowTemplateID)
		}

		// Convert payload to JSON
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			fmt.Printf("Error marshalling workflow payload: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to marshal payload: %v", err)})
			return
		}

		fmt.Printf("Sending request to AWX API: %s with workflow payload: %s\n", apiURL, string(payloadBytes))

		// Create the request
		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			fmt.Printf("Error creating workflow request: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create request: %v", err)})
			return
		}

		// Set request headers and authentication
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error executing workflow request: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to execute request: %v", err)})
			return
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error reading workflow response body: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read response body: %v", err)})
			return
		}

		// Check response status code (add title and code to error responses)
		if resp.StatusCode >= 400 {
			errMsg := fmt.Sprintf("AWX API returned error status %d: %s", resp.StatusCode, string(body))
			fmt.Printf("%s\n", errMsg)

			// Try to parse the error message for better user feedback
			var errorResponse map[string]interface{}
			var errorDetail string

			if err := json.Unmarshal(body, &errorResponse); err == nil {
				// Try to extract a specific error message if available
				if extraVarsErr, ok := errorResponse["extra_vars"].([]interface{}); ok && len(extraVarsErr) > 0 {
					errorDetail = fmt.Sprintf("Extra variables not allowed: %v", extraVarsErr[0])
				}
			}

			if errorDetail == "" {
				errorDetail = "Check that the workflow template exists and has valid parameters"
			}

			c.JSON(resp.StatusCode, gin.H{
				"error":   errMsg,
				"details": errorDetail,
				"title":   "AWX API Error",
				"code":    "AWX_API_ERROR",
			})
			return
		}

		// Parse response body
		var responseData map[string]interface{}
		if err := json.Unmarshal(body, &responseData); err != nil {
			fmt.Printf("Error parsing workflow response JSON: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse response body: %v", err)})
			return
		}

		// Extract job ID from response
		jobID, ok := responseData["id"]
		if !ok {
			fmt.Printf("No job ID returned from AWX workflow\n")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No job ID returned from AWX"})
			return
		}

		fmt.Printf("Successfully launched AWX workflow job with ID %v for host %s\n", jobID, hostname)

		// Return response to client
		c.JSON(http.StatusOK, gin.H{
			"job_id":               jobID,
			"status":               "launched",
			"host":                 hostname,
			"workflow_template_id": requestData.WorkflowTemplateID,
			"job_details":          responseData,
		})
	}
}

// Get the status of a job in AWX
func GetAwxJobStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Get job ID from URL
		jobID := c.Param("jobID")

		// Create a new HTTP client with timeout
		client := &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// AWX API endpoint for job status
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/jobs/%s/", awxURL, jobID)

		// Create the request
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
			return
		}

		// Set basic auth and headers
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
		req.Header.Set("Content-Type", "application/json")

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Read error response for debugging
			errorBody, _ := io.ReadAll(resp.Body)
			errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
			return
		}

		// Parse response
		var jobResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&jobResponse); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, jobResponse)
	}
}

// Force delete host handler - immediately and permanently deletes a host
func ForceDeleteHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var host Host
		if err := db.Where("name = ?", name).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		// Use Unscoped to bypass soft delete and permanently remove the host
		if err := db.Unscoped().Delete(&host).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to force delete host"})
			return
		}

		// Also delete any host keys associated with this host
		db.Where("host_name = ?", name).Unscoped().Delete(&HostKey{})

		// Also delete any host config files associated with this host
		db.Where("host_name = ?", name).Unscoped().Delete(&HostFileConfig{})

		db.Find(&models.HostsList)
		c.JSON(http.StatusOK, gin.H{"status": "force_deleted"})
	}
}

// Update host handler
func UpdateHost(db *gorm.DB) gin.HandlerFunc {
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

		db.Find(&models.HostsList)
		c.JSON(http.StatusOK, host)
	}
}

// Get assigned hosts handler
func GetAssignedHosts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			// DEBUG: Add temporary logging
			fmt.Printf("DEBUG: getAssignedHosts - No user found in context\n")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		// DEBUG: Log what type we got
		fmt.Printf("DEBUG: getAssignedHosts - User found in context, type: %T\n", user)

		// Try to cast to both User and models.User for compatibility
		var currentUser User
		var ok bool

		if currentUser, ok = user.(User); !ok {
			// Try models.User in case of type mismatch
			if modelsUser, modelsOk := user.(models.User); modelsOk {
				currentUser = User(modelsUser)
				fmt.Printf("DEBUG: getAssignedHosts - Successfully cast from models.User to User\n")
			} else {
				fmt.Printf("DEBUG: getAssignedHosts - Failed to cast user, type was: %T\n", user)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user type in context"})
				return
			}
		} else {
			fmt.Printf("DEBUG: getAssignedHosts - Successfully cast to User directly\n")
		}

		fmt.Printf("DEBUG: getAssignedHosts - User role: %s, inventories: %s\n", currentUser.Role, currentUser.Inventories)

		var filteredHosts []Host

		// First filter out AWX-only hosts that should not be shown in dashboard
		var visibleHosts []Host
		for _, host := range models.HostsList {
			if !host.AwxOnly {
				visibleHosts = append(visibleHosts, host)
			}
		}

		if currentUser.Role == "admin" {
			filteredHosts = visibleHosts
		} else {
			for _, host := range visibleHosts {
				userInventories := strings.Split(currentUser.Inventories, ",")
				for _, inv := range userInventories {
					if host.Inventory == strings.TrimSpace(inv) {
						filteredHosts = append(filteredHosts, host)
						break
					}
				}
			}
		}

		c.JSON(http.StatusOK, filteredHosts)
	}
}

// Group management handlers
func GetAllGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var groups []string
		for _, host := range models.HostsList {
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
func GetAllInventories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var inventories []Inventory
		if err := db.Find(&inventories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inventories"})
			return
		}
		c.JSON(http.StatusOK, inventories)
	}
}

func CreateInventory(db *gorm.DB) gin.HandlerFunc {
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

func DeleteInventory(db *gorm.DB) gin.HandlerFunc {
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
func UpdateHostVersion(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		version := c.Param("version")

		db.Find(&models.HostsList)

		idx := slices.IndexFunc(models.HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		models.HostsList[idx].WantsUpdateTo = version

		// Update the host in the pgsql database
		db.Model(&Host{}).Where("name = ?", name).Updates(&models.HostsList[idx])
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
func DeleteLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for admin authentication
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		currentUser := user.(User)
		if currentUser.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

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

// getHostAwxJobs fetches AWX jobs information for a specific host
// @Summary Get AWX jobs for a host
// @Description Get AWX jobs for a specific host by name
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Success 200 {array} map[string]interface{}
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Host not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /hosts/{name}/awx-jobs [get]
func GetHostAwxJobs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostname := c.Param("name")

		// Find the host in the database
		var host Host
		if err := db.Where("name = ?", hostname).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		// Check if AWX is enabled and host has AWX ID
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		if host.AwxHostId == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host has no AWX ID"})
			return
		}

		// Convert AWX host ID to integer
		awxHostID, err := strconv.Atoi(host.AwxHostId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid AWX host ID"})
			return
		}

		// Use the correct AWX API endpoint and parameter format
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/jobs/?hosts__id=%d", awxURL, awxHostID)

		// Create a new HTTP client with timeout
		client := &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// Create the request
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
			return
		}

		// Set basic auth
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
		req.Header.Set("Content-Type", "application/json")

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Read error response for debugging
			errorBody, _ := io.ReadAll(resp.Body)
			errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
			return
		}

		// Parse the response for direct jobs API
		var responseData struct {
			Count    int    `json:"count"`
			Next     string `json:"next"`
			Previous string `json:"previous"`
			Results  []struct {
				ID            int     `json:"id"`
				Name          string  `json:"name"`
				Status        string  `json:"status"`
				Failed        bool    `json:"failed"`
				Started       string  `json:"started"`
				Finished      string  `json:"finished"`
				Elapsed       float64 `json:"elapsed"`
				Type          string  `json:"type"`
				SummaryFields struct {
					JobTemplate struct {
						ID   int    `json:"id"`
						Name string `json:"name"`
					} `json:"job_template"`
				} `json:"summary_fields"`
			} `json:"results"`
		}

		// Decode JSON
		if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response: " + err.Error()})
			return
		}

		// Process jobs directly
		jobs := []gin.H{}

		for _, job := range responseData.Results {
			// Format job data according to required output format
			// Include URL for frontend linking to AWX interface
			jobData := gin.H{
				"id":                job.ID,
				"name":              job.Name,
				"status":            job.Status,
				"failed":            job.Failed,
				"started":           job.Started,
				"finished":          job.Finished,
				"elapsed":           job.Elapsed,
				"job_template_id":   job.SummaryFields.JobTemplate.ID,
				"job_template_name": job.SummaryFields.JobTemplate.Name,
				"type":              job.Type,
				"url":               fmt.Sprintf("%s/#/jobs/playbook/%d", strings.TrimSuffix(ServerConfig.Awx.Url, "/api"), job.ID),
			}

			jobs = append(jobs, jobData)
		}

		// No need to convert since we're already building the jobs slice directly

		c.JSON(http.StatusOK, jobs)
	}
}

// getHostAwxJobLogs fetches the logs for a specific AWX job
// @Summary Get logs for a specific AWX job
// @Description Get logs for a specific AWX job of a host
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Param jobID path integer true "AWX Job ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Host or job not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /hosts/{name}/awx-jobs/{jobID}/logs [get]
func GetHostAwxJobLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostname := c.Param("name")
		jobID := c.Param("jobID")

		// Find the host in the database
		var host Host
		if err := db.Where("name = ?", hostname).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Validate job ID
		if jobID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
			return
		}

		// Check if we should focus on host-specific logs
		focusOnHost := c.Query("focus_host") != "false" // Default to true if not specified

		// Use the correct AWX API endpoint for job logs with text download format
		// txt_download format ensures complete logs even for large jobs
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/jobs/%s/stdout/?format=txt_download", awxURL, jobID)

		// Create a new HTTP client with timeout
		client := &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// Create the request
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
			return
		}

		// Set basic auth
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
		req.Header.Set("Content-Type", "application/json")

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Read error response for debugging
			errorBody, _ := io.ReadAll(resp.Body)
			errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
			return
		}

		// Parse the response - AWX stdout response might be plain text or JSON
		contentType := resp.Header.Get("Content-Type")

		if strings.Contains(contentType, "application/json") {
			// Decode JSON response
			var logData map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&logData); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode logs: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, logData)
		} else {
			// Handle plain text response
			logs, err := io.ReadAll(resp.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read logs: " + err.Error()})
				return
			}

			logText := string(logs)

			// If focusOnHost is true, filter the logs to only show content related to this host
			if focusOnHost {
				logText = filterLogsForHost(logText, hostname)
			}

			c.JSON(http.StatusOK, gin.H{
				"job_id":   jobID,
				"logs":     logText,
				"format":   "plain_text",
				"filtered": focusOnHost,
			})
		}
	}
}

// getAwxJobTemplateDetails retrieves details of a specific job template from AWX
// @Summary Get AWX job template details
// @Description Get details of a specific job template from AWX including variables
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Param templateID path integer true "Template ID"
// @Success 200 {object} map[string]interface{} "Job template details"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Host or template not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /hosts/{name}/awx-job-templates/{templateID} [get]
func GetAwxJobTemplateDetails(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostname := c.Param("name")
		templateID := c.Param("templateID")

		// Find the host in the database
		var host Host
		if err := db.Where("name = ?", hostname).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Create a new HTTP client with timeout
		client := &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// AWX API endpoint for job template details
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/job_templates/%s/", awxURL, templateID)

		// Create the request
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
			return
		}

		// Set basic auth and headers
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
		req.Header.Set("Content-Type", "application/json")

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Read error response for debugging
			errorBody, _ := io.ReadAll(resp.Body)
			errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
			return
		}

		// Parse response for template details
		var templateDetails map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&templateDetails); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response: " + err.Error()})
			return
		}

		// Check if template has a survey
		surveyEnabled, exists := templateDetails["survey_enabled"]
		hasSurveyEnabled := false

		// Check if survey is enabled
		if exists {
			// Try to convert to boolean if it exists
			if boolVal, ok := surveyEnabled.(bool); ok {
				hasSurveyEnabled = boolVal
			}
		}

		// Store template variables
		variables := gin.H{}

		// If there are extra_vars, try to parse them
		if extraVars, exists := templateDetails["extra_vars"]; exists && extraVars != "" {
			extraVarsStr, ok := extraVars.(string)
			if ok && extraVarsStr != "" {
				var parsedVars map[string]interface{}
				if err := json.Unmarshal([]byte(extraVarsStr), &parsedVars); err == nil {
					variables["extra_vars"] = parsedVars
				} else {
					variables["extra_vars"] = extraVarsStr
				}
			}
		}

		// If survey is enabled, fetch survey details
		if hasSurveyEnabled {
			// Fetch survey spec
			surveyURL := fmt.Sprintf("%s/api/v2/job_templates/%s/survey_spec/", awxURL, templateID)
			surveyReq, err := http.NewRequest("GET", surveyURL, nil)
			if err == nil {
				surveyReq.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
				surveyReq.Header.Set("Content-Type", "application/json")

				surveyResp, err := client.Do(surveyReq)
				if err == nil && surveyResp.StatusCode == http.StatusOK {
					defer surveyResp.Body.Close()

					var surveySpec map[string]interface{}
					if err := json.NewDecoder(surveyResp.Body).Decode(&surveySpec); err == nil {
						variables["survey_spec"] = surveySpec
					}
				}
			}
		}

		// Prepare the response
		result := gin.H{
			"id":          templateDetails["id"],
			"name":        templateDetails["name"],
			"description": templateDetails["description"],
			"variables":   variables,
			"has_survey":  hasSurveyEnabled,
			"job_type":    templateDetails["job_type"],
			"created":     templateDetails["created"],
			"modified":    templateDetails["modified"],
		}

		c.JSON(http.StatusOK, result)
	}
}

// getAwxJobTemplates retrieves available job templates from AWX
// @Summary Get AWX job templates
// @Description Get available job templates from AWX for a specific host
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Success 200 {array} map[string]interface{} "List of job templates"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Host not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /hosts/{name}/awx-job-templates [get]
func GetAwxJobTemplates(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostname := c.Param("name")

		// Find the host in the database
		var host Host
		if err := db.Where("name = ?", hostname).First(&host).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}

		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Create a new HTTP client with timeout
		client := &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// AWX API endpoint for job templates
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/job_templates/", awxURL)

		// Create the request
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
			return
		}

		// Set basic auth and headers
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
		req.Header.Set("Content-Type", "application/json")

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Read error response for debugging
			errorBody, _ := io.ReadAll(resp.Body)
			errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
			return
		}

		// Parse response
		var responseData struct {
			Count    int    `json:"count"`
			Next     string `json:"next"`
			Previous string `json:"previous"`
			Results  []struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				URL         string `json:"url"`
			} `json:"results"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response: " + err.Error()})
			return
		}

		// Robust, case-insensitive, trimmed lookup for "manual-install-monokit-client"
		var foundTemplate *gin.H
		var availableNames []string
		templates := []gin.H{}

		// Pagination loop
		nextURL := apiURL
		client = &http.Client{
			Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}
		for nextURL != "" {
			req, err := http.NewRequest("GET", nextURL, nil)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
				return
			}
			req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
				return
			}
			if resp.StatusCode != http.StatusOK {
				errorBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
				c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
				return
			}

			var responseData struct {
				Count    int    `json:"count"`
				Next     string `json:"next"`
				Previous string `json:"previous"`
				Results  []struct {
					ID          int    `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
					URL         string `json:"url"`
				} `json:"results"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
				resp.Body.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response: " + err.Error()})
				return
			}
			resp.Body.Close()

			for _, template := range responseData.Results {
				availableNames = append(availableNames, template.Name)
				templateData := gin.H{
					"id":          template.ID,
					"name":        template.Name,
					"description": template.Description,
					"url":         template.URL,
				}
				templates = append(templates, templateData)
				if foundTemplate == nil && strings.EqualFold(strings.TrimSpace(template.Name), "manual-install-monokit-client") {
					t := templateData
					foundTemplate = &t
				}
			}

			// If there is a next page, follow it
			nextURL = ""
			if responseData.Next != "" {
				if strings.HasPrefix(responseData.Next, "/") {
					// Make sure we're using the base URL without any trailing slashes
					baseAwxURL := strings.TrimRight(awxURL, "/")
					nextURL = baseAwxURL + responseData.Next
					fmt.Printf("Following pagination to next URL: %s\n", nextURL)
				} else if strings.HasPrefix(responseData.Next, "http") {
					nextURL = responseData.Next
					fmt.Printf("Following pagination to absolute next URL: %s\n", nextURL)
				} else {
					// Handle other URL formats or log the unexpected format
					fmt.Printf("Unexpected next URL format: %s\n", responseData.Next)
				}
			}
		}

		if foundTemplate == nil {
			fmt.Printf("Job template 'manual-install-monokit-client' not found. Available templates: %v\n", availableNames)
		}

		c.JSON(http.StatusOK, templates)
	}
}

// Get all AWX workflow templates without requiring a host
func GetAwxWorkflowTemplatesGlobal(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Create a new HTTP client with timeout and custom transport for better error handling
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !ServerConfig.Awx.VerifySSL,
			},
			ResponseHeaderTimeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}
		client := &http.Client{
			Transport: transport,
			Timeout:   time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// AWX API endpoint for workflow templates
		awxURL := ServerConfig.Awx.Url
		apiURL := fmt.Sprintf("%s/api/v2/workflow_job_templates/", awxURL)

		// Initialize variables for collecting all templates
		var allResults []interface{}
		nextURL := apiURL

		// Follow pagination until there are no more pages
		for nextURL != "" {
			// Create the request
			req, err := http.NewRequest("GET", nextURL, nil)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
				return
			}

			// Set basic auth and headers
			req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
			req.Header.Set("Content-Type", "application/json")

			// Execute the request
			resp, err := client.Do(req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
				return
			}

			// Check response status
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				// Read error response for debugging
				errorBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
				c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
				return
			}

			// Parse response
			var responseData map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
				resp.Body.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response: " + err.Error()})
				return
			}
			resp.Body.Close()

			// Extract templates array from this page
			if results, ok := responseData["results"].([]interface{}); ok {
				allResults = append(allResults, results...)
				fmt.Printf("Added %d workflow templates from page, total now: %d\n", len(results), len(allResults))
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format from AWX"})
				return
			}

			// Determine if there's a next page
			nextURL = ""
			if next, ok := responseData["next"].(string); ok && next != "" {
				if strings.HasPrefix(next, "/") {
					// Make sure we're using the base URL without any trailing slashes
					baseAwxURL := strings.TrimRight(awxURL, "/")
					nextURL = baseAwxURL + next
					fmt.Printf("Following pagination to next URL: %s\n", nextURL)
				} else if strings.HasPrefix(next, "http") {
					nextURL = next
					fmt.Printf("Following pagination to absolute next URL: %s\n", nextURL)
				} else {
					// Handle other URL formats or log the unexpected format
					fmt.Printf("Unexpected next URL format: %s\n", next)
				}
			}
		}

		c.JSON(http.StatusOK, allResults)
	}
}

// executeAwxJob launches a job template on AWX for a specific host
// @Summary Execute an AWX job template
// @Description Launch a job template on AWX for a specific host
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Param job_data body map[string.interface{} true "Job execution parameters"
// @Success 200 {object} map[string.interface{} "Job launched successfully"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Host not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /hosts/{name}/awx-jobs/execute [post]

func ExecuteAwxJob(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostname := c.Param("name")
		log.Debug().
			Str("component", "api").
			Str("operation", "execute_awx_job").
			Str("host", hostname).
			Msg("Executing AWX job for host")

		// Check if host exists
		var host Host
		if err := db.Where("name = ?", hostname).First(&host).Error; err != nil {
			fmt.Printf("Error finding host %s: %v\n", hostname, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
			return
		}
		fmt.Printf("Found host: %s (ID=%d)\n", host.Name, host.ID)

		// Check if AWX is enabled
		if !ServerConfig.Awx.Enabled {
			fmt.Printf("AWX integration is not enabled in server config\n")
			c.JSON(http.StatusBadRequest, gin.H{"error": "AWX integration is not enabled"})
			return
		}

		// Parse request body with more lenient binding
		var requestData struct {
			TemplateID   int                    `json:"template_id"`
			TemplateName string                 `json:"template_name"`
			ExtraVars    map[string]interface{} `json:"extra_vars"`
			Format       string                 `json:"format"`
			InventoryID  int                    `json:"inventory_id"`
		}

		// Read the raw request body for debugging
		rawBody, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody)) // Reset the body

		fmt.Printf("Raw request body: %s\n", string(rawBody))

		if err := c.ShouldBindJSON(&requestData); err != nil {
			fmt.Printf("Error parsing request body: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
			return
		}

		// Double-check that the host still exists in the database (it might have been deleted)
		var checkHost Host
		if err := db.Where("name = ?", hostname).First(&checkHost).Error; err != nil {
			fmt.Printf("Host no longer exists in database: %s\n", hostname)
			c.JSON(http.StatusNotFound, gin.H{"error": "Host not found or has been deleted"})
			return
		}

		// Log received data for debugging
		fmt.Printf("Parsed request: template_id=%d, template_name=%s, format=%s, inventory_id=%d\n",
			requestData.TemplateID, requestData.TemplateName, requestData.Format, requestData.InventoryID)

		// Use template ID directly if provided
		if requestData.TemplateID <= 0 && requestData.TemplateName != "" {
			// Check if we have a template ID for this name in the configuration
			if ServerConfig.Awx.TemplateIDs != nil {
				if id, exists := ServerConfig.Awx.TemplateIDs[requestData.TemplateName]; exists {
					log.Debug().
						Str("component", "api").
						Str("operation", "find_template_id").
						Str("template_name", requestData.TemplateName).
						Int("template_id", id).
						Msg("Found template ID for name")
					requestData.TemplateID = id
				} else {
					// Check for common aliases
					templateName := strings.ToLower(strings.TrimSpace(requestData.TemplateName))

					// Try some common alternative names
					if templateName == "client" || templateName == "monokit-client" {
						if id, exists := ServerConfig.Awx.TemplateIDs["manual-install-monokit-client"]; exists {
							log.Debug().
								Str("component", "api").
								Str("operation", "find_template_id").
								Str("template_name", "manual-install-monokit-client").
								Int("template_id", id).
								Msg("Found template ID for alias")
							requestData.TemplateID = id
						}
					}
				}
			}

			// If we still don't have a template ID, report an error
			if requestData.TemplateID <= 0 {
				log.Debug().
					Str("component", "api").
					Str("operation", "no_template_id").
					Str("template_name", requestData.TemplateName).
					Msg("No template ID found for name")

				// Provide a helpful message listing available templates
				var availableTemplates []string
				for name := range ServerConfig.Awx.TemplateIDs {
					availableTemplates = append(availableTemplates, name)
				}

				c.JSON(http.StatusBadRequest, gin.H{
					"error":               fmt.Sprintf("No template ID configured for '%s'", requestData.TemplateName),
					"detail":              "Please configure the template ID in server.yml or provide a valid template_id directly",
					"available_templates": availableTemplates,
				})
				return
			}
		}

		// If no specific template was provided, use "manual-install-monokit-client" by default
		if requestData.TemplateID <= 0 && requestData.TemplateName == "" {
			if id, exists := ServerConfig.Awx.TemplateIDs["manual-install-monokit-client"]; exists {
				log.Debug().
					Str("component", "api").
					Str("operation", "use_default_template").
					Int("template_id", id).
					Msg("Using default 'manual-install-monokit-client' template ID")
				requestData.TemplateID = id
			} else {
				log.Debug().
					Str("component", "api").
					Str("operation", "no_default_template").
					Msg("No template ID configured for 'manual-install-monokit-client'")

				// Provide a helpful message listing available templates
				var availableTemplates []string
				for name := range ServerConfig.Awx.TemplateIDs {
					availableTemplates = append(availableTemplates, name)
				}

				c.JSON(http.StatusBadRequest, gin.H{
					"error":               "No default template ID configured",
					"detail":              "Please configure a template ID for 'manual-install-monokit-client' in server.yml",
					"available_templates": availableTemplates,
				})
				return
			}
		}

		// Validate we have a template ID
		if requestData.TemplateID <= 0 {
			log.Debug().
				Str("component", "api").
				Str("operation", "no_template_id").
				Msg("No template ID or name provided")
			c.JSON(http.StatusBadRequest, gin.H{"error": "No template_id or template_name provided"})
			return
		}

		log.Debug().
			Str("component", "api").
			Str("operation", "execute_template").
			Int("template_id", requestData.TemplateID).
			Str("host", hostname).
			Msg("Will execute template_id for host")

		// Check host IP address for local network
		if host.IpAddress == "" {
			fmt.Printf("Warning: Host %s has no IP address\n", hostname)
			return
		} else {
			fmt.Printf("Host %s has IP address: %s\n", hostname, host.IpAddress)
		}

		// Get AWX host ID for this host
		var awxHostId string
		if host.AwxHostId != "" {
			awxHostId = host.AwxHostId
			fmt.Printf("Using AWX host ID from database: %s\n", awxHostId)
		} else {
			fmt.Printf("Warning: Host %s has no AWX host ID in database\n", hostname)
		}

		// Prepare payload for AWX API
		payload := map[string]interface{}{}

		// Determine the appropriate limit parameter for the job
		if awxHostId != "" {
			// If we have AWX host ID, use it in a hosts list
			fmt.Printf("Adding AWX host ID to limit: %s\n", awxHostId)
			payload["limit"] = hostname
		} else {
			// Otherwise use hostname as limit
			fmt.Printf("Using hostname as limit: %s\n", hostname)
			payload["limit"] = hostname
		}

		// Log inventory configuration
		fmt.Printf("Inventory configuration: request_inventory_id=%d, default_inventory_id=%d\n",
			requestData.InventoryID, ServerConfig.Awx.DefaultInventoryID)

		// Check if the host is in AWX already
		if host.AwxHostId == "" {
			fmt.Printf("Host %s not registered in AWX, will create it first\n", hostname)

			// Try to find or create AWX host
			awxHostId, err := EnsureHostInAwx(db, host)
			if err != nil {
				fmt.Printf("Error ensuring host in AWX: %v\n", err)
				// Continue anyway as the job might still work with just hostname
			} else if awxHostId != "" {
				fmt.Printf("Successfully registered host in AWX with ID: %s\n", awxHostId)
				// Update the host record
				host.AwxHostId = awxHostId
				if err := db.Save(&host).Error; err != nil {
					fmt.Printf("Warning: Failed to update host with AWX ID: %v\n", err)
				}
			}
		}

		// Inventory ID is required by AWX API
		if requestData.InventoryID > 0 {
			// Use inventory ID from request if provided
			payload["inventory"] = requestData.InventoryID
			fmt.Printf("Using inventory ID from request: %d\n", requestData.InventoryID)
		} else if ServerConfig.Awx.DefaultInventoryID > 0 {
			// Use default inventory ID from config if available
			payload["inventory"] = ServerConfig.Awx.DefaultInventoryID
			fmt.Printf("Using default inventory ID from config: %d\n", ServerConfig.Awx.DefaultInventoryID)
		} else {
			// No inventory ID available - this will cause an error from AWX
			fmt.Printf("No inventory ID provided and no default configured\n")
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "No inventory ID provided and no default configured",
				"details": "The AWX API requires an inventory ID for job execution. Please provide an inventory_id in the request or set default_inventory_id in server config.",
			})
			return
		}

		// Add extra_vars if provided
		if requestData.ExtraVars != nil && len(requestData.ExtraVars) > 0 {
			fmt.Printf("Processing extra_vars: %+v\n", requestData.ExtraVars)
			var extraVarsStr string

			// Check if format is YAML, otherwise use JSON (default)
			format := strings.ToLower(requestData.Format)
			if format == "yaml" || format == "yml" {
				// Convert to YAML string
				extraVarsYAML, err := yaml.Marshal(requestData.ExtraVars)
				if err != nil {
					fmt.Printf("Error encoding extra_vars to YAML: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode extra_vars to YAML: " + err.Error()})
					return
				}
				extraVarsStr = string(extraVarsYAML)
				fmt.Printf("Converted to YAML: %s\n", extraVarsStr)
			} else {
				// Convert to JSON string (default for backward compatibility)
				extraVarsJSON, err := json.Marshal(requestData.ExtraVars)
				if err != nil {
					fmt.Printf("Error encoding extra_vars to JSON: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode extra_vars to JSON: " + err.Error()})
					return
				}
				extraVarsStr = string(extraVarsJSON)
				fmt.Printf("Converted to JSON: %s\n", extraVarsStr)
			}

			payload["extra_vars"] = extraVarsStr
			fmt.Printf("Added extra_vars to payload\n")
		} else {
			fmt.Printf("No extra_vars provided\n")
			// Make sure we at least specify an empty object
			payload["extra_vars"] = "{}"
		}

		// Marshal payload to JSON
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			fmt.Printf("Error encoding request payload: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode request payload: " + err.Error()})
			return
		}
		fmt.Printf("Final payload: %s\n", string(payloadBytes))

		// Create a new HTTP client with timeout and TLS settings
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !ServerConfig.Awx.VerifySSL,
			},
			ResponseHeaderTimeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}
		client := &http.Client{
			Transport: transport,
			Timeout:   time.Duration(ServerConfig.Awx.Timeout) * time.Second,
		}

		// AWX API endpoint for launching a job template - ensure URL is properly formatted
		awxURL := strings.TrimRight(ServerConfig.Awx.Url, "/")
		apiURL := fmt.Sprintf("%s/api/v2/job_templates/%d/launch/", awxURL, requestData.TemplateID)
		log.Debug().
			Str("component", "api").
			Str("operation", "call_awx_api").
			Str("api_url", apiURL).
			Msg("Calling AWX API")

		// Create the request with complete debugging
		log.Debug().
			Str("component", "api").
			Str("operation", "prepare_request").
			Str("api_url", apiURL).
			Str("request_payload", string(payloadBytes)).
			Msg("Preparing to execute AWX job template")

		// Simple log message about which template we're using
		log.Debug().
			Str("component", "api").
			Str("operation", "use_template").
			Int("template_id", requestData.TemplateID).
			Str("host", hostname).
			Msg("Using template ID from configuration")

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			fmt.Printf("Error creating request: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
			return
		}

		// Set basic auth and headers
		req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
		req.Header.Set("Content-Type", "application/json")

		// Add detailed debugging
		fmt.Printf("Request headers: %+v\n", req.Header)

		// Execute the request with recovery
		func() {
			// Use defer/recover to catch any panics during the HTTP request
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC in AWX job execution: %v\n", r)
					stack := make([]byte, 4096)
					stack = stack[:runtime.Stack(stack, false)]
					fmt.Printf("Stack trace: %s\n", string(stack))
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Internal server error: %v", r)})
				}
			}()

			// Execute the request
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("Error executing request: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute request: " + err.Error()})
				return
			}
			defer resp.Body.Close()

			fmt.Printf("AWX API response status: %d\n", resp.StatusCode)

			// Read the entire response body for logging and analysis
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("Error reading response body: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response: " + err.Error()})
				return
			}

			// Log the raw response for debugging
			fmt.Printf("AWX API response body: %s\n", string(respBody))

			// Check response status
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				errorMsg := fmt.Sprintf("AWX API returned status: %d - %s", resp.StatusCode, string(respBody))

				// Check for specific error messages and provide more user-friendly errors
				if strings.Contains(string(respBody), "Inventory matching query does not exist") {
					// This is likely due to missing or invalid inventory ID
					if ServerConfig.Awx.DefaultInventoryID <= 0 {
						errorMsg = "AWX Error: No inventory specified. Please configure default_inventory_id in server config or provide an inventory_id in the request."
					} else {
						errorMsg = fmt.Sprintf("AWX Error: Inventory with ID %d does not exist. Please update the inventory_id in your request or the default_inventory_id in server config.", ServerConfig.Awx.DefaultInventoryID)
					}
				} else if strings.Contains(string(respBody), "Template with this id not found") {
					errorMsg = fmt.Sprintf("AWX Error: Template with ID %d not found. Please check the template_id in your request.", requestData.TemplateID)
				} else if strings.Contains(string(respBody), "Job Template with this Name already exists") {
					errorMsg = "AWX Error: Job Template with this name already exists."
				} else if strings.Contains(string(respBody), "Unable to add job to queue") {
					errorMsg = "AWX Error: Unable to add job to queue. The system may be at capacity."
				}

				fmt.Printf("Error from AWX: %s\n", errorMsg)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
				return
			}

			// Create a new reader for the response body for parsing
			responseBodyReader := bytes.NewReader(respBody)

			// Parse response
			var responseData map[string]interface{}
			if err := json.NewDecoder(responseBodyReader).Decode(&responseData); err != nil {
				fmt.Printf("Error decoding response: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response: " + err.Error()})
				return
			}

			fmt.Printf("Job launched successfully: ID=%v, Status=%v\n", responseData["id"], responseData["status"])

			// Return job information
			c.JSON(http.StatusOK, gin.H{
				"job_id":      responseData["id"],
				"status":      responseData["status"],
				"message":     "Job launched successfully",
				"host_name":   hostname,
				"job_details": responseData,
			})
		}()
	}
}

// ensureHostInAwx makes sure the host exists in AWX, creating it if necessary
// Returns the AWX host ID if successful
func EnsureHostInAwx(db *gorm.DB, host Host) (string, error) {
	if !ServerConfig.Awx.Enabled {
		return "", fmt.Errorf("AWX integration is not enabled")
	}

	fmt.Printf("Ensuring host exists in AWX: %s (IP: %s)\n", host.Name, host.IpAddress)

	if host.IpAddress == "" {
		return "", fmt.Errorf("host has no IP address")
	}

	// Create a new HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(ServerConfig.Awx.Timeout) * time.Second,
	}

	// Check if inventory ID is available in config
	if ServerConfig.Awx.DefaultInventoryID <= 0 {
		return "", fmt.Errorf("no default inventory ID configured")
	}

	// First, check if host already exists in AWX by name
	awxURL := ServerConfig.Awx.Url
	searchURL := fmt.Sprintf("%s/api/v2/hosts/?name=%s", awxURL, host.Name)

	// Create the request to search for existing host
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create search request: %v", err)
	}

	// Set basic auth and headers
	req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute search request: %v", err)
	}

	// Parse response to check if host exists
	var searchResponse struct {
		Count   int `json:"count"`
		Results []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"results"`
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if err := json.Unmarshal(respBody, &searchResponse); err != nil {
		return "", fmt.Errorf("failed to parse search response: %v", err)
	}

	// If host exists, return its ID
	if searchResponse.Count > 0 {
		for _, result := range searchResponse.Results {
			if result.Name == host.Name {
				awxHostId := fmt.Sprintf("%d", result.ID)
				fmt.Printf("Host already exists in AWX with ID: %s\n", awxHostId)
				return awxHostId, nil
			}
		}
	}

	fmt.Printf("Host not found in AWX, creating new host: %s\n", host.Name)

	// Prepare variables for AWX API
	variables := map[string]interface{}{
		"ansible_host": host.IpAddress,
	}

	// Convert variables to YAML string
	variablesYaml, err := yaml.Marshal(variables)
	if err != nil {
		return "", fmt.Errorf("failed to format variables: %v", err)
	}

	// Prepare payload for AWX API
	payload := map[string]interface{}{
		"name":        host.Name,
		"variables":   string(variablesYaml),
		"enabled":     true,
		"instance_id": "",
		"inventory":   ServerConfig.Awx.DefaultInventoryID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Create the request to create new host
	apiURL := fmt.Sprintf("%s/api/v2/hosts/", awxURL)
	req, err = http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set basic auth and headers
	req.SetBasicAuth(ServerConfig.Awx.Username, ServerConfig.Awx.Password)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// Read error response for debugging
		errorBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AWX API returned status: %d - %s", resp.StatusCode, string(errorBody))
	}

	// Parse response
	var awxHostResponse map[string]interface{}
	respBody, _ = io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBody, &awxHostResponse); err != nil {
		return "", fmt.Errorf("failed to decode response: %s", err.Error())
	}

	// Extract AWX host ID
	awxHostIDValue, ok := awxHostResponse["id"]
	if !ok {
		return "", fmt.Errorf("no ID in AWX response")
	}

	// Convert to string (could be float64 or int)
	var awxHostId string
	switch v := awxHostIDValue.(type) {
	case float64:
		awxHostId = fmt.Sprintf("%d", int(v))
	case int:
		awxHostId = fmt.Sprintf("%d", v)
	case string:
		awxHostId = v
	default:
		awxHostId = fmt.Sprintf("%v", v)
	}

	fmt.Printf("Successfully created host in AWX with ID: %s\n", awxHostId)
	return awxHostId, nil
}

// filterLogsForHost parses AWX job logs and filters content to focus on a specific host
// while keeping task headers and other important structural elements
func filterLogsForHost(rawLogs string, hostname string) string {
	lines := strings.Split(rawLogs, "\n")
	filteredLines := []string{}
	keepNextLines := false
	inTaskBlock := false
	inPlayRecap := false
	addedRecapHeader := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Exact match for the "PLAY RECAP" line
		if strings.HasPrefix(line, "PLAY RECAP") {
			inPlayRecap = true
			addedRecapHeader = false
			// We'll add this line later when we find a matching host
			continue
		}

		// Special handling for PLAY RECAP section
		if inPlayRecap {
			// If we find the exact hostname followed by a colon (summary line)
			if strings.HasPrefix(line, hostname) && strings.Contains(line, ":") {
				// If we haven't added the PLAY RECAP header yet, add it first
				if !addedRecapHeader {
					filteredLines = append(filteredLines, "PLAY RECAP *********************************************************************")
					addedRecapHeader = true
				}
				// Add the summary line for this host
				filteredLines = append(filteredLines, line)
			}

			// If we hit a new section (line starting with uppercase letters followed by a space)
			if regexp.MustCompile(`^[A-Z]+\s`).MatchString(line) && !strings.HasPrefix(line, "PLAY RECAP") {
				inPlayRecap = false
				// Process this line normally (don't continue to next iteration)
			} else {
				// Stay in the PLAY RECAP section and skip to next line
				continue
			}
		}

		// General patterns
		playPattern := regexp.MustCompile(`PLAY\s+\[.*?\]`)
		taskPattern := regexp.MustCompile(`TASK\s+\[.*?\]`)
		hostPattern := regexp.MustCompile(`\[` + regexp.QuoteMeta(hostname) + `\]|\[` + regexp.QuoteMeta(hostname) + `\s+->.*?\]`)
		includePattern := regexp.MustCompile(`INCLUDED TASKS|RUNNING HANDLER`)
		skippingPattern := regexp.MustCompile(`skipping:\s+\[` + regexp.QuoteMeta(hostname) + `\]`)
		statsPattern := regexp.MustCompile(`STATS|failed=|ok=|changed=|unreachable=`)

		// Always include play headers, task headers, and statistical information
		if playPattern.MatchString(line) ||
			taskPattern.MatchString(line) ||
			includePattern.MatchString(line) ||
			statsPattern.MatchString(line) {
			filteredLines = append(filteredLines, line)
			inTaskBlock = true
			keepNextLines = false
			continue
		}

		// If the line contains the host name, include it and remember to keep next related lines
		if hostPattern.MatchString(line) {
			filteredLines = append(filteredLines, line)
			keepNextLines = true
			continue
		}

		// If we're keeping lines due to a previous host match
		if keepNextLines {
			// Keep the line only if it seems to be related (indented or has specific content)
			if strings.HasPrefix(line, " ") || line == "" || skippingPattern.MatchString(line) {
				filteredLines = append(filteredLines, line)
				continue
			} else {
				// Stop keeping lines if we hit another content type
				keepNextLines = false
			}
		}

		// If we're in a task block but not keeping specific lines,
		// add empty spacing where appropriate to maintain structure
		if inTaskBlock && !keepNextLines && len(filteredLines) > 0 &&
			filteredLines[len(filteredLines)-1] != "" && line == "" {
			filteredLines = append(filteredLines, "")
			inTaskBlock = false
		}
	}

	return strings.Join(filteredLines, "\n")
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
func EnableComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		service := c.Param("service")

		db.Find(&models.HostsList)

		idx := slices.IndexFunc(models.HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		host := models.HostsList[idx]
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
		db.Find(&models.HostsList)

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
func DisableComponent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		service := c.Param("service")

		db.Find(&models.HostsList)

		idx := slices.IndexFunc(models.HostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		host := models.HostsList[idx]

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
		db.Find(&models.HostsList)

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
func GetComponentStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			if h, ok := c.Get("hostname"); ok {
				name = h.(string)
			}
		}
		service := c.Param("service")
		if service == "" {
			service = c.Param("component")
		}
		if name == "" || service == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing host name or service name"})
			return
		}

		for _, host := range models.HostsList {
			if host.Name == name {
				components := strings.Split(host.DisabledComponents, "::")
				isDisabled := slices.Contains(components, service)

				// Return consistent responses with both formats to support different clients
				c.JSON(http.StatusOK, gin.H{
					"name":     name,
					"service":  service,
					"disabled": isDisabled,
					"status":   map[bool]string{true: "disabled", false: "enabled"}[isDisabled],
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
func SubmitHostLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get hostname from context
		hostname, exists := c.Get("hostname")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Hostname not found in context"})
			return
		}

		// Parse log data from request
		var logRequest APILogRequest
		if err := c.ShouldBindJSON(&logRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate log level
		validLevels := []string{"info", "warning", "error", "critical"}
		isValidLevel := false
		for _, level := range validLevels {
			if logRequest.Level == level {
				isValidLevel = true
				break
			}
		}
		if !isValidLevel {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log level. Must be one of: info, warning, error, critical"})
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

			// Use a batch approach instead of a single large delete
			batchSize := 500
			var deleted int
			for deleted < int(toDelete) {
				// Calculate current batch size
				currentBatch := batchSize
				if deleted+batchSize > int(toDelete) {
					currentBatch = int(toDelete) - deleted
				}

				// Get IDs of logs to delete in this batch
				var logIds []uint
				if err := db.Model(&HostLog{}).Where("deleted_at IS NULL").Order("timestamp ASC").Limit(currentBatch).Pluck("id", &logIds).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify old logs"})
					return
				}

				// Skip if no logs found
				if len(logIds) == 0 {
					break
				}

				// Delete logs by their IDs directly (avoids subquery)
				if err := db.Where("id IN ?", logIds).Delete(&HostLog{}).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete old logs"})
					return
				}

				deleted += currentBatch
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
func GetAllLogs(db *gorm.DB) gin.HandlerFunc {
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

		// Count total logs - explicitly specify not deleted for consistency and to use index
		var total int64
		db.Model(&HostLog{}).Where("deleted_at IS NULL").Count(&total)

		// Get logs with pagination - specify deleted_at IS NULL to use the composite index
		var logs []HostLog
		if err := db.Where("deleted_at IS NULL").Order("timestamp desc").Offset(offset).Limit(pageSizeInt).Find(&logs).Error; err != nil {
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
func GetHostLogs(db *gorm.DB) gin.HandlerFunc {
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
		db.Model(&HostLog{}).Where("host_name = ? AND deleted_at IS NULL", hostname).Count(&total)

		// Get logs with pagination - specify deleted_at IS NULL to use the composite index
		var logs []HostLog
		if err := db.Where("host_name = ? AND deleted_at IS NULL", hostname).Order("timestamp desc").Offset(offset).Limit(pageSizeInt).Find(&logs).Error; err != nil {
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
func SearchLogs(db *gorm.DB) gin.HandlerFunc {
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

		// Build query with filters - always include deleted_at IS NULL to use the index
		query := db.Model(&HostLog{}).Where("deleted_at IS NULL")

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

// @Summary Get hourly log statistics
// @Description Get log statistics for the last hour broken down by 5-minute intervals
// @Tags Logs
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {array} map[string]int "Array of 12 objects containing log counts for each 5-minute interval"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /logs/hourly [get]
func GetHourlyLogStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Calculate the time range - last hour
		now := time.Now()
		oneHourAgo := now.Add(-time.Hour)

		// Initialize the result array with 12 intervals (5 minutes each)
		result := make([]map[string]int, 12)
		for i := 0; i < 12; i++ {
			result[i] = map[string]int{
				"critical": 0,
				"error":    0,
				"warning":  0,
				"info":     0,
			}
		}

		// Query logs from the last hour
		var logs []HostLog
		if err := db.Where("timestamp >= ? AND timestamp <= ? AND deleted_at IS NULL", oneHourAgo, now).Find(&logs).Error; err != nil {
			log.Error().
				Str("component", "logs").
				Str("operation", "get_hourly_stats").
				Err(err).
				Msg("Error fetching hourly log statistics")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve hourly log statistics"})
			return
		}

		// Process each log and assign it to the correct 5-minute interval
		for _, log := range logs {
			// Calculate which 5-minute interval this log belongs to
			timeDiff := log.Timestamp.Sub(oneHourAgo)
			intervalIndex := int(timeDiff.Minutes()) / 5

			// Ensure the interval index is within bounds (0-11)
			if intervalIndex >= 0 && intervalIndex < 12 {
				level := strings.ToLower(log.Level)
				if _, exists := result[intervalIndex][level]; exists {
					result[intervalIndex][level]++
				}
			}
		}

		c.JSON(http.StatusOK, result)
	}
}

// getHealthTools returns a list of available health tools
// @Summary Get available health tools
// @Description Retrieves a combined list of names of all registered health check providers and tools found in submitted health data.
// @Tags Health
// @Produce json
// @Success 200 {array} string "List of health tool names"
// @Failure 401 {object} ErrorResponse "Unauthorized - authentication required"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /health/tools [get]
// @Security ApiKeyAuth
func GetHealthTools(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for authentication
		_, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		// Get tools from the health registry
		registeredTools := health.List()
		toolSet := make(map[string]bool)
		for _, tool := range registeredTools {
			toolSet[tool] = true
		}

		// Get tools from the database
		var dbTools []string
		if err := db.Model(&HostHealthData{}).Distinct("tool_name").Pluck("tool_name", &dbTools).Error; err != nil {
			log.Error().
				Str("component", "health").
				Str("operation", "get_tools").
				Err(err).
				Msg("Error fetching distinct tool names from database")
			// We can still return registered tools even if DB query fails
		} else {
			for _, tool := range dbTools {
				toolSet[tool] = true
			}
		}

		// Convert set to list
		finalToolList := make([]string, 0, len(toolSet))
		for tool := range toolSet {
			finalToolList = append(finalToolList, tool)
		}
		sort.Strings(finalToolList) // Optional: sort for consistent ordering

		c.JSON(http.StatusOK, finalToolList)
	}
}

// getHostHealth returns aggregated health data for all tools for a specific host
// @Summary Get aggregated health data for a host
// @Description Retrieves health data from the database for a given hostname.
// @Description For the Monokit server itself, if data is not found in the database, it may attempt to collect it directly from local providers.
// @Tags Health
// @Produce json
// @Param name path string true "Hostname"
// @Success 200 {object} map[string]interface{} "Aggregated health data"
// @Failure 404 {object} ErrorResponse "Host not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /hosts/{name}/health [get]
// @Security ApiKeyAuth
func GetHostHealth(db *gorm.DB, monokitHostname string) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostName := c.Param("name")
		var host Host
		if err := db.Where("name = ?", hostName).First(&host).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve host: " + err.Error()})
			return
		}

		results := make(map[string]interface{})
		var healthDataEntries []HostHealthData
		if err := db.Where("host_name = ?", hostName).Find(&healthDataEntries).Error; err != nil {
			log.Error().
				Str("component", "health").
				Str("operation", "get_host_health").
				Str("host", hostName).
				Err(err).
				Msg("Error fetching health data for host from DB")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve health data from database"})
			return
		}

		for _, entry := range healthDataEntries {
			var dataContent interface{}
			if err := json.Unmarshal([]byte(entry.DataJSON), &dataContent); err != nil {
				results[entry.ToolName] = gin.H{"error": fmt.Sprintf("Error unmarshalling health data for tool %s: %v", entry.ToolName, err)}
				log.Error().
					Str("component", "health").
					Str("operation", "unmarshal_health_data").
					Str("host", hostName).
					Str("tool", entry.ToolName).
					Err(err).
					Msg("Error unmarshalling health data")
			} else {
				results[entry.ToolName] = dataContent
			}
		}

		// Fallback for Monokit server's own health if it's the requested host
		if hostName == monokitHostname {
			allProviders := health.GetAllProviders()
			var wg sync.WaitGroup
			var mu sync.Mutex

			for toolName, provider := range allProviders {
				if _, exists := results[toolName]; !exists { // Only collect if not found in DB
					wg.Add(1)
					go func(name string, p health.Provider) {
						defer wg.Done()
						data, err := p.Collect(hostName)
						mu.Lock()
						defer mu.Unlock()
						if err != nil {
							results[name] = gin.H{"error": fmt.Sprintf("Error collecting local %s data: %v", name, err)}
							log.Error().
								Str("component", "health").
								Str("operation", "collect_local_health").
								Str("host", hostName).
								Str("tool", name).
								Err(err).
								Msg("Error collecting local health data")
						} else {
							results[name] = data
						}
					}(toolName, provider)
				}
			}
			wg.Wait()
		}

		if len(results) == 0 && len(healthDataEntries) == 0 {
			// If still no results, it could mean no data has been pushed or no local providers for self-health
			// c.JSON(http.StatusNotFound, gin.H{"message": "No health data available for this host"})
			// Return empty map instead of 404 to allow frontend to display "no data" state
			c.JSON(http.StatusOK, results)
			return
		}

		c.JSON(http.StatusOK, results)
	}
}

// postHostHealth receives and stores health data submitted by a host agent.
// @Summary Submit health data from a host agent
// @Description Allows a host agent to POST its health data for a specific tool.
// @Tags Health
// @Accept json
// @Produce json
// @Param tool path string true "Name of the health tool (e.g., osHealth)"
// @Param healthData body string true "JSON health data from the tool"
// @Success 200 {object} interface{} "Data received and stored"
// @Failure 400 {object} ErrorResponse "Invalid JSON data or missing tool name"
// @Failure 401 {object} ErrorResponse "Unauthorized (invalid host token)"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /host/health/{tool} [post]
// @Security ApiKeyAuth
func PostHostHealth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostName, exists := c.Get("hostname")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Hostname not found in context, host auth middleware might not have run"})
			return
		}

		toolName := c.Param("tool")
		if toolName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tool name parameter is required"})
			return
		}

		// Read the raw body
		jsonData, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		defer c.Request.Body.Close()

		// Validate if it's valid JSON - Unmarshal into a generic interface
		var dataInterface interface{}
		if err := json.Unmarshal(jsonData, &dataInterface); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data provided"})
			return
		}

		// Ensure the Host record exists, or handle as an error
		// This prevents orphaned health data if a host is deleted and an agent tries to post.
		var host Host
		if err := db.Where("name = ?", hostName.(string)).First(&host).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Host not found in database. Cannot submit health data."})
				return
			}
			log.Error().
				Str("component", "health").
				Str("operation", "post_host_health").
				Str("host", hostName.(string)).
				Err(err).
				Msg("Error fetching host for health data submission")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify host existence"})
			return
		}

		healthData := HostHealthData{
			HostName: hostName.(string),
			ToolName: toolName,
			DataJSON: string(jsonData), // Store as raw JSON string
		}

		// Upsert logic: Try to update if exists, otherwise create.
		// GORM's Save handles this if Primary Key is zero or if record is found.
		// For composite unique keys, we need to be more explicit or use raw SQL for true UPSERT.
		// Let's use a "find then update or create" approach for simplicity with GORM.

		var existingData HostHealthData
		err = db.Where("host_name = ? AND tool_name = ?", hostName.(string), toolName).First(&existingData).Error

		if err == nil { // Record found, update it
			existingData.DataJSON = string(jsonData)
			existingData.LastUpdated = time.Now() // GORM autoUpdateTime might handle this if configured
			if err := db.Save(&existingData).Error; err != nil {
				log.Error().
					Str("component", "health").
					Str("operation", "update_health_data").
					Str("host", hostName.(string)).
					Str("tool", toolName).
					Err(err).
					Msg("Error updating host health data")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update health data"})
				return
			}
		} else if err == gorm.ErrRecordNotFound { // Record not found, create it
			if err := db.Create(&healthData).Error; err != nil {
				log.Error().
					Str("component", "health").
					Str("operation", "create_health_data").
					Str("host", hostName.(string)).
					Str("tool", toolName).
					Err(err).
					Msg("Error creating host health data")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create health data"})
				return
			}
		} else { // Other DB error
			log.Error().
				Str("component", "health").
				Str("operation", "check_health_data").
				Str("host", hostName.(string)).
				Str("tool", toolName).
				Err(err).
				Msg("Error checking for existing host health data")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error while checking health data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Health data received and stored"})
	}
}

// getHostToolHealth returns health data for a specific tool for a specific host
// @Summary Get specific tool health data for a host
// @Description Retrieves health data from a specific registered provider for a given hostname and tool name.
// @Tags Health
// @Produce json
// @Param name path string true "Hostname"
// @Param tool path string true "Health tool name"
// @Success 200 {object} interface{} "Health data for the specified tool"
// @Failure 404 {object} ErrorResponse "Host not found or Tool not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /hosts/{name}/health/{tool} [get]
// @Security ApiKeyAuth
func GetHostToolHealth(db *gorm.DB, monokitHostname string) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostName := c.Param("name")
		toolName := c.Param("tool")

		var host Host
		if err := db.Where("name = ?", hostName).First(&host).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve host: " + err.Error()})
			return
		}

		var healthDataEntry HostHealthData
		err := db.Where("host_name = ? AND tool_name = ?", hostName, toolName).First(&healthDataEntry).Error

		if err == nil { // Data found in DB
			var dataContent interface{}
			if unmarshalErr := json.Unmarshal([]byte(healthDataEntry.DataJSON), &dataContent); unmarshalErr != nil {
				log.Error().
					Str("component", "health").
					Str("operation", "unmarshal_tool_health").
					Str("host", hostName).
					Str("tool", toolName).
					Err(unmarshalErr).
					Msg("Error unmarshalling health data")
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error unmarshalling health data for tool %s", toolName)})
				return
			}
			c.JSON(http.StatusOK, dataContent)
			return
		}

		if err != gorm.ErrRecordNotFound { // Some other DB error
			log.Error().
				Str("component", "health").
				Str("operation", "get_tool_health").
				Str("host", hostName).
				Str("tool", toolName).
				Err(err).
				Msg("Error fetching health data from DB")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve health data from database"})
			return
		}

		// Data not found in DB, try fallback for Monokit server's own health
		if hostName == monokitHostname {
			provider := health.Get(toolName)
			if provider == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Health tool '%s' not found locally for self-check and not in DB", toolName)})
				return
			}

			data, collectErr := provider.Collect(hostName)
			if collectErr != nil {
				log.Error().
					Str("component", "health").
					Str("operation", "collect_local_tool_health").
					Str("host", hostName).
					Str("tool", toolName).
					Err(collectErr).
					Msg("Error collecting local health data")
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error collecting local %s data: %v", toolName, collectErr)})
				return
			}
			c.JSON(http.StatusOK, data)
			return
		}

		// If not Monokit server and data not in DB
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Health data not found for tool '%s' on host '%s'", toolName, hostName)})
	}
}
