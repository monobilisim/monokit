package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

var ClientURL string

func GetServiceStatus(serviceName string) (bool, string) {
	apiVersion := "1"

	req, err := http.NewRequest("GET", ClientURL+"/api/v"+apiVersion+"/hosts/"+Config.Identifier+"/"+serviceName, nil)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create GET request")
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
		log.Error().Err(err).Msg("Failed to send GET request")
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
		Update(updateVersion, false, true, []string{}, "/var/lib/monokit/plugins")

		// Re-run sendReq after the update
		// Note: SendReq is removed as it's not needed in the common package
	}
}

// PostHostHealth sends collected health JSON data to the Monokit server.
// It relies on ClientURL and Config.Identifier being previously initialized (e.g., by WrapperGetServiceStatus or similar).
func PostHostHealth(toolName string, payload interface{}) error {
	if ClientURL == "" {
		return nil
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
		// Host key doesn't exist, attempt to register the host to obtain one
		log.Warn().Str("keyPath", keyPath).Err(err).Str("toolName", toolName).Msg("Host key not found, attempting host registration")

		// Try to register the host to get an API key
		if regErr := attemptHostRegistration(); regErr != nil {
			log.Error().Err(regErr).Str("toolName", toolName).Msg("Failed to register host and obtain API key")
			return fmt.Errorf("failed to read host key from %s: %w. Host registration also failed: %v. Cannot authenticate health POST", keyPath, err, regErr)
		}

		// Try to read the key again after registration
		hostKey, err = os.ReadFile(keyPath)
		if err != nil {
			log.Error().Str("keyPath", keyPath).Err(err).Str("toolName", toolName).Msg("Host key still not available after registration attempt")
			return fmt.Errorf("failed to read host key from %s even after registration attempt: %w. Cannot authenticate health POST", keyPath, err)
		}

		log.Info().Str("keyPath", keyPath).Str("toolName", toolName).Msg("Successfully obtained host key after registration")
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

// attemptHostRegistration tries to register the host with the API server to obtain an API key
func attemptHostRegistration() error {
	if ClientURL == "" {
		return fmt.Errorf("client URL not configured")
	}
	if Config.Identifier == "" {
		return fmt.Errorf("host identifier not configured")
	}

	// Create a basic host registration payload
	host := struct {
		Name           string `json:"name"`
		CpuCores       int    `json:"cpuCores"`
		Ram            string `json:"ram"`
		MonokitVersion string `json:"monokitVersion"`
		Os             string `json:"os"`
		IpAddress      string `json:"ipAddress"`
		Status         string `json:"status"`
	}{
		Name:           Config.Identifier,
		CpuCores:       0, // Will be filled by server if needed
		Ram:            "Unknown",
		MonokitVersion: MonokitVersion,
		Os:             "Unknown",
		IpAddress:      "Unknown",
		Status:         "Online",
	}

	hostJSON, err := json.Marshal(host)
	if err != nil {
		return fmt.Errorf("failed to marshal host data: %w", err)
	}

	// Send registration request
	url := ClientURL + "/api/v1/hosts"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(hostJSON))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("registration failed with status %d, and failed to read response body: %w", resp.StatusCode, readErr)
		}
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response to get API key
	var response struct {
		Host   interface{} `json:"host"`
		ApiKey string      `json:"apiKey,omitempty"`
		Error  string      `json:"error,omitempty"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read registration response: %w", err)
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to decode registration response: %w", err)
	}

	if response.Error != "" {
		return fmt.Errorf("server error during registration: %s", response.Error)
	}

	// Save the API key if received
	if response.ApiKey != "" {
		keyDir := "/var/lib/mono/api/hostkey"
		if err := os.MkdirAll(keyDir, 0755); err != nil {
			return fmt.Errorf("failed to create key directory: %w", err)
		}

		keyPath := filepath.Join(keyDir, Config.Identifier)
		if err := os.WriteFile(keyPath, []byte(response.ApiKey), 0600); err != nil {
			return fmt.Errorf("failed to write API key to file: %w", err)
		}

		log.Info().Str("keyPath", keyPath).Msg("Successfully saved API key from host registration")
		return nil
	}

	return fmt.Errorf("no API key received in registration response")
}
