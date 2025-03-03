package common

import (
	"encoding/json"
	"fmt"
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
