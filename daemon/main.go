package daemon

import (
    "os"
    "fmt"
    "time"
    "os/exec"
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/common"
    "github.com/monobilisim/monokit/osHealth"
    "github.com/monobilisim/monokit/k8sHealth"
    "github.com/monobilisim/monokit/pritunlHealth"
    "github.com/monobilisim/monokit/wppconnectHealth"
)

type HealthCheck struct {
    Name string // Name of the health check, eg. mysqld
    Enabled bool 
}

type Daemon struct {
    Frequency int // Frequency to run health checks
    Debug    bool // Debug mode
    Health_Checks []HealthCheck
}

var DaemonConfig Daemon

func IsEnabled(name string) (bool, bool) {
    for _, hc := range DaemonConfig.Health_Checks {
        if hc.Name == name {
            return true, hc.Enabled
        }
    }

    return false, false
}

func CommExists(command string, confCheckOnly bool) bool {
    path, _ := exec.LookPath(command)
   
    existsOnConfig, enabled := IsEnabled(command)

    if existsOnConfig {
        return enabled
    }

    if path != "" && !confCheckOnly {
        return true
    } 

    return false
    
}

func Main(cmd *cobra.Command, args []string) {
    version := "1.0.0"
    common.Init()

    if common.ConfExists("daemon") {
        common.ConfInit("daemon", &DaemonConfig)
    } else {
        DaemonConfig.Frequency = 60
    }


    fmt.Println("Monokit daemon - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))
    
    runOnce, _ := cmd.Flags().GetBool("once")
    
    if runOnce {
        fmt.Println("Running once")
        RunAll()
        os.Exit(0)
    }
    
    for {
        RunAll()
        time.Sleep(time.Duration(DaemonConfig.Frequency) * time.Second)
    }
}


func RunAll() {

    common.Update("", false)
  

    var osHealthCmd = &cobra.Command{
        Run: osHealth.Main,
        DisableFlagParsing: true,
    }
    osHealthCmd.ExecuteC()
    
    if CommExists("pritunl", false) {
        var pritunlHealthCmd = &cobra.Command{
            Run: pritunlHealth.Main,
            DisableFlagParsing: true,
        }
        pritunlHealthCmd.ExecuteC()
    } 

    if CommExists("postal", false) {
        PostalCommandExecute()
    }

    if CommExists("pmgversion", false) {
        PmgCommandExecute()
    }
    
    if CommExists("k8s", true) {
        var k8sHealthCmd = &cobra.Command{
            Run: k8sHealth.Main,
            DisableFlagParsing: true,
        }
        k8sHealthCmd.ExecuteC()
    }

    if CommExists("mysqld", false) || CommExists("mariadbd", false) {
        MysqlCommandExecute()
    }
    
    if CommExists("redis-server", false) {
        RedisCommandExecute()
    }
   
    if CommExists("rabbitmq-server", false) {
        RmqCommandExecute()
    }

    if CommExists("traefik", false) {
        TraefikCommandExecute()
    }

    if CommExists("wppconnect", true) {
        wppconnectHealthCmd := &cobra.Command{
            Run: wppconnectHealth.Main,
            DisableFlagParsing: true,
        }
        wppconnectHealthCmd.ExecuteC()
    }
}
