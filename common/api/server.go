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

// ServerMain is the main entry point for the API server
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

	// Migrate the schema in the correct order
	db.AutoMigrate(&Inventory{}) // First create Inventory table
	db.AutoMigrate(&Host{})      // Then Host table that references Inventory
	db.AutoMigrate(&HostKey{})
	db.AutoMigrate(&Group{})
	db.AutoMigrate(&User{})
	db.AutoMigrate(&Session{})
	db.AutoMigrate(&HostLog{}) // Add migration for HostLog table

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

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Setup API routes first
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Setup routes
	setupRoutes(r, db)

	// Setup frontend (conditional based on build tag)
	SetupFrontend(r)

	r.Run(":" + ServerConfig.Port)
}
