package api

import (
    "os"
    "fmt"
    "time"
    "bytes"
    "strings"
    "net/http"
    "encoding/json"
    "github.com/spf13/cobra"
    "github.com/shirou/gopsutil/v4/cpu"
    "github.com/shirou/gopsutil/v4/mem"
    "github.com/shirou/gopsutil/v4/net"
    "github.com/shirou/gopsutil/v4/host"
    "github.com/monobilisim/monokit/common"
)
type Client struct {
    URL string
}

var ClientConf Client

func GetCPUCores() int {
    cpuCount, err := cpu.Counts(true)
    if err != nil {
        common.LogError(err.Error())
        return 0
    }
    return cpuCount
}

func GetRAM() string {
    memory, err := mem.VirtualMemory()
    if err != nil {
        common.LogError(err.Error())
        return ""
    }

    return fmt.Sprintf("%vGB", memory.Total / 1024 / 1024 / 1024)
}

func GetIP() string {
    interfaces, err := net.Interfaces()
    if err != nil {
        common.LogError(err.Error())
        return ""
    }

    for _, iface := range interfaces {
        if iface.Name != "lo" {
            return strings.Split(iface.Addrs[0].Addr, "/")[0]
        }
    }

    return ""
}

func GetOS() string {
    info, err := host.Info()
    if err != nil {
        common.LogError(err.Error())
        return ""
    }

    return info.Platform + " " + info.PlatformVersion + " " + info.KernelVersion
}
    

func SendReq(apiVersion string) {

    // Marshal the response to Host struct
    host := Host{Name: common.Config.Identifier, CpuCores: GetCPUCores(), Ram: GetRAM(), MonokitVersion: common.MonokitVersion, Os: GetOS(), EnabledComponents: "osHealth::zimbraHealth", IpAddress: GetIP(), Status: "Online"}

    // Marshal the response to JSON
    hostJson, _ := json.Marshal(host)

    // Send the response to the API
    req, err := http.NewRequest("POST", ClientConf.URL + "/api/v" + apiVersion + "/hostsList", bytes.NewBuffer(hostJson))

    if err != nil {
        common.LogError(err.Error())
    }

    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    
    resp, err := client.Do(req)

    if err != nil {
        common.LogError(err.Error())
    }

    defer resp.Body.Close()
}

    
func ClientMain(cmd *cobra.Command, args []string) {
    version := "1.0.0"
    apiVersion := strings.Split(version, ".")[0]
    common.ScriptName = "client"
    common.TmpDir = common.TmpDir + "client"
    common.Init()
    common.ConfInit("client", &ClientConf)

    fmt.Println("Monokit API Client - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05") + " - API v" + apiVersion)

    if ClientConf.URL == "" {
        fmt.Println("error: API URL is not set.")
        os.Exit(1)
    }

    SendReq(apiVersion)
}
