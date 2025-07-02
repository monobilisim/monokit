//go:build linux

package vaultHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/monobilisim/monokit/common/health"
	"github.com/spf13/cobra"
)

func init() {
	common.RegisterComponent(common.Component{
		Name:       "vaultHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectVault,
	})
	health.Register(&VaultHealthProvider{})
}

var VaultHealthConfig VaultConfig

// collectVaultHealthData collects all Vault health information and returns it
func collectVaultHealthData() (*VaultHealthData, error) {
	version := "1.0.0"

	// Initialize config if not already done
	if common.ConfExists("vault") {
		common.LogDebug("vault config file exists, loading configuration...")

		// Load config directly into the nested config structure
		common.ConfInit("vault", &VaultHealthConfig)

		common.LogDebug(fmt.Sprintf("Loaded vault config - Address: %s, TLS.Verify: %t, Alerts.SealedVault: %t, Alerts.VersionUpdates: %t",
			VaultHealthConfig.Vault.Address, VaultHealthConfig.Vault.Tls.Verify, VaultHealthConfig.Vault.Alerts.Sealed_vault, VaultHealthConfig.Vault.Alerts.Version_updates))

		// Debug token loading (without exposing the actual token)
		if VaultHealthConfig.Vault.Token != "" {
			if strings.HasPrefix(VaultHealthConfig.Vault.Token, "${") {
				common.LogDebug("Token appears to be unexpanded environment variable")
			} else {
				common.LogDebug("Token loaded successfully from configuration")
			}
		} else {
			common.LogDebug("No token configured")
		}
	} else {
		common.LogDebug("vault config file not found")
	}

	// Set defaults if not configured
	if VaultHealthConfig.Vault.Address == "" {
		VaultHealthConfig.Vault.Address = "https://127.0.0.1:8200"
		common.LogDebug("No address configured, using default: https://127.0.0.1:8200")
	} else {
		common.LogDebug(fmt.Sprintf("Using configured address: %s", VaultHealthConfig.Vault.Address))
	}

	// Create health data
	healthData := &VaultHealthData{
		Version:     version,
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Check service status
	healthData.Service.Installed = isVaultInstalled()
	healthData.Service.Active = common.SystemdUnitActive("vault.service")
	if healthData.Service.Active {
		healthData.Service.Status = "Running"
		common.AlarmCheckUp("vault_service", "Vault service is now running", false)
	} else {
		healthData.Service.Status = "Not Running"
		common.AlarmCheckDown("vault_service", "Vault service is not running", false, "", "")
	}

	// Only proceed with API checks if service is running
	if !healthData.Service.Active {
		return healthData, nil
	}

	// Check API connectivity and gather health information
	if err := checkVaultAPI(healthData); err != nil {
		common.LogError(fmt.Sprintf("Vault API check failed: %v", err))
		healthData.Connection.Connected = false
		healthData.Connection.Error = err.Error()
		common.AlarmCheckDown("vault_api", "Vault API is not accessible: "+err.Error(), false, "", "")
		return healthData, nil
	}

	common.AlarmCheckUp("vault_api", "Vault API is now accessible", false)

	// Check seal status
	if err := checkSealStatus(healthData); err != nil {
		common.LogError(fmt.Sprintf("Vault seal status check failed: %v", err))
	}

	// Check cluster status
	if err := checkClusterStatus(healthData); err != nil {
		common.LogError(fmt.Sprintf("Vault cluster status check failed: %v", err))
	}

	// Check replication status (Enterprise)
	if err := checkReplicationStatus(healthData); err != nil {
		common.LogDebug(fmt.Sprintf("Vault replication status check failed (might be CE): %v", err))
	}

	// Check for version updates
	if err := checkVaultVersionUpdates(healthData); err != nil {
		common.LogError(fmt.Sprintf("Vault version update check failed: %v", err))
	}

	return healthData, nil
}

// Main function for CLI usage
func Main(cmd *cobra.Command, args []string) {
	common.ScriptName = "vaultHealth"
	common.TmpDir = common.TmpDir + "vaultHealth"
	common.Init()

	api.WrapperGetServiceStatus("vaultHealth")

	// Collect health data using the shared function
	healthData, err := collectVaultHealthData()
	if err != nil {
		common.LogError("Failed to collect Vault health data: " + err.Error())
		return
	}

	// Attempt to POST health data to the Monokit server
	if err := common.PostHostHealth("vaultHealth", healthData); err != nil {
		common.LogError(fmt.Sprintf("vaultHealth: failed to POST health data: %v", err))
		// Continue execution even if POST fails, e.g., to display UI locally
	}

	fmt.Println(RenderVaultHealthCLI(healthData, healthData.Version))
}
