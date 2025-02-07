package api

import (
    "fmt"
    "time"
    "strings"
    "net/http"
    "gorm.io/gorm"
    "gorm.io/driver/postgres"
    "github.com/spf13/viper"
    "github.com/spf13/cobra"
    "github.com/gin-gonic/gin"
    "github.com/monobilisim/monokit/common"
)

type Server struct {
    Port string
    Postgres struct {
        Host string
        Port string
        User string
        Password string
        Dbname string
    }
}

type Host struct {
    gorm.Model
    Name string `json:"name"`
    CpuCores int `json:"cpuCores"`
    Ram string `json:"ram"`
    MonokitVersion string `json:"monokitVersion"`
    Os string `json:"os"`
    EnabledComponents string `json:"enabledComponents"`
    IpAddress string `json:"ipAddress"`
    Status string `json:"status"`
}

var ServerConfig Server
var hostsList []Host

func Main(cmd *cobra.Command, args []string) {
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

    r := gin.Default()

    //gin.SetMode(gin.ReleaseMode)

    r.GET("/api/v" + apiVersion + "/hostsList", func(c *gin.Context) {
        c.JSON(http.StatusOK, hostsList)     
    })
    
    r.POST("/api/v" + apiVersion + "/hostsList", func(c *gin.Context) {
        var host Host
        
        if err := c.ShouldBindJSON(&host); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
       
        for i := 0; i < len(hostsList); i++ {
            if hostsList[i].Name == host.Name {
                // Remove the host from the pgsql database
            }
        }

        // Add the host to the pgsql database
        db.Create(&host)

        // Sync the hosts list
        db.Find(&hostsList)

        c.JSON(http.StatusOK, hostsList)
    })
    
    r.GET("/api/v" + apiVersion + "/hostsList/:name", func(c *gin.Context) {
        name := c.Param("name")
        var host Host

        // Get the host from the pgsql database
        db.Where("name = ?", name).First(&host)
        
        c.JSON(http.StatusOK, host)
    })

    r.Run(":" + ServerConfig.Port)
}
