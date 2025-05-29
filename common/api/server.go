//go:build with_api

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

// @securityDefinitions.oauth2.implicit KeycloakAuth
// @tokenUrl https://keycloak.example.com/auth/realms/your-realm/protocol/openid-connect/token
// @authorizationUrl https://keycloak.example.com/auth/realms/your-realm/protocol/openid-connect/auth
// @scope read Grants read access
// @scope write Grants write access

// @tag.name hosts
// @tag.description Host management operations

// @tag.name auth
// @tag.description Authentication operations

// @tag.name admin
// @tag.description Admin operations

// @tag.name logs
// @tag.description Log management operations

package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common"
	_ "github.com/monobilisim/monokit/docs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ServerDeps struct {
	LoadConfig  func()
	OpenDB      func() (*gorm.DB, error)
	SetupDB     func(db *gorm.DB)
	BuildRouter func(db *gorm.DB) *gin.Engine
	RunRouter   func(r *gin.Engine) error
}

func serverMainWithDeps(deps ServerDeps) {
	deps.LoadConfig()
	db, err := deps.OpenDB()
	if err != nil {
		panic("failed to connect database")
	}
	deps.SetupDB(db)
	r := deps.BuildRouter(db)
	err = deps.RunRouter(r)
	if err != nil {
		panic("failed to run router")
	}
}

// ServerMain is the main entry point for the API server
func ServerMain(cmd *cobra.Command, args []string) {
	version := "1.0.0"
	apiVersion := strings.Split(version, ".")[0]
	common.ScriptName = "server"
	common.TmpDir = common.TmpDir + "server"
	common.Init()

	defaultDeps := ServerDeps{
		LoadConfig: func() {
			viper.SetDefault("port", "9989")
			common.ConfInit("server", &ServerConfig)
			fmt.Println("Monokit API Server - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05") + " - API v" + apiVersion)
		},
		OpenDB: func() (*gorm.DB, error) {
			dsn := "host=" + ServerConfig.Postgres.Host + " user=" + ServerConfig.Postgres.User + " password=" + ServerConfig.Postgres.Password + " dbname=" + ServerConfig.Postgres.Dbname + " port=" + ServerConfig.Postgres.Port + " sslmode=disable TimeZone=Europe/Istanbul"
			return gorm.Open(postgres.Open(dsn), &gorm.Config{})
		},
		SetupDB: func(db *gorm.DB) {
			db.AutoMigrate(&Inventory{}) // First create Inventory table
			db.AutoMigrate(&Host{})      // Then Host table that references Inventory
			db.AutoMigrate(&HostKey{})
			db.AutoMigrate(&Group{})
			db.AutoMigrate(&User{})
			db.AutoMigrate(&Session{})
			db.AutoMigrate(&HostLog{})        // Add migration for HostLog table
			db.AutoMigrate(&HostFileConfig{}) // Add migration for host file configs
			db.Exec("CREATE INDEX IF NOT EXISTS idx_host_logs_timestamp ON host_logs (timestamp)")
			// Create default inventory if it doesn't exist
			var defaultInventory Inventory
			if db.Where("name = ?", "default").First(&defaultInventory).Error == gorm.ErrRecordNotFound {
				db.Create(&Inventory{Name: "default"})
			}
			// Create initial admin user if no users exist
			if err := createInitialAdmin(db); err != nil {
				fmt.Printf("Warning: Failed to create initial admin user: %v\n", err)
			}
			// Get the hosts list from the pgsql database
			db.Find(&HostsList)
			// Fix any duplicate host names
			fmt.Println("=============== RUNNING DUPLICATE HOST FIX (MAIN) ===============")
			FixDuplicateHosts(db)
			fmt.Println("=============== DUPLICATE HOST FIX COMPLETED (MAIN) ===============")
		},
		BuildRouter: func(db *gorm.DB) *gin.Engine {
			gin.SetMode(gin.ReleaseMode)
			r := gin.Default()
			r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
			setupRoutes(r, db)
			SetupFrontend(r)
			return r
		},
		RunRouter: func(r *gin.Engine) error {
			return r.Run(":" + ServerConfig.Port)
		},
	}

	serverMainWithDeps(defaultDeps)
}

// ServerMainWithDeps exposes the dependency-injected main for testing.
func ServerMainWithDeps(deps ServerDeps) {
	serverMainWithDeps(deps)
}

// FixDuplicateHosts fixes any duplicate host names in the database (exported for testing)
func FixDuplicateHosts(db *gorm.DB) {
	fmt.Println("*** CHECKING FOR DUPLICATE HOST NAMES ***")

	// Get all hosts
	var hosts []Host
	db.Find(&hosts)

	// Track all host names and their counts
	hostCounts := make(map[string]int)
	for _, host := range hosts {
		hostCounts[host.Name]++
	}

	// Find duplicates
	var duplicates []string
	for name, count := range hostCounts {
		if count > 1 {
			duplicates = append(duplicates, name)
			fmt.Printf("Found %d hosts with name '%s'\n", count, name)
		}
	}

	if len(duplicates) == 0 {
		fmt.Println("No duplicate host names found")
		db.Find(&HostsList)
		return
	}

	// Fix each duplicate
	for _, name := range duplicates {
		// Get all hosts with this name
		var dupeHosts []Host
		db.Where("name = ?", name).Order("id asc").Find(&dupeHosts)

		// Keep first one, rename others
		for i := 1; i < len(dupeHosts); i++ {
			newName := fmt.Sprintf("%s-%d", name, i)
			fmt.Printf("Renaming host ID=%d from '%s' to '%s'\n",
				dupeHosts[i].ID, dupeHosts[i].Name, newName)

			// Update the host
			db.Model(&dupeHosts[i]).Update("name", newName)
		}
	}

	// Reload hosts list
	db.Find(&HostsList)
	fmt.Println("Duplicate host names fixed")
}
