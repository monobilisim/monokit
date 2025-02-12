package common

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	Name               string    `json:"name"`
	CpuCores           int       `json:"cpuCores"`
	Ram                string    `json:"ram"`
	MonokitVersion     string    `json:"monokitVersion"`
	Os                 string    `json:"os"`
	DisabledComponents string    `json:"disabledComponents"`
	IpAddress          string    `json:"ipAddress"`
	Status             string    `json:"status"`
	UpdatedAt          time.Time `json:"UpdatedAt"`
	CreatedAt          time.Time `json:"CreatedAt"`
	WantsUpdateTo      string    `json:"wantsUpdateTo"`
	Groups             string    `json:"groups"`
}

var ServerConfig Server
var hostsList []Host

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

	// Setup authentication routes
	SetupAuthRoutes(r, db)

	// Unprotected route for hosts to register themselves
	r.POST("/api/v"+apiVersion+"/hostsList", func(c *gin.Context) {
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
	})

	// Protected API group
	api := r.Group("/api/v" + apiVersion)
	api.Use(AuthMiddleware(db))
	{
		api.GET("/hostsList", func(c *gin.Context) {
			// Sync all hosts' groups before returning
			for i := range hostsList {
				syncHostGroups(db, &hostsList[i])
				db.Save(&hostsList[i])
			}

			// Check UpdatedAt
			for i := 0; i < len(hostsList); i++ {
				if time.Since(hostsList[i].UpdatedAt).Minutes() > 5 {
					hostsList[i].Status = "Offline"
				}
			}

			// If user is not admin, use filtered hosts
			if filteredHosts, exists := c.Get("filteredHosts"); exists {
				c.JSON(http.StatusOK, filteredHosts)
				return
			}

			c.JSON(http.StatusOK, hostsList)
		})

		api.GET("/hostsList/:name", func(c *gin.Context) {
			name := c.Param("name")

			idx := slices.IndexFunc(hostsList, func(h Host) bool {
				return h.Name == name
			})

			if idx == -1 {
				c.JSON(http.StatusOK, gin.H{"status": "not found"})
				return
			}

			c.JSON(http.StatusOK, hostsList[idx])
		})

		api.DELETE("/hostsList/:name", func(c *gin.Context) {
			name := c.Param("name")

			// Delete the host from the pgsql database
			db.Where("name = ?", name).Delete(&Host{})

			// Sync the hosts list
			db.Find(&hostsList)

			c.JSON(http.StatusOK, hostsList)
		})

		api.POST("/hostsList/:name/updateTo/:version", func(c *gin.Context) {
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

		})

		api.POST("/hostsList/:name/enable/:service", func(c *gin.Context) {
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

		})

		api.POST("/hostsList/:name/disable/:service", func(c *gin.Context) {
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

		})

		api.GET("/hostsList/:name/:service", func(c *gin.Context) {
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
		})
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
