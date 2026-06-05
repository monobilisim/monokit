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
		addToNotInstalled("Proxmox VE")
		return
	}

	// Get the version of Proxmox VE
	out, err := exec.Command("pveversion").Output()
	if err != nil {
		addToVersionErrors(fmt.Errorf("Error getting Proxmox VE version"))
		return
	}

	// Parse the version
	// Eg. output
	// pve-manager/6.4-13/1c2b3f0e (running kernel: 5.4.78-2-pve)
	version := strings.Split(string(out), "/")[1]

	oldVersion := GatherVersion("pve")

	if oldVersion != "" && oldVersion != version {
		addToUpdated(AppVersion{Name: "Proxmox VE", OldVersion: oldVersion, NewVersion: version})
		CreateNews("Proxmox VE", oldVersion, version, false)
	} else {
		addToNotUpdated(AppVersion{Name: "Proxmox VE", OldVersion: version})
	}

	StoreVersion("pve", version)
}

func ProxmoxMGCheck() {
	// Check if Proxmox Mail Gateway is installed by checking the existence of command "pmgversion"
	_, err := exec.LookPath("pmgversion")
	if err != nil {
		addToNotInstalled("Proxmox Mail Gateway")
		return
	}

	// Get the version of Proxmox Mail Gateway
	out, err := exec.Command("pmgversion").Output()
	if err != nil {
		addToVersionErrors(fmt.Errorf("Error getting Proxmox Mail Gateway version"))
		return
	}

	// Parse the version
	// Eg. output
	// pmg/6.4-13/1c2b3f0e (running kernel: 5.4.78-2-pve)
	version := strings.Split(string(out), "/")[1]

	oldVersion := GatherVersion("pmg")

	if oldVersion != "" && oldVersion != version {
		addToUpdated(AppVersion{Name: "Proxmox Mail Gateway", OldVersion: oldVersion, NewVersion: version})
		CreateNews("Proxmox Mail Gateway", oldVersion, version, false)
	} else {
		addToNotUpdated(AppVersion{Name: "Proxmox Mail Gateway", OldVersion: version})
	}

	StoreVersion("pmg", version)
}

func ProxmoxBSCheck() {
	// Check if Proxmox Backup Server is installed by checking the existence of command "proxmox-backup-manager"
	_, err := exec.LookPath("proxmox-backup-manager")
	if err != nil {
		addToNotInstalled("Proxmox Backup Server")
		return
	}

	// Get the version of Proxmox Backup Server
	out, err := exec.Command("proxmox-backup-manager", "version").Output()
	if err != nil {
		addToVersionErrors(fmt.Errorf("Error getting Proxmox Backup Server version"))
		return
	}

	// Parse the version
	// Eg. output
	// proxmox-backup-server 3.3.2-1 running version: 3.3.2
	version := strings.Split(string(out), " ")[1]

	oldVersion := GatherVersion("pbs")

	if oldVersion != "" && oldVersion != version {
		addToUpdated(AppVersion{Name: "Proxmox Backup Server", OldVersion: oldVersion, NewVersion: version})
		CreateNews("Proxmox Backup Server", oldVersion, version, false)
	} else {
		addToNotUpdated(AppVersion{Name: "Proxmox Backup Server", OldVersion: version})
	}

	StoreVersion("pbs", version)
}
