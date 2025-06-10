package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

var ClientURL string

func GetServiceStatus(serviceName string) (bool, string) {
	apiVersion := "1"

	req, err := http.NewRequest("GET", ClientURL+"/api/v"+apiVersion+"/hosts/"+Config.Identifier+"/"+serviceName, nil)

	if err != nil {
		LogError(err.Error())
		return true, ""
	}

	// Add host key if available
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", Config.Identifier)
	if hostKey, err := os.ReadFile(keyPath); err == nil {
		req.Header.Set("Authorization", string(hostKey))
	}

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		LogError(err.Error())
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
	if !ConfExists("client") {
		return
	}

	type ClientConfig struct {
		URL string
	}
	var clientConf ClientConfig

	ConfInit("client", &clientConf)
	ClientURL = clientConf.URL

	if ClientURL == "" {
		return
	}

	status, updateVersion := GetServiceStatus(serviceName)

	if !status {
		fmt.Println(serviceName + " is disabled. Exiting...")
		// Remove lockfile
		RemoveLockfile()
		os.Exit(0)
	}

	if updateVersion != MonokitVersion && updateVersion != "" {
		fmt.Println(serviceName + " wants to be updated to " + updateVersion)
		Update(updateVersion, false)

		// Re-run sendReq after the update
		// Note: SendReq is removed as it's not needed in the common package
	}
}

// PostHostHealth sends collected health JSON data to the Monokit server.
// It relies on ClientURL and Config.Identifier being previously initialized (e.g., by WrapperGetServiceStatus or similar).
func PostHostHealth(toolName string, payload interface{}) error {
	if ClientURL == "" {
		return fmt.Errorf("monokit client URL not configured")
	}
	if Config.Identifier == "" {
		return fmt.Errorf("monokit client identifier (hostname) not configured")
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal health data for tool %s: %w", toolName, err)
	}

	url := fmt.Sprintf("%s/api/v1/host/health/%s", ClientURL, toolName)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create POST request for %s to %s: %w", toolName, url, err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Fetch and set the host token, similar to GetServiceStatus but using X-Host-Token
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", Config.Identifier)
	hostKey, err := os.ReadFile(keyPath)
	if err != nil {
		// Log this, but proceed? Or fail?
		// For now, let's make it a hard requirement for posting health.
		// If the agent is running, it should have its key.
		LogError(fmt.Sprintf("PostHostHealth: Failed to read host key from %s: %v. Health data for %s will not be sent.", keyPath, err, toolName))
		return fmt.Errorf("failed to read host key from %s: %w. Cannot authenticate health POST", keyPath, err)
	}
	// The server expects the host token in the "Authorization" header for routes protected by hostAuthMiddleware.
	req.Header.Set("Authorization", string(hostKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to POST health data for tool %s to %s: %w", toolName, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("health data POST for %s to %s failed with status %d, and failed to read response body: %w", toolName, url, resp.StatusCode, readErr)
		}
		return fmt.Errorf("health data POST for %s to %s failed with status %d: %s", toolName, url, resp.StatusCode, string(bodyBytes))
	}

	return nil
}
