//go:build linux

package pgsqlHealth

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
)

type HAProxyHealthData struct {
	Enabled bool
	Status  string
	Active  bool
}

const haproxyStatsPort = 1936 // Common default HAProxy stats port

func checkHAProxyService() (installed bool, running bool) {
	installed = common.SystemdUnitExists("haproxy.service")
	if !installed {
		common.AlarmCheckDown("haproxy_service_installed", "HAProxy service is not installed", false, "", "")
	} else {
		common.AlarmCheckUp("haproxy_service_installed", "HAProxy service is now installed", false)
	}

	if installed {
		running = common.SystemdUnitActive("haproxy.service")
		if !running {
			common.AlarmCheckDown("haproxy_service_running", "HAProxy service is not running", false, "", "")
		} else {
			common.AlarmCheckUp("haproxy_service_running", "HAProxy service is now running", false)
		}
	}
	return installed, running
}

func getHAProxyBindPorts(filePath string) ([]int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open haproxy config file %s: %w", filePath, err)
	}
	defer file.Close()

	var ports []int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "bind ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				address := parts[1]
				// Address can be like :80, 0.0.0.0:80, *:80, 127.0.0.1:80
				var portStr string
				if strings.Contains(address, ":") {
					addrParts := strings.Split(address, ":")
					portStr = addrParts[len(addrParts)-1] // Last part is the port
				} else {
					// If no colon, it might be just the port (though less common for bind)
					// Or an invalid format, we'll try to parse anyway
					portStr = address
				}

				if port, err := strconv.Atoi(portStr); err == nil {
					ports = append(ports, port)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading haproxy config file %s: %w", filePath, err)
	}

	if len(ports) == 0 {
		return nil, fmt.Errorf("no bind ports found in %s", filePath)
	}

	return ports, nil
}

func checkHAProxyBindPortsOpen() bool {
	ports, err := getHAProxyBindPorts("/etc/haproxy/haproxy.cfg")
	if err != nil {
		common.AlarmCheckDown("haproxy_config_read", fmt.Sprintf("Failed to read or parse HAProxy bind ports: %s", err.Error()), false, "", "")
		return false
	}
	common.AlarmCheckUp("haproxy_config_read", "Successfully read HAProxy config for bind ports.", false)

	var onePortIsOpen bool
	for _, port := range ports {
		address := fmt.Sprintf("localhost:%d", port)
		conn, err := net.DialTimeout("tcp", address, 1*time.Second) // Shorter timeout per port
		if err == nil {
			_ = conn.Close()
			common.AlarmCheckUp("haproxy_bind_port_accessible", fmt.Sprintf("HAProxy bind port %d is accessible", port), false)
			onePortIsOpen = true
			// If we find one open port, we can consider it active for this check's purpose
			// Or, we could require all configured ports to be open. For now, any is fine.
			break
		} else {
			common.AlarmCheckDown("haproxy_bind_port_accessible", fmt.Sprintf("HAProxy bind port %d is not accessible", port), false, "", "")
		}
	}

	if !onePortIsOpen && len(ports) > 0 {
		common.AlarmCheckDown("haproxy_all_bind_ports_inaccessible", "No configured HAProxy bind ports are accessible", false, "", "")
		return false
	}
	if onePortIsOpen {
		// Clear the "all inaccessible" alarm if it was previously set
		common.AlarmCheckUp("haproxy_all_bind_ports_inaccessible", "At least one HAProxy bind port is now accessible", false)
	}

	return onePortIsOpen
}

func checkSpecificHAProxyPort(portToCheck int, portIdentifier string) bool {
	address := fmt.Sprintf("localhost:%d", portToCheck)
	alarmName := fmt.Sprintf("haproxy_port_%s_accessible", portIdentifier)

	conn, err := net.DialTimeout("tcp", address, 1*time.Second)
	if err != nil {
		common.AlarmCheckDown(alarmName, fmt.Sprintf("HAProxy specific port %s (%d) is not accessible", portIdentifier, portToCheck), false, "", "")
		return false
	}
	_ = conn.Close()
	common.AlarmCheckUp(alarmName, fmt.Sprintf("HAProxy specific port %s (%d) is now accessible", portIdentifier, portToCheck), false)
	return true
}

func GetHAProxyHealth() HAProxyHealthData {
	serviceInstalled, serviceRunning := checkHAProxyService()

	if !serviceInstalled {
		return HAProxyHealthData{
			Enabled: false,
			Status:  "HAProxy service not installed",
			Active:  false,
		}
	}

	if !serviceRunning {
		return HAProxyHealthData{
			Enabled: true, // Installed, but not running
			Status:  "HAProxy service not running",
			Active:  false,
		}
	}

	// If service is installed and running, check the configured bind ports
	bindPortsOpen := checkHAProxyBindPortsOpen()
	if !bindPortsOpen {
		return HAProxyHealthData{
			Enabled: true,
			Status:  "HAProxy service running, but no configured bind ports are accessible",
			Active:  false,
		}
	}

	return HAProxyHealthData{
		Enabled: true,
		Status:  "HAProxy service is running and at least one configured bind port is accessible",
		Active:  true,
	}
}
