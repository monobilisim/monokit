//go:build with_api

package host

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/monobilisim/monokit/common/api/models"
)

// Type aliases for commonly used types from models package
type (
	Host = models.Host
)

// GetHosts retrieves a list of hosts or a single host by name (decoupled for testability).
func (s *HostService) GetHosts(apiVersion string, hostName string) ([]Host, error) {
	if hostName != "" {
		url := fmt.Sprintf("%s/api/v%s/hosts/%s", s.Conf.URL, apiVersion, hostName)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var host Host
		json.NewDecoder(resp.Body).Decode(&host)
		return []Host{host}, nil
	} else {
		url := fmt.Sprintf("%s/api/v%s/hosts", s.Conf.URL, apiVersion)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var hosts []Host
		json.NewDecoder(resp.Body).Decode(&hosts)
		return hosts, nil
	}
}
