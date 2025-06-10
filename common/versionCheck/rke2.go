package common

import (
	"fmt"
	"os"
)

func RKE2VersionCheck() {
	// Check if RKE2 is installed by looking for RKE2 specific paths
	if !isRKE2Environment() {
		fmt.Println("RKE2 environment not detected, skipping RKE2 version check.")
		return
	}

	// Get cluster name using detection options
	clusterName := GetClusterName("")
	if clusterName == "" {
		fmt.Println("Could not determine cluster name for RKE2 version check.")
		return
	}

	// Get RKE2/K8s version
	version, err := getRKE2Version()
	if err != nil {
		fmt.Printf("Error getting RKE2 version: %v\n", err)
		return
	}

	fmt.Printf("RKE2 cluster '%s' version: %s\n", clusterName, version)

	// Check stored version
	oldVersion := GatherVersion("rke2-" + clusterName)

	if oldVersion != "" && oldVersion == version {
		fmt.Printf("RKE2 cluster '%s' version is unchanged.\n", clusterName)
		return
	} else if oldVersion != "" && oldVersion != version {
		fmt.Printf("RKE2 cluster '%s' version has been updated.\n", clusterName)
		fmt.Printf("Old version: %s\n", oldVersion)
		fmt.Printf("New version: %s\n", version)

		// Create news only if this is a master node
		if IsMasterNodeViaAPI() {
			createK8sVersionNews(clusterName, "RKE2", oldVersion, version)
		} else {
			fmt.Println("Skipping news creation (not a master node)")
		}
	} else {
		fmt.Printf("First time detecting RKE2 cluster '%s' version.\n", clusterName)
	}

	// Store the new version
	StoreVersion("rke2-"+clusterName, version)
}

// isRKE2Environment checks if we're running in an RKE2 environment
func isRKE2Environment() bool {
	// Check for RKE2 specific paths/files
	rke2Paths := []string{
		"/var/lib/rancher/rke2",
		"/etc/rancher/rke2",
		"/var/lib/rancher/rke2/server/manifests",
	}

	for _, path := range rke2Paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// getRKE2Version gets the RKE2/Kubernetes version
func getRKE2Version() (string, error) {
	// Use common Kubernetes version detection
	versionInfo, err := GetKubernetesServerVersion()
	if err != nil {
		return "", err
	}

	// Return GitVersion which contains the full RKE2 version (e.g., "v1.28.5+rke2r1")
	return versionInfo.GitVersion, nil
}
