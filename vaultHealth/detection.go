//go:build linux

package vaultHealth

import (
	"os/exec"

	"github.com/monobilisim/monokit/common"
)

// DetectVault checks if Vault is installed and available
func DetectVault() bool {
	// Check if vault binary exists in PATH
	_, err := exec.LookPath("vault")
	if err != nil {
		common.LogDebug("Vault detection failed: vault binary not found in PATH")
		return false
	}

	// Check if vault service unit file exists
	if !common.SystemdUnitExists("vault.service") {
		common.LogDebug("Vault detection failed: vault.service systemd unit not found")
		return false
	}

	common.LogDebug("Vault detected: binary and service unit found")
	return true
}

// isVaultInstalled checks if Vault binary is installed
func isVaultInstalled() bool {
	_, err := exec.LookPath("vault")
	return err == nil
}
