//go:build linux

package vaultHealth

import (
	"os/exec"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

// DetectVault checks if Vault is installed and available
func DetectVault() bool {
	// Check if vault binary exists in PATH
	_, err := exec.LookPath("vault")
	if err != nil {
		log.Debug().Msg("Vault detection failed: vault binary not found in PATH")
		return false
	}

	// Check if vault service unit file exists
	if !common.SystemdUnitExists("vault.service") {
		log.Debug().Msg("Vault detection failed: vault.service systemd unit not found")
		return false
	}

	log.Debug().Msg("Vault detected: binary and service unit found")
	return true
}

// isVaultInstalled checks if Vault binary is installed
func isVaultInstalled() bool {
	_, err := exec.LookPath("vault")
	return err == nil
}
