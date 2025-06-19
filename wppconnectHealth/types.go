package wppconnectHealth

import (
	"github.com/monobilisim/monokit/common"
)

// WppConnectHealthProvider implements the health.Provider interface.
type WppConnectHealthProvider struct{}

// Name returns the name of the provider
func (p *WppConnectHealthProvider) Name() string {
	return "wppconnectHealth"
}

// Collect gathers WPPConnect health data.
func (p *WppConnectHealthProvider) Collect(hostname string) (interface{}, error) {
	// Initialize config if not already done
	if WppConnectHealthConfig.Wpp.Url == "" || WppConnectHealthConfig.Wpp.Secret == "" {
		if common.ConfExists("wppconnect") {
			common.ConfInit("wppconnect", &WppConnectHealthConfig)
		}
	}

	return CollectWppConnectHealthData(), nil
}

// Config holds configuration specific to wppconnectHealth checks
type Config struct {
	Wpp struct {
		Secret string `yaml:"secret"`
		Url    string `yaml:"url"`
	} `yaml:"wpp"`
}

// WppConnectHealthConfig is the global instance of the wppconnectHealth configuration.
var WppConnectHealthConfig Config

// WppConnectData holds information about WPPConnect session status
type WppConnectData struct {
	Session     string
	ContactName string
	Status      string
	Healthy     bool
}

// WppConnectHealthData holds the overall health status of WPPConnect
type WppConnectHealthData struct {
	Sessions     []WppConnectData
	TotalCount   int
	HealthyCount int
	Healthy      bool
	Version      string
}
