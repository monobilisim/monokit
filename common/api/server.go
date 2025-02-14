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
	UpForDeletion       bool      `json:"upForDeletion"`
}

var ServerConfig Server
var hostsList []Host

// @Summary Register host
// @Description Register a new host or update existing host information
// @Tags hosts
// @Accept json
// @Produce json
// @Param host body Host true "Host information"
// @Success 200 {array} Host
// @Router /hostsList [post]
func registerHost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var host Host
		if err := c.ShouldBindJSON(&host); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		update := false
		for i := 0; i < len(hostsList); i++ {
			if hostsList[i].Name == host.Name {
				update = true
				break
			}
		}

		if update {
			// Sync groups before updating
			if err := syncHostGroups(db, &host); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync host groups"})
				return
			}
			db.Model(&Host{}).Where("name = ?", host.Name).Updates(&host)
		} else {
			// For new hosts, start with nil groups if none specified
			if host.Groups == "" {
				host.Groups = "nil"
			}
			db.Create(&host)
		}

		db.Find(&hostsList)
		c.JSON(http.StatusOK, hostsList)
	}
}

// @Summary Get all hosts
// @Description Get list of all monitored hosts
// @Tags hosts
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {array} Host
// @Router /hostsList [get]
func getAllHosts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Sync all hosts' groups before returning
		for i := range hostsList {
			syncHostGroups(db, &hostsList[i])
			db.Save(&hostsList[i])
		}

		// Check UpdatedAt and delete offline hosts that are scheduled for deletion
		var hostsToKeep []Host
		for _, host := range hostsList {
			isOffline := time.Since(host.UpdatedAt).Minutes() > 5
			if isOffline {
				if host.UpForDeletion {
					// Delete the host from database
					db.Delete(&host)
					continue // Skip adding to hostsToKeep
				}
				host.Status = "Offline"
			}
			hostsToKeep = append(hostsToKeep, host)
		}
		hostsList = hostsToKeep

		// If user is not admin, use filtered hosts
		if filteredHosts, exists := c.Get("filteredHosts"); exists {
			c.JSON(http.StatusOK, filteredHosts)
			return
		}

		c.JSON(http.StatusOK, hostsList)
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
// @Router /hostsList/{name} [get]
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
// @Router /hostsList/{name} [delete]
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
// @Router /hostsList/{name}/updateTo/{version} [post]
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
// @Router /hostsList/{name}/enable/{service} [post]
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
// @Router /hostsList/{name}/disable/{service} [post]
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
// @Router /hostsList/{name}/{service} [get]
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

		host.UpForDeletion = true
		if err := db.Save(&host).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to schedule host for deletion"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Host scheduled for deletion"})
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
	db.AutoMigrate(&Host{})

	// Get the hosts list from the pgsql database
	db.Find(&hostsList)

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// Add Swagger documentation endpoint
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Setup authentication routes
	SetupAuthRoutes(r, db)

	// Use the handler functions
	r.POST("/api/v"+apiVersion+"/hostsList", registerHost(db))

	api := r.Group("/api/v" + apiVersion)
	api.Use(AuthMiddleware(db))
	{
		api.GET("/hostsList", getAllHosts(db))
		api.GET("/hostsList/:name", getHostByName())
		api.DELETE("/hostsList/:name", deleteHost(db))
		api.POST("/hostsList/:name/updateTo/:version", updateHostVersion(db))
		api.POST("/hostsList/:name/enable/:service", enableComponent(db))
		api.POST("/hostsList/:name/disable/:service", disableComponent(db))
		api.GET("/hostsList/:name/:service", getComponentStatus())
	}

	admin := r.Group("/api/v1/admin")
	admin.Use(AuthMiddleware(db))
	{
		admin.DELETE("/hosts/:hostname", scheduleHostDeletion(db))
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
