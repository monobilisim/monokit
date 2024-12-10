package lbPolicy

import (
    "fmt"
    "time"
    "strings" 
    "io/ioutil"
	"github.com/spf13/cobra"
	"github.com/monobilisim/monokit/common"
)

type ConfigStruct struct {
    Caddy struct {
        Api_Urls []string
        Servers []string
        Lb_Urls []string
        Override_Config bool
        Nochange_Exit_Threshold int
        Dynamic_Api_Urls bool
        Loop_Order string
        Lb_Policy_Change_Sleep time.Duration
    }
}

var Config ConfigStruct

func ConfReset() {
    Config = ConfigStruct{}
}

func LoopOverConfigs(server string, configs []string, t string) {
    if len(configs) > 0 {
        var printConfig bool

        if len(configs) > 1 {
            printConfig = true
        }
        
        for _, config := range configs {
            ConfReset()
            if printConfig {
                fmt.Println("Config: " + config)
            }
            common.ConfInit("glb-" + config, &Config)
            if t == "switch" {
                SwitchMain(server)
            } else if t == "list" {
                ShowListMulti(Config.Caddy.Servers)
            }

            if printConfig {
                fmt.Println("")
            }
        }
    } else {
        // Loop over all files at /etc/mono that start with glb-
        entries, err := ioutil.ReadDir("/etc/mono/")
        if err != nil {
            common.LogError(err.Error())
        }
        for _, file := range entries {
            if strings.HasPrefix(file.Name(), "glb-") {
                ConfReset()
                fmt.Println("Config: " + file.Name())
                common.ConfInit(file.Name(), &Config)
                if t == "switch" {
                    SwitchMain(server)
                } else if t == "list" {
                    ShowListMulti(Config.Caddy.Servers)
                }
            }
        }
    }
}


func Switch(cmd *cobra.Command, args []string) {
    //version := "2.0.0"
    common.ScriptName = "lbPolicy"
	common.TmpDir = common.TmpDir + "glb"
	common.Init()
    config, _ := cmd.Flags().GetStringArray("configs")
    server, _ := cmd.Flags().GetString("server")
    fmt.Println("Server: " + server)
    LoopOverConfigs(server, config, "switch")
}

func List(cmd *cobra.Command, args []string) {
	//version := "2.0.0"
	common.ScriptName = "lbPolicy"
	common.TmpDir = "/tmp/glb"
	common.Init()
    config, _ := cmd.Flags().GetStringArray("configs")
    LoopOverConfigs("", config, "list")
}
