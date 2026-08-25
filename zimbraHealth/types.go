//go:build plugin && linux

package zimbraHealth

import (
	"os"
	"time"

	mail "github.com/monobilisim/monokit/common/mail"
	"github.com/rs/zerolog/log"
)

// ZimbraHealthProvider implements the health.Provider interface
type ZimbraHealthProvider struct{}

// Name returns the name of the provider
func (p *ZimbraHealthProvider) Name() string {
	return "zimbraHealth"
}

// Collect gathers Zimbra health data.
// The 'hostname' parameter is ignored for zimbraHealth as it collects local data.
func (p *ZimbraHealthProvider) Collect(_ string) (interface{}, error) {
	return collectZimbraHealthData()
}

// collectZimbraHealthData is the plugin entry point called by the daemon via
// gRPC on every cycle. It applies the same caching + live-service-check
// throttling as Main() to prevent spawning ProvUtil JVMs every minute via
// zmcontrol status when Zimbra services are degraded.
func collectZimbraHealthData() (*ZimbraHealthData, error) {
	if !shouldRunFullCheck() {
		healthData, err := loadCachedData()
		if err == nil {
			if zimbraPath == "" {
				if _, derr := os.Stat("/opt/zimbra"); !os.IsNotExist(derr) {
					zimbraPath = "/opt/zimbra"
				}
			}
			if zimbraPath != "" && shouldRunLiveServiceCheck() {
				recordLiveServiceCheck()
				currentServices := CheckZimbraServices()
				if len(currentServices) > 0 {
					healthData.Services = currentServices
					if healthData.System.ProductPath == "" {
						healthData.System.ProductPath = zimbraPath
					}
					healthData.System.LastChecked = time.Now().Format("2006-01-02 15:04:05")
				}
			}
			return healthData, nil
		}
		log.Warn().Err(err).Msg("Failed to load cached data in plugin path, running full check")
	}
	healthData := collectHealthData()
	_ = saveCachedData(healthData)
	return healthData, nil
}

// ZimbraHealthConfig is the global instance of the zimbraHealth configuration
// This is an alias to the MailHealthConfig used by the zimbraHealth package
var ZimbraHealthConfig mail.MailHealth
