//go:build with_api

package host

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
)

// GetServiceStatus performs the API call to check service status, decoupled for testability.
func (s *HostService) GetServiceStatus(serviceName string) (bool, string, error) {
	apiVersion := "1"
	url := fmt.Sprintf("%s/api/v%s/hosts/%s/%s", s.Conf.URL, apiVersion, s.Conf.Identifier, serviceName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return true, "", err
	}

	keyPath := filepath.Join(s.Conf.APIKeyDir, s.Conf.Identifier)
	if hostKey, err := s.FS.ReadFile(keyPath); err == nil {
		req.Header.Set("Authorization", string(hostKey))
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return true, "", err
	}
	defer resp.Body.Close()

	var serviceStatus map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&serviceStatus)

	wantsUpdateTo := ""
	if v, ok := serviceStatus["wantsUpdateTo"].(string); ok {
		wantsUpdateTo = v
	}

	if status, ok := serviceStatus["status"].(string); ok {
		return status == "enabled", wantsUpdateTo, nil
	}
	if v, ok := serviceStatus["disabled"].(bool); ok {
		return !v, wantsUpdateTo, nil
	}
	return true, wantsUpdateTo, nil
}
