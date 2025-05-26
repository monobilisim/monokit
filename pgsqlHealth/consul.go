//go:build linux

package pgsqlHealth

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/monobilisim/monokit/common"
)

var consulURL string = "http://localhost:8500"

func checkConsulService() (bool, error) {
	consulServiceExists := common.SystemdUnitExists("consul.service")
	consulServiceRunning := common.SystemdUnitActive("consul.service")

	if !consulServiceExists {
		common.AlarmCheckDown("consul_service", "Consul service is not installed", false, "", "")
	} else {
		common.AlarmCheckUp("consul_service", "Consul service is now installed", false)
	}

	if consulServiceExists && !consulServiceRunning {
		common.AlarmCheckDown("consul_service", "Consul service is not running", false, "", "")
	} else {
		common.AlarmCheckUp("consul_service", "Consul service is now running", false)
	}

	return consulServiceExists && consulServiceRunning, nil
}

func checkConsulPorts() (bool, error) {
	// Check ports 8500 and 8600 using net.DialTimeout
	conn, err := net.DialTimeout("tcp", "localhost:8500", 5*time.Second)
	port8500 := err == nil
	_ = conn.Close()

	conn, err = net.DialTimeout("tcp", "localhost:8600", 5*time.Second)
	port8600 := err == nil
	_ = conn.Close()

	if !port8500 {
		common.AlarmCheckDown("consul_port_8500", "Consul port 8500 is not open", false, "", "")
	} else {
		common.AlarmCheckUp("consul_port_8500", "Consul port 8500 is now open", false)
	}

	if !port8600 {
		common.AlarmCheckDown("consul_port_8600", "Consul port 8600 is not open", false, "", "")
	} else {
		common.AlarmCheckUp("consul_port_8600", "Consul port 8600 is now open", false)
	}

	return port8500 && port8600, nil
}

func getConsulCatalog(consulURL string) (*ConsulCatalog, error) {
	// Get all services
	url := fmt.Sprintf("%s/v1/catalog/services", consulURL)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse service list
	var services map[string][]string
	err = json.Unmarshal(body, &services)
	if err != nil {
		return nil, err
	}

	// Get details for each service
	var catalogServices []Service
	seen := make(map[string]bool) // Track unique service names

	for serviceName := range services {
		// Skip if we've seen this service name before
		if seen[serviceName] {
			continue
		}
		seen[serviceName] = true

		url = fmt.Sprintf("%s/v1/catalog/service/%s", consulURL, serviceName)
		resp, err = http.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		var serviceInstances []struct {
			Node           string   `json:"Node"`
			Address        string   `json:"Address"`
			ServiceID      string   `json:"ServiceID"`
			ServiceName    string   `json:"ServiceName"`
			ServiceTags    []string `json:"ServiceTags"`
			ServiceAddress string   `json:"ServiceAddress"`
			ServicePort    int      `json:"ServicePort"`
		}
		err = json.Unmarshal(body, &serviceInstances)
		if err != nil {
			continue
		}

		// Only take the first instance of each service
		if len(serviceInstances) > 0 {
			si := serviceInstances[0]
			// Use Node address if ServiceAddress is empty
			serviceAddr := si.ServiceAddress
			if serviceAddr == "" {
				serviceAddr = si.Address
			}

			service := Service{
				Name:   si.ServiceName,
				ID:     si.ServiceID,
				State:  "running", // Default state
				Host:   serviceAddr,
				Port:   int64(si.ServicePort),
				APIURL: fmt.Sprintf("http://%s:%d", serviceAddr, si.ServicePort),
			}

			// Check for non-consul services
			if service.Name != "consul" {
				common.AlarmCheckDown("consul_service_"+service.Name, fmt.Sprintf("Unexpected consul service found: %s", service.Name), false, "", "")
			}

			catalogServices = append(catalogServices, service)
		}
	}

	catalog := &ConsulCatalog{
		ServiceName: "all",
		Services:    catalogServices,
	}

	return catalog, nil
}

func getConsulMembers(consulURL string) ([]ConsulMember, error) {
	// First get the members
	url := fmt.Sprintf("%s/v1/agent/members", consulURL)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result []struct {
		Name string            `json:"Name"`
		Addr string            `json:"Addr"`
		Tags map[string]string `json:"Tags"`
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	members := make([]ConsulMember, len(result))
	for i, item := range result {
		// Get health status for this node
		healthURL := fmt.Sprintf("%s/v1/health/node/%s", consulURL, item.Name)
		healthResp, err := http.Get(healthURL)
		if err != nil {
			members[i] = ConsulMember{
				Node:    item.Name,
				Address: item.Addr,
				Status:  "unknown",
			}
			continue
		}
		defer healthResp.Body.Close()

		healthBody, err := io.ReadAll(healthResp.Body)
		if err != nil {
			members[i] = ConsulMember{
				Node:    item.Name,
				Address: item.Addr,
				Status:  "unknown",
			}
			continue
		}

		var healthChecks []struct {
			Status string `json:"Status"`
		}
		err = json.Unmarshal(healthBody, &healthChecks)
		if err != nil || len(healthChecks) == 0 {
			members[i] = ConsulMember{
				Node:    item.Name,
				Address: item.Addr,
				Status:  "unknown",
			}
			continue
		}

		// Use the first health check status
		status := "passing"
		if healthChecks[0].Status != "passing" {
			status = healthChecks[0].Status
		}

		if status != "passing" {
			common.AlarmCheckDown("consul_member", fmt.Sprintf("Consul member %s is not passing, is status: %s", item.Name, status), false, "", "")
		} else {
			common.AlarmCheckUp("consul_member", fmt.Sprintf("Consul member %s is now passing", item.Name), false)
		}

		members[i] = ConsulMember{
			Node:    item.Name,
			Address: item.Addr,
			Status:  status,
		}
	}

	return members, nil
}
