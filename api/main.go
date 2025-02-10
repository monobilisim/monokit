package api

import (
    "fmt"
    "time"
    "slices"
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
    UpdatedAt time.Time `json:"UpdatedAt"`
    CreatedAt time.Time `json:"CreatedAt"`
    WantsUpdateTo string `json:"wantsUpdateTo"`
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

    gin.SetMode(gin.ReleaseMode)
    
    r := gin.Default()

    r.GET("/api/v" + apiVersion + "/hostsList", func(c *gin.Context) {
        // Check UpdatedAt
        for i := 0; i < len(hostsList); i++ {
            if time.Since(hostsList[i].UpdatedAt).Minutes() > 5 {
                hostsList[i].Status = "Offline"
            }
        }

        c.JSON(http.StatusOK, hostsList)
    })
    
    r.POST("/api/v" + apiVersion + "/hostsList", func(c *gin.Context) {
        var host Host
        
        if err := c.ShouldBindJSON(&host); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        
        update := false

        for i := 0; i < len(hostsList); i++ {
            if hostsList[i].Name == host.Name {
                update = true
            }
        }

        if update {
            // Update the host in the pgsql database
            db.Model(&Host{}).Where("name = ?", host.Name).Updates(&host)
        } else {
            // Add the host to the pgsql database
            db.Create(&host)
        }

        // Sync the hosts list
        db.Find(&hostsList)

        c.JSON(http.StatusOK, hostsList)
    })
    
    r.GET("/api/v" + apiVersion + "/hostsList/:name", func(c *gin.Context) {
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

    r.DELETE("/api/v" + apiVersion + "/hostsList/:name", func(c *gin.Context) {
        name := c.Param("name")

        // Delete the host from the pgsql database
        db.Where("name = ?", name).Delete(&Host{})

        // Sync the hosts list
        db.Find(&hostsList)

        c.JSON(http.StatusOK, hostsList)
    })

    r.POST("/api/v" + apiVersion + "/hostsList/:name/updateTo/:version", func(c *gin.Context) {
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

    r.POST("/api/v" + apiVersion + "/hostsList/:name/enable/:service", func(c *gin.Context) {
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

        enabledComponents := strings.Split(host.EnabledComponents, "::")

        for j := 0; j < len(enabledComponents); j++ {
            if enabledComponents[j] == service {
                c.JSON(http.StatusOK, gin.H{"status": "already enabled"})
                return
            }
        }

        host.EnabledComponents = host.EnabledComponents + "::" + service

        // Update the host in the pgsql database

        db.Model(&Host{}).Where("name = ?", name).Updates(&host)

        // Sync the hosts list

        db.Find(&hostsList)

        c.JSON(http.StatusOK, gin.H{"status": "enabled"})

    })


    r.POST("/api/v" + apiVersion + "/hostsList/:name/disable/:service", func(c *gin.Context) {
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

        enabledComponents := strings.Split(host.EnabledComponents, "::")

        for j := 0; j < len(enabledComponents); j++ {
            if enabledComponents[j] == service {
                enabledComponents = append(enabledComponents[:j], enabledComponents[j+1:]...)
                break
            }
        }

        host.EnabledComponents = strings.Join(enabledComponents, "::")

        // Update the host in the pgsql database

        db.Model(&Host{}).Where("name = ?", name).Updates(&host)

        // Sync the hosts list

        db.Find(&hostsList)

        c.JSON(http.StatusOK, gin.H{"status": "disabled"})

    })





    r.GET("/api/v" + apiVersion + "/hostsList/:name/:service", func(c *gin.Context) {
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
        enabledComponents := strings.Split(host.EnabledComponents, "::")
        for j := 0; j < len(enabledComponents); j++ {
            if enabledComponents[j] == service {
                c.JSON(http.StatusOK, gin.H{"status": "enabled", "wantsUpdateTo": wantsUpdateTo})
                return
            }
        }

        c.JSON(http.StatusOK, gin.H{"status": "disabled", "wantsUpdateTo": wantsUpdateTo})
    })

    r.Run(":" + ServerConfig.Port)
}
