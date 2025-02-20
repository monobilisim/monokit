// Package common Monokit API.
// @title           Monokit API
// @version         1.0
// @description     API Server for Monokit monitoring and management system
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    https://mono.tr

// @license.name  GPLv3
// @license.url   https://www.gnu.org/licenses/gpl-3.0.en.html

// @host      localhost:9989
// @BasePath  /api/v1

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

package common

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common"
	_ "github.com/monobilisim/monokit/docs" // This will be generated
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Server struct {
	Port     string
	Postgres struct {
		Host     string
		Port     string
		User     string
		Password string
		Dbname   string
	}
}

type Host struct {
	gorm.Model
	Name                string    `json:"name"`
	CpuCores            int       `json:"cpuCores"`
	Ram                 string    `json:"ram"`
	MonokitVersion      string    `json:"monokitVersion"`
	Os                  string    `json:"os"`
	DisabledComponents  string    `json:"disabledComponents"`
	InstalledComponents string    `json:"installedComponents"`
	IpAddress           string    `json:"ipAddress"`
	Status              string    `json:"status"`
	UpdatedAt           time.Time `json:"UpdatedAt"`
	CreatedAt           time.Time `json:"CreatedAt"`
	WantsUpdateTo       string    `json:"wantsUpdateTo"`
	Groups              string    `json:"groups"`
	UpForDeletion       bool      `json:"upForDeletion" gorm:"default:false"`
	Inventory           string    `json:"inventory"`
}

type Inventory struct {
	gorm.Model
	Name string `json:"name" gorm:"unique"`
}

var ServerConfig Server
var hostsList []Host

// @Summary Register host
// @Description Register a new host or update existing host information
// @Tags hosts
// @Accept json
// @Produce json
// @Param host body Host true "Host information"
// @Success 200 {object} Host
// @Router /hosts [post]
func registerHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var host Host
		if err := c.ShouldBindJSON(&host); err != nil {
			fmt.Printf("Error binding JSON: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Set default inventory if not specified
		if host.Inventory == "" {
			host.Inventory = "default"
		}

		var existingHost Host
		result := db.Where("name = ?", host.Name).First(&existingHost)
		if result.Error == nil {
			// Update existing host
			host.ID = existingHost.ID
			host.UpForDeletion = existingHost.UpForDeletion
			if err := db.Model(&existingHost).Updates(&host).Error; err != nil {
				fmt.Printf("Error updating host: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update host"})
				return
			}
		} else {
			// Create new host
			if err := db.Create(&host).Error; err != nil {
				fmt.Printf("Error creating host: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create host"})
				return
			}
		}

		// Update the hosts list
		db.Find(&hostsList)

		// Return just the updated/created host
		c.JSON(http.StatusOK, host)
	}
}

// @Summary Get all hosts
// @Description Get list of all monitored hosts (filtered by user's inventory access)
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {array} Host
// @Router /hosts [get]
func getAllHosts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Sync all hosts' groups before returning
		for i := range hostsList {
			syncHostGroups(db, &hostsList[i])
			db.Save(&hostsList[i])
		}

		// Get the user from context
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		currentUser := user.(User)
		var filteredHosts []Host

		// Admin can see all hosts
		if currentUser.Role == "admin" {
			filteredHosts = hostsList
		} else {
			// Regular users can only see hosts in their inventory
			for _, host := range hostsList {
				if host.Inventory == currentUser.Inventory {
					filteredHosts = append(filteredHosts, host)
				}
			}
		}

		// Check UpdatedAt and update status
		for i := range filteredHosts {
			isOffline := time.Since(filteredHosts[i].UpdatedAt).Minutes() > 5
			if filteredHosts[i].UpForDeletion {
				if isOffline {
					// Only admins can actually delete hosts
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

// @Summary Get host by name
// @Description Get specific host information
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Success 200 {object} Host
// @Router /hosts/{name} [get]
func getHostByName() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		idx := slices.IndexFunc(hostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		c.JSON(http.StatusOK, hostsList[idx])
	}
}

// @Summary Delete host
// @Description Delete a host from the system
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Host name"
// @Success 200 {array} Host
// @Router /hosts/{name} [delete]
func deleteHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		// Delete the host from the pgsql database
		db.Where("name = ?", name).Delete(&Host{})

		// Sync the hosts list
		db.Find(&hostsList)

		c.JSON(http.StatusOK, hostsList)
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

		db.Find(&hostsList)

		idx := slices.IndexFunc(hostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		hostsList[idx].WantsUpdateTo = version

		// Update the host in the pgsql database
		db.Model(&Host{}).Where("name = ?", name).Updates(&hostsList[idx])
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

		db.Find(&hostsList)

		idx := slices.IndexFunc(hostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		host := hostsList[idx]
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
		db.Find(&hostsList)

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

		db.Find(&hostsList)

		idx := slices.IndexFunc(hostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		host := hostsList[idx]

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
		db.Find(&hostsList)

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
		idx := slices.IndexFunc(hostsList, func(h Host) bool {
			return h.Name == name
		})

		if idx == -1 {
			c.JSON(http.StatusOK, gin.H{"status": "not found"})
			return
		}

		host := hostsList[idx]

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

// @Summary Get all inventories
// @Description Get list of all inventories with host counts (admin only)
// @Tags inventory
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {array} InventoryResponse
// @Failure 403 {object} ErrorResponse
// @Router /inventory [get]
func getAllInventories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		var inventories []Inventory
		if err := db.Find(&inventories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inventories"})
			return
		}

		response := make([]InventoryResponse, 0)

		// First add default inventory
		var defaultCount int64
		db.Model(&Host{}).Where("inventory = ? OR inventory = ''", "default").Count(&defaultCount)
		response = append(response, InventoryResponse{
			Name:  "default",
			Hosts: int(defaultCount),
		})

		// Then add other inventories
		for _, inv := range inventories {
			if inv.Name == "default" {
				continue // Skip default as we already added it
			}
			var count int64
			if err := db.Model(&Host{}).Where("inventory = ?", inv.Name).Count(&count).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count hosts"})
				return
			}

			response = append(response, InventoryResponse{
				Name:  inv.Name,
				Hosts: int(count),
			})
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Create new inventory
// @Description Create a new inventory (admin only)
// @Tags inventory
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param inventory body CreateInventoryRequest true "Inventory information"
// @Success 201 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /inventory [post]
func createInventory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		var req CreateInventoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Name == "default" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create inventory named 'default'"})
			return
		}

		// Check if inventory already exists
		var existingInventory Inventory
		if err := db.Where("name = ?", req.Name).First(&existingInventory).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Inventory already exists"})
			return
		}

		inventory := Inventory{Name: req.Name}
		if err := db.Create(&inventory).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create inventory"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("Inventory %s created successfully", req.Name)})
	}
}

// @Summary Delete inventory
// @Description Schedule deletion of an inventory and all its hosts (admin only)
// @Tags inventory
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param name path string true "Inventory name"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /inventory/{name} [delete]
func deleteInventory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		inventoryName := c.Param("name")
		if inventoryName == "default" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete default inventory"})
			return
		}

		// Find the inventory
		var inventory Inventory
		if err := db.Where("name = ?", inventoryName).First(&inventory).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Inventory not found"})
			return
		}

		// Mark all hosts in the inventory for deletion
		var hosts []Host
		if err := db.Where("inventory = ?", inventoryName).Find(&hosts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch hosts in inventory"})
			return
		}

		for _, host := range hosts {
			host.UpForDeletion = true
			if err := db.Save(&host).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to schedule hosts for deletion"})
				return
			}
		}

		// Delete the inventory
		if err := db.Delete(&inventory).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete inventory"})
			return
		}

		// Update the hosts list
		db.Find(&hostsList)

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Inventory %s and all its hosts (%d) have been scheduled for deletion", inventoryName, len(hosts)),
		})
	}
}

func ServerMain(cmd *cobra.Command, args []string) {
	version := "1.0.0"
	apiVersion := strings.Split(version, ".")[0]
	common.ScriptName = "server"
	common.TmpDir = common.TmpDir + "server"
	common.Init()
	viper.SetDefault("port", "9989")
	common.ConfInit("server", &ServerConfig)

	fmt.Println("Monokit API Server - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05") + " - API v" + apiVersion)

	// Connect to the database
	dsn := "host=" + ServerConfig.Postgres.Host + " user=" + ServerConfig.Postgres.User + " password=" + ServerConfig.Postgres.Password + " dbname=" + ServerConfig.Postgres.Dbname + " port=" + ServerConfig.Postgres.Port + " sslmode=disable TimeZone=Europe/Istanbul"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&Host{}, &Inventory{})

	// Create default inventory if it doesn't exist
	var defaultInventory Inventory
	if db.Where("name = ?", "default").First(&defaultInventory).Error == gorm.ErrRecordNotFound {
		db.Create(&Inventory{Name: "default"})
	}

	// Get the hosts list from the pgsql database
	db.Find(&hostsList)

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// Add Swagger documentation endpoint
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Setup authentication routes
	SetupAuthRoutes(r, db)

	// Use the handler functions
	r.POST("/api/v"+apiVersion+"/hosts", registerHost(db))

	api := r.Group("/api/v" + apiVersion)
	api.Use(AuthMiddleware(db))
	{
		api.GET("/hosts", getAllHosts(db))
		api.GET("/hosts/:name", getHostByName())
		api.DELETE("/hosts/:name", deleteHost(db))
		api.POST("/hosts/:name/updateTo/:version", updateHostVersion(db))
		api.POST("/hosts/:name/enable/:service", enableComponent(db))
		api.POST("/hosts/:name/disable/:service", disableComponent(db))
		api.GET("/hosts/:name/:service", getComponentStatus())
		api.GET("/inventory", getAllInventories(db))
		api.POST("/inventory", createInventory(db))
		api.DELETE("/inventory/:name", deleteInventory(db))
	}

	r.Run(":" + ServerConfig.Port)
}

// Add this function to sync host groups
func syncHostGroups(db *gorm.DB, host *Host) error {
	// Get all groups that contain this host
	var groups []Group
	if err := db.Preload("Hosts").Find(&groups).Error; err != nil {
		return err
	}

	// Build groups string
	var hostGroups []string
	for _, group := range groups {
		for _, h := range group.Hosts {
			if h.Name == host.Name {
				hostGroups = append(hostGroups, group.Name)
				break
			}
		}
	}

	// Update host's Groups field
	if len(hostGroups) > 0 {
		host.Groups = strings.Join(hostGroups, ",")
	} else {
		host.Groups = "nil"
	}

	return nil
}
