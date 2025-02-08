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

func GetServiceStatus(serviceName string) (bool, string) {
    apiVersion := "1"

    req, err := http.NewRequest("GET", ClientConf.URL + "/api/v" + apiVersion + "/hostsList/" + common.Config.Identifier + "/" + serviceName, nil)

    if err != nil {
        common.LogError(err.Error())
    }

    client := &http.Client{}

    resp, err := client.Do(req)

    if err != nil {
        common.LogError(err.Error())
        return true, ""
    }

    defer resp.Body.Close()
    
    // Demarshal the response
    var serviceStatus map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&serviceStatus)
   
    wantsUpdateTo := ""
    if serviceStatus["wantsUpdateTo"] != nil {
        wantsUpdateTo = serviceStatus["wantsUpdateTo"].(string)
    }

    if serviceStatus["status"] == nil {
        return true, ""
    }
    
    return (serviceStatus["status"] == "enabled" || serviceStatus["status"] == "not found"), wantsUpdateTo
}

func WrapperGetServiceStatus(serviceName string) {
    common.ConfInit("client", &ClientConf)
    
    if ClientConf.URL == "" {
        return
    }

    status, updateVersion := GetServiceStatus(serviceName)
    
    if !status {
        fmt.Println(serviceName + " is disabled. Exiting...")
        // Remove lockfile
        common.RemoveLockfile()
        os.Exit(0)
    }

    if updateVersion != common.MonokitVersion && updateVersion != "" {
        fmt.Println(serviceName + " wants to be updated to " + updateVersion)
        common.Update(updateVersion, false)

        // Re-run sendReq after the update
        SendReq("1")
    }
    
}

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
    

func GetReq(apiVersion string) (map[string]interface{}, error) {
    req, err := http.NewRequest("GET", ClientConf.URL + "/api/v" + apiVersion + "/hostsList/" + common.Config.Identifier, nil)

    if err != nil {
        common.LogError(err.Error())
        return nil, err
    }

    client := &http.Client{}

    resp, err := client.Do(req)

    if err != nil {
        common.LogError(err.Error())
        return nil, err
    }

    defer resp.Body.Close()
    
    // Demarshal the response
    var host map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&host)
    
    return host, nil
}

func SendReq(apiVersion string) {
    
    beforeHost, err := GetReq(apiVersion)
    
    if err != nil {
        return
    }
    enabledComponents := "osHealth::pmgHealth::zimbraHealth::rmqHealth"

    if beforeHost != nil && beforeHost["enabledComponents"] != nil {
        enabledComponents = beforeHost["enabledComponents"].(string)
    }

    if enabledComponents == "" {
        enabledComponents = "nil" // If there is no enabled components, set it to nil
    }
    
    // Marshal the response to Host struct
    host := Host{Name: common.Config.Identifier, CpuCores: GetCPUCores(), Ram: GetRAM(), MonokitVersion: common.MonokitVersion, Os: GetOS(), EnabledComponents: enabledComponents, IpAddress: GetIP(), Status: "Online", WantsUpdateTo: ""}

    // Marshal the response to JSON
    hostJson, _ := json.Marshal(host)

    // Send the response to the API
    req, err := http.NewRequest("POST", ClientConf.URL + "/api/v" + apiVersion + "/hostsList", bytes.NewBuffer(hostJson))

    if err != nil {
        common.LogError(err.Error())
        return
    }

    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    
    resp, err := client.Do(req)

    if err != nil {
        common.LogError(err.Error())
        return
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
