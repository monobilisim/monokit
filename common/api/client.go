package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Client struct {
	URL string
}

var ClientConf Client

type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Groups   string `json:"groups"`
	} `json:"user"`
}

type ClientAuth struct {
	Token string
}

var AuthConfig ClientAuth

type AdminGroupResponse struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Hosts []Host `json:"hosts"`
}

func GetServiceStatus(serviceName string) (bool, string) {
	apiVersion := "1"

	req, err := http.NewRequest("GET", ClientConf.URL+"/api/v"+apiVersion+"/hostsList/"+common.Config.Identifier+"/"+serviceName, nil)

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

	return fmt.Sprintf("%vGB", memory.Total/1024/1024/1024)
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
	req, err := http.NewRequest("GET", ClientConf.URL+"/api/v"+apiVersion+"/hostsList/"+common.Config.Identifier, nil)

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
	groups := "nil"

	if beforeHost != nil && beforeHost["disabledComponents"] != nil {
		disabledComponents = beforeHost["disabledComponents"].(string)
	}

	if beforeHost != nil && beforeHost["groups"] != nil {
		groups = beforeHost["groups"].(string)
	}

	if disabledComponents == "" {
		disabledComponents = "nil" // If there is no disabled components, set it to nil
	}

	if groups == "" {
		groups = "nil" // If there is no groups, set it to nil
	}

	// Marshal the response to Host struct
	host := Host{Name: common.Config.Identifier, CpuCores: GetCPUCores(), Ram: GetRAM(), MonokitVersion: common.MonokitVersion, Os: GetOS(), DisabledComponents: disabledComponents, IpAddress: GetIP(), Status: "Online", WantsUpdateTo: "", Groups: groups}

	// Marshal the response to JSON
	hostJson, _ := json.Marshal(host)

	// Send the response to the API
	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v"+apiVersion+"/hostsList", bytes.NewBuffer(hostJson))

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
		req, err := http.NewRequest("GET", ClientConf.URL+"/api/v"+apiVersion+"/hostsList/"+hostName, nil)

		if err != nil {
			common.LogError(err.Error())
			return nil
		}

		addAuthHeader(req)

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
		req, err := http.NewRequest("GET", ClientConf.URL+"/api/v"+apiVersion+"/hostsList", nil)

		if err != nil {
			common.LogError(err.Error())
			return nil
		}

		addAuthHeader(req)

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
	for _, host := range hosts {
		fmt.Println(common.Blue + host.Name + common.Reset)
		fmt.Println(common.Green + "\tStatus: " + common.Reset + host.Status)
		fmt.Println(common.Green + "\tCPU: " + common.Reset + fmt.Sprintf("%v cores", host.CpuCores))
		fmt.Println(common.Green + "\tMEM: " + common.Reset + fmt.Sprintf("%v", host.Ram))
		fmt.Println(common.Green + "\tOS: " + common.Reset + host.Os)
		fmt.Println(common.Green + "\tIP: " + common.Reset + host.IpAddress)
		fmt.Println(common.Green + "\tMonokit Version: " + common.Reset + host.MonokitVersion)

		// Add groups display
		if host.Groups != "" && host.Groups != "nil" {
			fmt.Println(common.Green + "\tGroups: " + common.Reset + host.Groups)
		} else {
			fmt.Println(common.Green + "\tGroups: " + common.Reset + "none")
		}

		if host.DisabledComponents != "" && host.DisabledComponents != "nil" {
			fmt.Println(common.Green + "\tDisabled Components: " + common.Reset + host.DisabledComponents)
		}

		if host.WantsUpdateTo != "" {
			fmt.Println(common.Green + "\tWill update to: " + common.Reset + host.WantsUpdateTo)
		}

		fmt.Println(common.Green + "\tUpdated At: " + common.Reset + fmt.Sprintf("%v", host.UpdatedAt))
		fmt.Println(common.Green + "\tCreated At: " + common.Reset + fmt.Sprintf("%v", host.CreatedAt))
		fmt.Println()
	}
}

func SendUpdateTo(apiVersion string, hostName string, versionTo string) {
	// POST /api/v1/hostsList/{hostName}/updateTo/{versionTo}
	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v"+apiVersion+"/hostsList/"+hostName+"/updateTo/"+versionTo, nil)

	if err != nil {
		common.LogError(err.Error())
		return
	}

	addAuthHeader(req)

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

	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v"+apiVersion+"/hostsList/"+hostName+"/disable/"+component, nil)

	if err != nil {
		common.LogError(err.Error())
		return
	}

	addAuthHeader(req)

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
	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v"+apiVersion+"/hostsList/"+hostName+"/enable/"+component, nil)

	if err != nil {
		common.LogError(err.Error())
		return
	}

	addAuthHeader(req)

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

	// Load auth token if it exists
	AuthConfig.Token = viper.GetString("auth.token")

	return apiVersion
}

func Login(username, password string) error {
	loginReq := LoginRequest{
		Username: username,
		Password: password,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("error marshaling login request: %v", err)
	}

	resp, err := http.Post(ClientConf.URL+"/api/v1/auth/login",
		"application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		return fmt.Errorf("login failed: %s", errorResp["error"])
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("error decoding response: %v", err)
	}

	// Save token to config using viper
	AuthConfig.Token = loginResp.Token
	viper.Set("auth.token", loginResp.Token)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("error saving auth token: %v", err)
	}

	fmt.Printf("Logged in as %s (%s)\n", loginResp.User.Username, loginResp.User.Role)
	return nil
}

func addAuthHeader(req *http.Request) {
	if AuthConfig.Token != "" {
		req.Header.Set("Authorization", AuthConfig.Token)
	}
}

func LoginCmd(cmd *cobra.Command, args []string) {
	ClientInit() // Just call ClientInit() without storing the return value

	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")

	if username == "" || password == "" {
		fmt.Println("error: username and password are required")
		os.Exit(1)
	}

	if err := Login(username, password); err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}
}

// Add this to your init() function or wherever you set up commands
func init() {
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login to the API server",
		Run:   LoginCmd,
	}
	loginCmd.Flags().String("username", "", "Username for login")
	loginCmd.Flags().String("password", "", "Password for login")

	// Add to root command
	// rootCmd.AddCommand(loginCmd)
}

// Add these API functions after the existing ones

func adminGet(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", ClientConf.URL+"/api/v1/admin/"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	addAuthHeader(req)
	client := &http.Client{}
	return client.Do(req)
}

func adminPost(path string, data []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v1/admin/"+path, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	addAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	return client.Do(req)
}

func adminDelete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", ClientConf.URL+"/api/v1/admin/"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	addAuthHeader(req)
	client := &http.Client{}
	return client.Do(req)
}

// Update the admin command functions to use these helpers
func AdminGroupsAdd(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: group name required")
		os.Exit(1)
	}

	groupName := args[0]
	data := []byte(fmt.Sprintf(`{"name":"%s"}`, groupName))

	resp, err := adminPost("groups", data)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		os.Exit(1)
	}

	fmt.Printf("Group '%s' created successfully\n", groupName)
}

func AdminGroupsRm(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: group name required")
		os.Exit(1)
	}

	groupName := args[0]
	resp, err := adminDelete("groups/" + groupName)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		os.Exit(1)
	}

	fmt.Printf("Group '%s' removed successfully\n", groupName)
}

func AdminGroupsGet(cmd *cobra.Command, args []string) {
	ClientInit()

	resp, err := adminGet("groups")
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var groups []AdminGroupResponse
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		fmt.Printf("error decoding response: %v\n", err)
		os.Exit(1)
	}

	for _, group := range groups {
		fmt.Printf("Group: %s\n", group.Name)
		if len(group.Hosts) > 0 {
			fmt.Println("  Hosts:")
			for _, host := range group.Hosts {
				fmt.Printf("    - %s\n", host.Name)
			}
		}
	}
}

func AdminGroupsAddHost(cmd *cobra.Command, args []string) {
	ClientInit()

	groupName, _ := cmd.Flags().GetString("group")
	if groupName == "" {
		fmt.Println("error: group name required")
		os.Exit(1)
	}

	if len(args) == 0 {
		fmt.Println("error: hostname required")
		os.Exit(1)
	}

	hostname := args[0]
	resp, err := adminPost("groups/"+groupName+"/hosts/"+hostname, nil)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		os.Exit(1)
	}

	fmt.Printf("Host '%s' added to group '%s' successfully\n", hostname, groupName)
}
