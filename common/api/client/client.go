package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Type aliases for commonly used types from models package
type (
	Host              = models.Host
	User              = models.User
	LoginRequest      = models.LoginRequest
	LoginResponse     = models.LoginResponse
	RegisterRequest   = models.RegisterRequest
	UpdateMeRequest   = models.UpdateMeRequest
	InventoryResponse = models.InventoryResponse
)

type Client struct {
	URL        string
	HTTPClient *http.Client // Allows injection for testing
}

// hc returns the configured *http.Client, or http.DefaultClient if nil.
func (c *Client) hc() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func getIdentifier() string {
	if v := os.Getenv("MONOKIT_TEST_IDENTIFIER"); v != "" {
		return v
	}
	// Fallback to the runtime configuration identifier if it has been set
	if common.Config.Identifier != "" {
		return common.Config.Identifier
	}
	// Final fallback to a hard-coded test-safe value
	return "test-host"
}

var ClientConf Client

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
	log.Debug().Str("serviceName", serviceName).Msg("Checking service status")
	apiVersion := "1"

	req, err := http.NewRequest("GET", ClientConf.URL+"/api/v"+apiVersion+"/hosts/"+getIdentifier()+"/"+serviceName, nil)

	log.Debug().Str("url", ClientConf.URL+"/api/v"+apiVersion+"/hosts/"+getIdentifier()+"/"+serviceName).Msg("Sending GET request")

	if err != nil {
		log.Error().Err(err).Msg("Failed to create GET request")
		return true, ""
	}

	// Add host key if available
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", common.Config.Identifier)
	if hostKey, err := os.ReadFile(keyPath); err == nil {
		req.Header.Set("Authorization", string(hostKey))
	}

	client := ClientConf.hc()

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Msg("Failed to send GET request")
		return true, ""
	}

	defer resp.Body.Close()

	// Demarshal the response
	var serviceStatus map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&serviceStatus)

	log.Debug().Str("serviceStatus", fmt.Sprintf("%v", serviceStatus)).Msg("Service status response")

	wantsUpdateTo := ""
	if serviceStatus["wantsUpdateTo"] != nil {
		wantsUpdateTo = serviceStatus["wantsUpdateTo"].(string)
	}

	// First try to check if there's an explicit "status" field
	if serviceStatus["status"] != nil {
		return serviceStatus["status"] == "enabled", wantsUpdateTo
	}

	// Next, check if there's a "disabled" field
	if serviceStatus["disabled"] != nil {
		disabled, ok := serviceStatus["disabled"].(bool)
		if ok {
			return !disabled, wantsUpdateTo // Return !disabled because we want to return true if enabled
		}
	}

	// Default to enabled if we can't determine status
	return true, wantsUpdateTo
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
		common.Update(updateVersion, false, true, []string{}, "/var/lib/monokit/plugins")

		// Re-run sendReq after the update
		SendReq("1")
	}

}

func GetCPUCores() int {
	cpuCount, err := cpu.Counts(true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get CPU cores")
		return 0
	}
	return cpuCount
}

func GetRAM() string {
	memory, err := mem.VirtualMemory()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get RAM")
		return ""
	}

	return fmt.Sprintf("%vGB", memory.Total/1024/1024/1024)
}

func GetIP() string {
	interfaces, err := netInterfacesFn()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get IP")
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
		log.Error().Err(err).Msg("Failed to get OS")
		return ""
	}

	return info.Platform + " " + info.PlatformVersion + " " + info.KernelVersion
}

func GetReq(apiVersion string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", ClientConf.URL+"/api/v"+apiVersion+"/hosts/"+getIdentifier(), nil)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create GET request")
		return nil, err
	}

	client := ClientConf.hc()

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Msg("Failed to send GET request")
		return nil, err
	}

	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Demarshal the response
	var host map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&host)

	return host, nil
}

// ---- Begin shims for test injection ----
// These function vars are used for unit tests. Production code uses the original implementations below.
var (
	syncConfigFn           = SyncConfig
	getReqFn               = GetReq
	getInstalledComponents = common.GetInstalledComponents
	netInterfacesFn        = net.Interfaces
)

// ---- End shims for test injection ----

func SendReq(apiVersion string) {
	// Sync host configuration with server
	//syncConfigFn(nil, nil)

	beforeHost, err := getReqFn(apiVersion)

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

	// Get installed components directly
	installedComponents := getInstalledComponents()

	// Split identifier to get inventory name
	inventoryName := strings.Split(getIdentifier(), "-")[0]

	// Marshal the response to Host struct
	host := Host{
		Name:                common.Config.Identifier,
		CpuCores:            GetCPUCores(),
		Ram:                 GetRAM(),
		MonokitVersion:      common.MonokitVersion,
		Os:                  GetOS(),
		DisabledComponents:  disabledComponents,
		InstalledComponents: installedComponents,
		IpAddress:           GetIP(),
		Status:              "Online",
		WantsUpdateTo:       "",
		Groups:              groups,
		Inventory:           inventoryName,
	}

	// Marshal the response to JSON
	hostJson, _ := json.Marshal(host)

	// Send the response to the API
	log.Debug().Msg("Preparing to send POST request to " + ClientConf.URL + "/api/v" + apiVersion + "/hosts")
	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v"+apiVersion+"/hosts", bytes.NewBuffer(hostJson))

	if err != nil {
		log.Error().Err(err).Msg("Failed to create POST request")
		return
	}

	// Try to read the host key
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", common.Config.Identifier)
	if hostKey, err := os.ReadFile(keyPath); err == nil {
		req.Header.Set("Authorization", string(hostKey))
	}

	req.Header.Set("Content-Type", "application/json")

	client := ClientConf.hc()

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Msg("Failed to send POST request")
		return
	}

	defer resp.Body.Close()

	log.Debug().Msg("POST request completed, decoding response...")
	// Handle the response
	var response struct {
		Host   *Host  `json:"host"`
		ApiKey string `json:"apiKey,omitempty"`
		Error  string `json:"error,omitempty"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	log.Debug().Str("body", string(body)).Msg("Response body")

	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error decoding response: %v\nBody: %s\n", err, string(body))
		return
	}

	if response.Error != "" {
		fmt.Printf("Server error: %s\n", response.Error)
		return
	}

	// If we received an API key, save it
	if response.ApiKey != "" {
		keyPath := "/var/lib/mono/api/hostkey"
		if err := os.MkdirAll(keyPath, 0755); err != nil {
			fmt.Printf("Error creating directory: %v\n", err)
			return
		}

		if err := os.WriteFile(filepath.Join(keyPath, host.Name), []byte(response.ApiKey), 0600); err != nil {
			fmt.Printf("Error writing key file: %v\n", err)
			return
		}
	}

	// Check if this host is scheduled for deletion
	if response.Host != nil && response.Host.UpForDeletion {
		fmt.Println("This host is scheduled for deletion. Running removal process...")
		common.RemoveMonokit()
		os.Exit(0)
	}

	// Remove lockfile
	common.RemoveLockfile()
}

func GetHosts(apiVersion string, hostName string) []Host {
	// if hostName is empty, use /api/v1hosts
	// if hostName is not empty, use /api/v1hosts/{hostName}
	if hostName != "" {
		req, err := http.NewRequest("GET", ClientConf.URL+"/api/v"+apiVersion+"/hosts/"+hostName, nil)

		if err != nil {
			log.Error().Err(err).Msg("Failed to create GET request")
			return nil
		}
		common.AddUserAgent(req)
		addAuthHeader(req)

		client := ClientConf.hc()

		resp, err := client.Do(req)

		if err != nil {
			log.Error().Err(err).Msg("Failed to send GET request")
			return nil
		}

		defer resp.Body.Close()

		// Demarshal the response
		var host Host
		json.NewDecoder(resp.Body).Decode(&host)

		return []Host{host}
	} else {
		req, err := http.NewRequest("GET", ClientConf.URL+"/api/v"+apiVersion+"/hosts", nil)

		if err != nil {
			log.Error().Err(err).Msg("Failed to create GET request")
			return nil
		}

		addAuthHeader(req)

		client := ClientConf.hc()

		resp, err := client.Do(req)

		if err != nil {
			log.Error().Msg(err.Error())
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

		if host.InstalledComponents != "" && host.InstalledComponents != "nil" {
			fmt.Println(common.Green + "\tInstalled Components: " + common.Reset + host.InstalledComponents)
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
	// POST /api/v1hosts/{hostName}/updateTo/{versionTo}
	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v"+apiVersion+"/hosts/"+hostName+"/updateTo/"+versionTo, nil)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create POST request")
		return
	}

	addAuthHeader(req)

	client := ClientConf.hc()

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Msg("Failed to send POST request")
		return
	}

	defer resp.Body.Close()

	fmt.Println("Update request sent to " + hostName + " to update to " + versionTo)
}

func SendDisable(apiVersion string, hostName string, component string) {
	// POST /api/v1hosts/{hostName}/disable/{component}

	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v"+apiVersion+"/hosts/"+hostName+"/disable/"+component, nil)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create POST request")
		return
	}

	addAuthHeader(req)

	client := ClientConf.hc()

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Msg("Failed to send POST request")
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
	// POST /api/v1hosts/{hostName}/enable/{component}
	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v"+apiVersion+"/hosts/"+hostName+"/enable/"+component, nil)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create POST request")
		return
	}

	addAuthHeader(req)

	client := ClientConf.hc()

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Msg("Failed to send POST request")
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

	// Ensure lockfile is removed after update
	common.RemoveLockfile()
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
		common.RemoveLockfile()
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
		common.RemoveLockfile()
		os.Exit(1)
	}

	if err := Login(username, password); err != nil {
		fmt.Printf("Login failed: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}
}

// Update the admin helper functions to check for admin access
func checkAdminAccess(resp *http.Response) error {
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("not admin")
	}
	return nil
}

func adminGet(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", ClientConf.URL+"/api/v1/admin/"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	addAuthHeader(req)
	client := ClientConf.hc()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if err := checkAdminAccess(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func adminPost(path string, data []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", ClientConf.URL+"/api/v1/admin/"+path, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	addAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	client := ClientConf.hc()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if err := checkAdminAccess(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func adminDelete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", ClientConf.URL+"/api/v1/admin/"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	addAuthHeader(req)
	client := ClientConf.hc()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if err := checkAdminAccess(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func AdminGroupsGet(cmd *cobra.Command, args []string) {
	ClientInit()

	resp, err := adminGet("groups")
	if err != nil {
		if err.Error() == "not admin" {
			fmt.Println("error: admin access required")
		} else {
			fmt.Printf("error: %v\n", err)
		}
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	var groups []AdminGroupResponse
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		fmt.Printf("error decoding response: %v\n", err)
		common.RemoveLockfile()
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

func AdminGroupsAdd(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: group name required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	groupName := args[0]
	data := []byte(fmt.Sprintf(`{"name":"%s"}`, groupName))

	resp, err := adminPost("groups", data)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Printf("Group '%s' created successfully\n", groupName)
}

func AdminGroupsRm(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: group name required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	groupName := args[0]
	withHosts, _ := cmd.Flags().GetBool("withHosts")

	url := "groups/" + groupName
	if withHosts {
		url += "?withHosts=true"
	}

	resp, err := adminDelete(url)
	if err != nil {
		if err.Error() == "not admin" {
			fmt.Println("error: admin access required")
		} else {
			fmt.Printf("error: %v\n", err)
		}
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	if withHosts {
		fmt.Printf("Group '%s' and its hosts deleted successfully\n", groupName)
	} else {
		fmt.Printf("Group '%s' deleted successfully\n", groupName)
	}
}

func AdminGroupsAddHost(cmd *cobra.Command, args []string) {
	ClientInit()

	groupName, _ := cmd.Flags().GetString("group")
	if groupName == "" {
		fmt.Println("error: group name required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	if len(args) == 0 {
		fmt.Println("error: hostname required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	hostname := args[0]
	resp, err := adminPost("groups/"+groupName+"/hosts/"+hostname, nil)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Printf("Host '%s' added to group '%s' successfully\n", hostname, groupName)
}

func AdminGroupsRemoveHost(cmd *cobra.Command, args []string) {
	ClientInit()

	groupName, _ := cmd.Flags().GetString("group")
	if groupName == "" {
		fmt.Println("error: group name required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	if len(args) == 0 {
		fmt.Println("error: hostname required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	hostname := args[0]
	resp, err := adminDelete("groups/" + groupName + "/hosts/" + hostname)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Printf("Host '%s' removed from group '%s' successfully\n", hostname, groupName)
}

// Add this function to handle user creation
func AdminUsersCreate(cmd *cobra.Command, args []string) {
	ClientInit()

	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	email, _ := cmd.Flags().GetString("email")
	role, _ := cmd.Flags().GetString("role")
	groups, _ := cmd.Flags().GetString("groups")

	if username == "" || password == "" || email == "" || role == "" {
		fmt.Println("error: username, password, email, and role are required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	data := map[string]string{
		"username": username,
		"password": password,
		"email":    email,
		"role":     role,
		"groups":   groups,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}

	resp, err := adminPost("users", jsonData)
	if err != nil {
		if err.Error() == "not admin" {
			fmt.Println("error: admin access required")
		} else {
			fmt.Printf("error: %v\n", err)
		}
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Printf("User '%s' created successfully with role '%s'\n", username, role)
}

func AdminUsersDelete(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: username required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	username := args[0]
	resp, err := adminDelete("users/" + username)
	if err != nil {
		if err.Error() == "not admin" {
			fmt.Println("error: admin access required")
		} else {
			fmt.Printf("error: %v\n", err)
		}
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Printf("User '%s' deleted successfully\n", username)
}

func adminPut(path string, data []byte) (*http.Response, error) {
	req, err := http.NewRequest("PUT", ClientConf.URL+"/api/v1/admin/"+path, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	addAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	client := ClientConf.hc()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if err := checkAdminAccess(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func AdminUsersUpdate(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: username required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	username := args[0]
	data := map[string]string{}

	// Get all possible flags
	if newUsername, _ := cmd.Flags().GetString("username"); newUsername != "" {
		data["username"] = newUsername
	}
	if password, _ := cmd.Flags().GetString("password"); password != "" {
		data["password"] = password
	}
	if email, _ := cmd.Flags().GetString("email"); email != "" {
		data["email"] = email
	}
	if role, _ := cmd.Flags().GetString("role"); role != "" {
		data["role"] = role
	}
	if groups, _ := cmd.Flags().GetString("groups"); groups != "" {
		data["groups"] = groups
	}

	if len(data) == 0 {
		fmt.Println("error: at least one field to update must be provided")
		common.RemoveLockfile()
		os.Exit(1)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}

	resp, err := adminPut("users/"+username, jsonData)
	if err != nil {
		if err.Error() == "not admin" {
			fmt.Println("error: admin access required")
		} else {
			fmt.Printf("error: %v\n", err)
		}
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Printf("User '%s' updated successfully\n", username)
}

func authPut(path string, data []byte) (*http.Response, error) {
	req, err := http.NewRequest("PUT", ClientConf.URL+"/api/v1/auth/"+path, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	addAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	client := ClientConf.hc()
	return client.Do(req)
}

func UpdateMe(cmd *cobra.Command, args []string) {
	ClientInit()
	data := map[string]string{}
	if username, _ := cmd.Flags().GetString("username"); username != "" {
		data["username"] = username
	}
	if password, _ := cmd.Flags().GetString("password"); password != "" {
		data["password"] = password
	}
	if email, _ := cmd.Flags().GetString("email"); email != "" {
		data["email"] = email
	}

	if len(data) == 0 {
		fmt.Println("error: at least one of username, password, or email must be provided")
		common.RemoveLockfile()
		os.Exit(1)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}

	resp, err := authPut("me/update", jsonData)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Println("User details updated successfully")
}

func AdminHostsDelete(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: hostname required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	hostname := args[0]
	resp, err := adminDelete("hosts/" + hostname)
	if err != nil {
		if err.Error() == "not admin" {
			fmt.Println("error: admin access required")
		} else {
			fmt.Printf("error: %v\n", err)
		}
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Printf("Host '%s' scheduled for deletion\n", hostname)
}

func DeleteMe(cmd *cobra.Command, args []string) {
	ClientInit()

	resp, err := authDelete("me")
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Println("Account deleted successfully")
}

// SyncConfig synchronizes local config with the server.
func SyncConfig(cmd *cobra.Command, args []string) {
	apiVersion := ClientInit()
	hostname := common.Config.Identifier
	configDir := "/etc/mono"
	client := ClientConf.hc()

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create config directory")
		return
	}

	log.Debug().Str("hostname", hostname).Msg("SyncConfig: Starting configuration sync for host")

	// Get host key for authentication
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", hostname)
	hostKey, _ := os.ReadFile(keyPath) // Ignore error, we'll just proceed without host key

	// GET remote configs
	url := ClientConf.URL + "/api/v" + apiVersion + "/hosts/" + hostname + "/config"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error().Err(err).Msg("Error creating GET request")
		return
	}

	if len(hostKey) > 0 {
		req.Header.Set("Authorization", string(hostKey))
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving remote configs")
		return
	}
	defer resp.Body.Close()

	var remoteConfigs map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&remoteConfigs); err != nil {
		log.Error().Err(err).Msg("Error decoding remote configs")
		return
	}

	log.Debug().Int("count", len(remoteConfigs)).Msg("Retrieved remote config file(s)")

	// Write remote configs to local files
	for filename, content := range remoteConfigs {
		localPath := filepath.Join(configDir, filename)
		if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
			log.Error().Str("filename", filename).Err(err).Msg("Error writing config file")
			return
		}
	}

	// Read local config files
	localConfigs := make(map[string]string)
	files, err := os.ReadDir(configDir)
	if err != nil {
		log.Error().Err(err).Msg("Error reading config directory")
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			data, err := os.ReadFile(filepath.Join(configDir, file.Name()))
			if err != nil {
				log.Error().Str("filename", file.Name()).Err(err).Msg("Error reading config file")
				return
			}
			localConfigs[file.Name()] = string(data)
		}
	}

	// POST local configs to server
	payload, err := json.Marshal(localConfigs)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling config data")
		return
	}

	postReq, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		log.Error().Err(err).Msg("Error creating POST request")
		return
	}

	postReq.Header.Set("Content-Type", "application/json")
	if len(hostKey) > 0 {
		postReq.Header.Set("Authorization", string(hostKey))
	}

	postResp, err := client.Do(postReq)
	if err != nil {
		log.Error().Err(err).Msg("Error sending local configs")
		return
	}
	defer postResp.Body.Close()

	// Read response body to parse success/error message
	bodyBytes, err := io.ReadAll(postResp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Error reading response body")
		return
	}

	// Check for successful status codes OR check if the response contains success indicators
	if postResp.StatusCode != http.StatusOK && postResp.StatusCode != http.StatusCreated {
		// Try to parse the response as JSON
		var respData map[string]string
		if err := json.Unmarshal(bodyBytes, &respData); err == nil {
			// If status is "created" or message indicates success, don't treat as error
			if respData["status"] == "created" || strings.Contains(respData["message"], "updated") {
				fmt.Println("Configuration synchronized successfully")
				return
			}
		}
		log.Error().Str("body", string(bodyBytes)).Msg("Server error during sync")
		return
	}

	fmt.Println("Configuration synchronized successfully")
}

func authDelete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", ClientConf.URL+"/api/v1/auth/"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	addAuthHeader(req)
	client := ClientConf.hc()
	return client.Do(req)
}

// Add this new function to handle generic requests
func SendGenericRequest(method, path string, data []byte) (*http.Response, error) {
	fullURL := ClientConf.URL + path
	req, err := http.NewRequest(method, fullURL, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	addAuthHeader(req)
	if len(data) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	client := ClientConf.hc()
	return client.Do(req)
}

// Add this new command handler
func RequestCmd(cmd *cobra.Command, args []string) {
	ClientInit()

	method, _ := cmd.Flags().GetString("X")
	if method == "" {
		method = "GET" // Default to GET if no method specified
	}

	if len(args) == 0 {
		fmt.Println("error: path required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	path := args[0]
	data, _ := cmd.Flags().GetString("data")

	var requestData []byte
	if data != "" {
		requestData = []byte(data)
	}

	resp, err := SendGenericRequest(method, path, requestData)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("error reading response: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}

	// Try to pretty print if it's JSON
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
		fmt.Println(prettyJSON.String())
	} else {
		// If it's not JSON, print as plain text
		fmt.Println(string(body))
	}

	if resp.StatusCode >= 400 {
		common.RemoveLockfile()
		os.Exit(1)
	}
}

func AdminInventoryDelete(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: inventory name required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	inventoryName := args[0]
	resp, err := SendGenericRequest("DELETE", "/api/v1/inventory/"+inventoryName, nil)
	if err != nil {
		if err.Error() == "not admin" {
			fmt.Println("error: admin access required")
		} else {
			fmt.Printf("error: %v\n", err)
		}
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		fmt.Printf("error decoding response: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Println(response["message"])
}

func AdminInventoryList(cmd *cobra.Command, args []string) {
	ClientInit()

	resp, err := SendGenericRequest("GET", "/api/v1/inventory", nil)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("error reading response: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		if err := json.Unmarshal(body, &errorResp); err != nil {
			fmt.Printf("error: %s\n", string(body))
		} else {
			fmt.Printf("error: %s\n", errorResp["error"])
		}
		common.RemoveLockfile()
		os.Exit(1)
	}

	var inventories []InventoryResponse
	if err := json.Unmarshal(body, &inventories); err != nil {
		fmt.Printf("error decoding response: %v\nresponse body: %s\n", err, string(body))
		common.RemoveLockfile()
		os.Exit(1)
	}

	// Print inventories in a formatted way
	for _, inv := range inventories {
		fmt.Printf("Inventory: %s\n", inv.Name)
		fmt.Printf("  Hosts: %d\n", len(inv.Hosts))
	}
}

func AdminInventoryCreate(cmd *cobra.Command, args []string) {
	ClientInit()

	if len(args) == 0 {
		fmt.Println("error: inventory name required")
		common.RemoveLockfile()
		os.Exit(1)
	}

	inventoryName := args[0]
	data := map[string]string{
		"name": inventoryName,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		common.RemoveLockfile()
		os.Exit(1)
	}

	resp, err := SendGenericRequest("POST", "/api/v1/inventory", jsonData)
	if err != nil {
		if err.Error() == "not admin" {
			fmt.Println("error: admin access required")
		} else {
			fmt.Printf("error: %v\n", err)
		}
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		fmt.Printf("error: %s\n", errorResp["error"])
		common.RemoveLockfile()
		os.Exit(1)
	}

	fmt.Printf("Inventory '%s' created successfully\n", inventoryName)
}

// LogsCmd handles the logs command and its subcommands
func LogsCmd(cmd *cobra.Command, args []string) {
	ClientInit()

	host, _ := cmd.Flags().GetString("host")
	level, _ := cmd.Flags().GetString("level")
	component, _ := cmd.Flags().GetString("component")
	message, _ := cmd.Flags().GetString("message")
	startTime, _ := cmd.Flags().GetString("startTime")
	endTime, _ := cmd.Flags().GetString("endTime")
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("pageSize")

	// Construct query string
	query := "/api/v1/logs?"
	if host != "" {
		query += "host=" + host + "&"
	}
	if level != "" {
		query += "level=" + level + "&"
	}
	if component != "" {
		query += "component=" + component + "&"
	}
	if message != "" {
		query += "message=" + message + "&"
	}
	if startTime != "" {
		query += "startTime=" + startTime + "&"
	}
	if endTime != "" {
		query += "endTime=" + endTime + "&"
	}
	query += fmt.Sprintf("page=%d&pageSize=%d", page, pageSize)

	// Send request to API
	resp, err := SendGenericRequest("GET", query, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch logs")
		common.RemoveLockfile()
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read response")
		common.RemoveLockfile()
		os.Exit(1)
	}

	// Check if response is successful
	if resp.StatusCode != http.StatusOK {
		// Try to parse error as JSON
		var errorResp map[string]string
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp["error"] != "" {
			log.Error().Str("error", errorResp["error"]).Msg("API error")
		} else {
			// If not JSON, print as plain text
			log.Error().Str("error", string(body)).Msg("API error")
		}
		common.RemoveLockfile()
		os.Exit(1)
	}

	// Try to parse as JSON and pretty print
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
		fmt.Println(prettyJSON.String())
	} else {
		// If not valid JSON, print as plain text
		fmt.Println(string(body))
		return
	}

	fmt.Println(prettyJSON.String())
}
