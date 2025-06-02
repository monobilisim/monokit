//go:build with_api

package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/monobilisim/monokit/common/api/clientport"
)

// Config represents host-specific configuration for API interaction.
type Config struct {
	URL        string
	Identifier string
	Version    string
	APIKeyDir  string
}

// HostService provides methods for host registration and service status checks.
type HostService struct {
	HTTP clientport.HTTPDoer
	FS   clientport.FS
	Info clientport.SysInfo
	Exit clientport.Exiter
	Conf *Config
}

// SendHostReport uploads or updates this host's registration/status to the server.
func (s *HostService) SendHostReport() error {
	hostObj := Host{
		Name:                s.Conf.Identifier,
		CpuCores:            s.Info.CPUCores(),
		Ram:                 s.Info.RAM(),
		MonokitVersion:      s.Conf.Version,
		Os:                  s.Info.OSPlatform(),
		DisabledComponents:  "nil",
		InstalledComponents: "", // fill from specific injector if needed
		IpAddress:           s.Info.PrimaryIP(),
		Status:              "Online",
		WantsUpdateTo:       "",
		Groups:              "nil",
		Inventory:           "", // parse from identifier if needed
	}
	hostJSON, err := json.Marshal(hostObj)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", s.Conf.URL+"/api/v1/hosts", bytes.NewBuffer(hostJSON))
	if err != nil {
		return err
	}

	keyPath := filepath.Join(s.Conf.APIKeyDir, s.Conf.Identifier)
	authKey, _ := s.FS.ReadFile(keyPath)
	if len(authKey) > 0 {
		req.Header.Set("Authorization", string(authKey))
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Host   *Host  `json:"host"`
		ApiKey string `json:"apiKey,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode: %w\nbody: %s", err, string(body))
	}
	if result.Error != "" {
		return fmt.Errorf("server error: %s", result.Error)
	}
	if result.ApiKey != "" {
		// Create API key file
		if err := s.FS.MkdirAll(s.Conf.APIKeyDir, 0755); err != nil {
			return err
		}
		if err := s.FS.WriteFile(keyPath, []byte(result.ApiKey), 0600); err != nil {
			return err
		}
	}
	if result.Host != nil && result.Host.UpForDeletion {
		s.Exit.Exit(0)
	}
	return nil
}

// GetHosts retrieves a list of hosts or a single host by name.
