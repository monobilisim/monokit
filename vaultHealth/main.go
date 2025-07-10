//go:build linux

package vaultHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	"github.com/monobilisim/monokit/common/health"
	"github.com/rs/zerolog/log"
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
		log.Debug().Msg("vault config file exists, loading configuration...")

		// Load config directly into the nested config structure
		common.ConfInit("vault", &VaultHealthConfig)

		log.Debug().Str("address", VaultHealthConfig.Vault.Address).Bool("tls_verify", VaultHealthConfig.Vault.Tls.Verify).Bool("alerts_sealed_vault", VaultHealthConfig.Vault.Alerts.Sealed_vault).Bool("alerts_version_updates", VaultHealthConfig.Vault.Alerts.Version_updates).Msg("Loaded vault config")

		// Debug token loading (without exposing the actual token)
		if VaultHealthConfig.Vault.Token != "" {
			if strings.HasPrefix(VaultHealthConfig.Vault.Token, "${") {
				log.Debug().Msg("Token appears to be unexpanded environment variable")
			} else {
				log.Debug().Msg("Token loaded successfully from configuration")
			}
		} else {
			log.Debug().Msg("No token configured")
		}
	} else {
		log.Debug().Msg("vault config file not found")
	}

	// Set defaults if not configured
	if VaultHealthConfig.Vault.Address == "" {
		VaultHealthConfig.Vault.Address = "https://127.0.0.1:8200"
		log.Debug().Str("address", VaultHealthConfig.Vault.Address).Msg("No address configured, using default")
	} else {
		log.Debug().Str("address", VaultHealthConfig.Vault.Address).Msg("Using configured address")
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
		log.Error().Err(err).Msg("Vault API check failed")
		healthData.Connection.Connected = false
		healthData.Connection.Error = err.Error()
		common.AlarmCheckDown("vault_api", "Vault API is not accessible: "+err.Error(), false, "", "")
		return healthData, nil
	}

	common.AlarmCheckUp("vault_api", "Vault API is now accessible", false)

	// Check seal status
	if err := checkSealStatus(healthData); err != nil {
		log.Error().Err(err).Msg("Vault seal status check failed")
	}

	// Check cluster status
	if err := checkClusterStatus(healthData); err != nil {
		log.Error().Err(err).Msg("Vault cluster status check failed")
	}

	// Check replication status (Enterprise)
	if err := checkReplicationStatus(healthData); err != nil {
		log.Debug().Err(err).Msg("Vault replication status check failed (might be CE)")
	}

	// Check for version updates
	if err := checkVaultVersionUpdates(healthData); err != nil {
		log.Error().Err(err).Msg("Vault version update check failed")
	}

	return healthData, nil
}

// Main function for CLI usage
func Main(cmd *cobra.Command, args []string) {
	common.ScriptName = "vaultHealth"
	common.TmpDir = common.TmpDir + "vaultHealth"
	common.Init()

	client.WrapperGetServiceStatus("vaultHealth")

	// Collect health data using the shared function
	healthData, err := collectVaultHealthData()
	if err != nil {
		log.Error().Err(err).Msg("Failed to collect Vault health data")
		return
	}

	// Attempt to POST health data to the Monokit server
	if err := common.PostHostHealth("vaultHealth", healthData); err != nil {
		log.Error().Err(err).Msg("vaultHealth: failed to POST health data")
		// Continue execution even if POST fails, e.g., to display UI locally
	}

	fmt.Println(RenderVaultHealthCLI(healthData, healthData.Version))
}
