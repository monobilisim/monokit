package common

import (
	"fmt"
	"os/exec"
	"strings"
)

func ProxmoxVECheck() {
	// Check if Proxmox VE is installed by checking the existence of command "pveversion"
	_, err := exec.LookPath("pveversion")
	if err != nil {
		fmt.Println("Proxmox VE is not installed on this system.")
		return
	}

	// Get the version of Proxmox VE
	out, err := exec.Command("pveversion").Output()
	if err != nil {
		fmt.Println("Error getting Proxmox VE version.")
		return
	}

	// Parse the version
	// Eg. output
	// pve-manager/6.4-13/1c2b3f0e (running kernel: 5.4.78-2-pve)
	version := strings.Split(string(out), "/")[1]
	fmt.Println("Proxmox VE version:", version)

	oldVersion := GatherVersion("pve")

	if oldVersion != "" && oldVersion == version {
		fmt.Println("Proxmox VE is not updated.")
		return
	} else if oldVersion != "" && oldVersion != version {
		fmt.Println("Proxmox VE has been updated.")
		fmt.Println("Old version:", oldVersion)
		fmt.Println("New version:", version)
		CreateNews("Proxmox VE", oldVersion, version, false)
	}

	StoreVersion("pve", version)
}

func ProxmoxMGCheck() {
	// Check if Proxmox Mail Gateway is installed by checking the existence of command "pmgversion"
	_, err := exec.LookPath("pmgversion")
	if err != nil {
		fmt.Println("Proxmox Mail Gateway is not installed on this system.")
		return
	}

	// Get the version of Proxmox Mail Gateway
	out, err := exec.Command("pmgversion").Output()
	if err != nil {
		fmt.Println("Error getting Proxmox Mail Gateway version.")
		return
	}

	// Parse the version
	// Eg. output
	// pmg/6.4-13/1c2b3f0e (running kernel: 5.4.78-2-pve)
	version := strings.Split(string(out), "/")[1]
	fmt.Println("Proxmox Mail Gateway version:", version)

	oldVersion := GatherVersion("pmg")

	if oldVersion != "" && oldVersion == version {
		fmt.Println("Proxmox Mail Gateway is not updated.")
		return
	} else if oldVersion != "" && oldVersion != version {
		fmt.Println("Proxmox Mail Gateway has been updated.")
		fmt.Println("Old version:", oldVersion)
		fmt.Println("New version:", version)
		CreateNews("Proxmox Mail Gateway", oldVersion, version, false)
	}

	StoreVersion("pmg", version)
}

func ProxmoxBSCheck() {
	// Check if Proxmox Backup Server is installed by checking the existence of command "proxmox-backup-manager"
	_, err := exec.LookPath("proxmox-backup-manager")
	if err != nil {
		fmt.Println("Proxmox Backup Server is not installed on this system.")
		return
	}

	// Get the version of Proxmox Backup Server
	out, err := exec.Command("proxmox-backup-manager", "version").Output()
	if err != nil {
		fmt.Println("Error getting Proxmox Backup Server version.")
		return
	}

	// Parse the version
	// Eg. output
	// proxmox-backup-server 3.3.2-1 running version: 3.3.2
	version := strings.Split(string(out), " ")[1]
	fmt.Println("Proxmox Backup Server version:", version)

	oldVersion := GatherVersion("pbs")

	if oldVersion != "" && oldVersion == version {
		fmt.Println("Proxmox Backup Server is not updated.")
		return
	} else if oldVersion != "" && oldVersion != version {
		fmt.Println("Proxmox Backup Server has been updated.")
		fmt.Println("Old version:", oldVersion)
		fmt.Println("New version:", version)
		CreateNews("Proxmox Backup Server", oldVersion, version, false)
	}

	StoreVersion("pbs", version)
}
