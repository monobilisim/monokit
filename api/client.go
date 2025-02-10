package api

import (
    "os"
    "fmt"
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
    if !common.ConfExists("client") {
        return
    }

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
    disabledComponents := "nil"

    if beforeHost != nil && beforeHost["disabledComponents"] != nil {
        disabledComponents = beforeHost["disabledComponents"].(string)
    }

    if disabledComponents == "" {
        disabledComponents = "nil" // If there is no disabled components, set it to nil
    }
    
    // Marshal the response to Host struct
    host := Host{Name: common.Config.Identifier, CpuCores: GetCPUCores(), Ram: GetRAM(), MonokitVersion: common.MonokitVersion, Os: GetOS(), DisabledComponents: disabledComponents, IpAddress: GetIP(), Status: "Online", WantsUpdateTo: ""}

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

func GetHosts(apiVersion string, hostName string) []Host {
    // if hostName is empty, use /api/v1/hostsList
    // if hostName is not empty, use /api/v1/hostsList/{hostName}
    if hostName != "" {
        req, err := http.NewRequest("GET", ClientConf.URL + "/api/v" + apiVersion + "/hostsList/" + hostName, nil)
        
        if err != nil {
            common.LogError(err.Error())
            return nil
        }

        client := &http.Client{}

        resp, err := client.Do(req)

        if err != nil {
            common.LogError(err.Error())
            return nil
        }

        defer resp.Body.Close()

        // Demarshal the response
        var host Host
        json.NewDecoder(resp.Body).Decode(&host)

        return []Host{host}
    } else {
        req, err := http.NewRequest("GET", ClientConf.URL + "/api/v" + apiVersion + "/hostsList", nil)
        
        if err != nil {
            common.LogError(err.Error())
            return nil
        }

        client := &http.Client{}

        resp, err := client.Do(req)

        if err != nil {
            common.LogError(err.Error())
            return nil
        }

        defer resp.Body.Close()

        // Demarshal the response
        var hosts []Host
        json.NewDecoder(resp.Body).Decode(&hosts)


        return hosts
    }
}


func GetHostsPretty(hosts []Host) {
    // Layout
    // {name}
    //      {CpuCores} cores
    //      {Ram}
    //      {Os}
    //      {IpAddress}
    // and so on...

    for _, host := range hosts {
        fmt.Println(common.Blue + host.Name + common.Reset)
        fmt.Println(common.Green + "\tCPU: " + common.Reset + fmt.Sprintf("%v cores", host.CpuCores))
        fmt.Println(common.Green + "\tMEM: " + common.Reset + fmt.Sprintf("%v", host.Ram))
        fmt.Println(common.Green + "\tOS: " + common.Reset + host.Os) 
        fmt.Println(common.Green + "\tIP: " + common.Reset + host.IpAddress)
        fmt.Println(common.Green + "\tStatus: " + common.Reset + host.Status)
        fmt.Println(common.Green + "\tMonokit Version: " + common.Reset + host.MonokitVersion)
        fmt.Println(common.Green + "\tDisabled Components: " + common.Reset + host.DisabledComponents)
        if host.WantsUpdateTo != "" {
            fmt.Println(common.Green + "\tWill update to: " + common.Reset + host.WantsUpdateTo)
        }

        fmt.Println(common.Green + "\tUpdated At: " + common.Reset + fmt.Sprintf("%v", host.UpdatedAt))
        fmt.Println(common.Green + "\tCreated At: " + common.Reset + fmt.Sprintf("%v", host.CreatedAt))
    }
}

func SendUpdateTo(apiVersion string, hostName string, versionTo string) {
    // POST /api/v1/hostsList/{hostName}/updateTo/{versionTo}
    req, err := http.NewRequest("POST", ClientConf.URL + "/api/v" + apiVersion + "/hostsList/" + hostName + "/updateTo/" + versionTo, nil)
    
    if err != nil {
        common.LogError(err.Error())
        return
    }

    client := &http.Client{}

    resp, err := client.Do(req)

    if err != nil {
        common.LogError(err.Error())
        return
    }

    defer resp.Body.Close()

    fmt.Println("Update request sent to " + hostName + " to update to " + versionTo)
}


func SendDisable(apiVersion string, hostName string, component string) {
    // POST /api/v1/hostsList/{hostName}/disable/{component}

    req, err := http.NewRequest("POST", ClientConf.URL + "/api/v" + apiVersion + "/hostsList/" + hostName + "/disable/" + component, nil)

    if err != nil {
        common.LogError(err.Error())
        return
    }

    client := &http.Client{}

    resp, err := client.Do(req)

    if err != nil {
        common.LogError(err.Error())
        return
    }

    defer resp.Body.Close()

    // Demarshal the response
    var response map[string]interface{}

    json.NewDecoder(resp.Body).Decode(&response)

    if response["status"] == "not found" {
        fmt.Println("Host with name " + hostName + " not found.")
        return
    } else if response["status"] == "disabled" {
        fmt.Println("Component " + component + " is now disabled on " + hostName)
    }
}

func SendEnable(apiVersion string, hostName string, component string) {
    // POST /api/v1/hostsList/{hostName}/enable/{component}
    req, err := http.NewRequest("POST", ClientConf.URL + "/api/v" + apiVersion + "/hostsList/" + hostName + "/enable/" + component, nil)

    if err != nil {
        common.LogError(err.Error())
        return
    }

    client := &http.Client{}

    resp, err := client.Do(req)

    if err != nil {
        common.LogError(err.Error())
        return
    }

    defer resp.Body.Close()

    // Demarshal the response
    var response map[string]interface{}

    json.NewDecoder(resp.Body).Decode(&response)

    if response["status"] == "not found" {
        fmt.Println("Host with name " + hostName + " not found.")
        return
    } else if response["status"] == "already enabled" {
        fmt.Println("Component " + component + " is already enabled on " + hostName)
        return
    } else if response["status"] == "enabled" {
        fmt.Println("Component " + component + " is now enabled on " + hostName)
    }
}

func Update(cmd *cobra.Command, args []string) {
    apiVersion := ClientInit()

    SendReq(apiVersion)
}

func Get(cmd *cobra.Command, args []string) {
    apiVersion := ClientInit()

    if len(args) > 0 {
        for _, hostName := range args {
            GetHostsPretty(GetHosts(apiVersion, hostName))
        }
    } else {
        GetHostsPretty(GetHosts(apiVersion, ""))
    }
}


func Upgrade(cmd *cobra.Command, args []string) {
    apiVersion := ClientInit()
    versionTo, _ := cmd.Flags().GetString("version")
    for _, hostName := range args {
        SendUpdateTo(apiVersion, hostName, versionTo)
    }
}

func Enable(cmd *cobra.Command, args []string) {
    apiVersion := ClientInit()
    component, _ := cmd.Flags().GetString("component")
    for _, hostName := range args {
        SendEnable(apiVersion, hostName, component)
    }
}

func Disable(cmd *cobra.Command, args []string) {
    apiVersion := ClientInit()
    component, _ := cmd.Flags().GetString("component")
    for _, hostName := range args {
        SendDisable(apiVersion, hostName, component)
    }
}

func ClientInit() string {
    version := "1.0.0"
    apiVersion := strings.Split(version, ".")[0]
    common.ScriptName = "client"
    common.TmpDir = common.TmpDir + "client"
    common.Init()
    common.ConfInit("client", &ClientConf)

    if ClientConf.URL == "" {
        fmt.Println("error: API URL is not set.")
        os.Exit(1)
    }

    return apiVersion
}
