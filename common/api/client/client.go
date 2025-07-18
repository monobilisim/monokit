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

func Update(cmd *cobra.Command, args []string) {
	apiVersion := ClientInit()

	SendReq(apiVersion)

	// Ensure lockfile is removed after update
	common.RemoveLockfile()
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

	return apiVersion
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
