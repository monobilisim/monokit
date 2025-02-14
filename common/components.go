package common

import (
	"fmt"
	"os/exec"
	"strings"
)

type Component struct {
	Name        string
	Command     string
	ConfigCheck bool
}

var Components = []Component{
	{"pritunl", "pritunl", false},
	{"postal", "postal", false},
	{"pmg", "pmgversion", false},
	{"k8s", "k8s", true},
	{"mysql", "mysqld", false},
	{"redis", "redis-server", false},
	{"rabbitmq", "rabbitmq-server", false},
	{"traefik", "traefik", false},
	{"wppconnect", "wppconnect", true},
}

func IsEnabled(name string) (bool, bool) {
	fmt.Printf("Checking component %s in config: %+v\n", name, Config.Components)
	if Config.Components == nil {
		fmt.Println("No components in config")
		return false, false
	}
	for _, hc := range Config.Components {
		fmt.Printf("Comparing with config entry: name=%s enabled=%v\n", hc.Name, hc.Enabled)
		if hc.Name == name {
			return true, hc.Enabled
		}
	}
	return false, false
}

func GetInstalledComponents() string {
	var installed []string

	for _, comp := range Components {
		if comp.Name == "mysql" {
			if CommExists("mysqld", comp.ConfigCheck) || CommExists("mariadbd", comp.ConfigCheck) {
				installed = append(installed, comp.Name)
			}
			continue
		}

		if CommExists(comp.Command, comp.ConfigCheck) {
			installed = append(installed, comp.Name)
		}
	}

	if len(installed) == 0 {
		return "nil"
	}
	return strings.Join(installed, "::")
}

func CommExists(command string, confCheckOnly bool) bool {
	// Find the component definition
	var component Component
	found := false
	for _, comp := range Components {
		if comp.Command == command {
			component = comp
			found = true
			break
		}
	}
	if !found {
		return false
	}

	// Check if it's enabled in config
	existsOnConfig, enabled := IsEnabled(component.Name)
	fmt.Printf("Component %s: config=%v enabled=%v\n", component.Name, existsOnConfig, enabled)

	if confCheckOnly || component.ConfigCheck {
		return existsOnConfig && enabled
	}

	// If not confCheckOnly, check both config and PATH
	path, _ := exec.LookPath(command)
	fmt.Printf("Component %s: path=%v\n", component.Name, path)
	return (existsOnConfig && enabled) || path != ""
}
