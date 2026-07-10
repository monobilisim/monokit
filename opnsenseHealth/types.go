package opnsenseHealth

import (
	"os"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/health"
)

type Config struct {
	Port       int `yaml:"port"`
	ExpireDays int `yaml:"expire_days"`
}

var OpnsenseHealthConfig Config

type OpnsenseHealthData struct {
	Subject       string `json:"subject"`
	Issuer        string `json:"issuer"`
	ExpiryDate    string `json:"expiryDate"`
	DaysRemaining int    `json:"daysRemaining"`
	Status        string `json:"status"` // "Valid", "Expiring Soon", "Expired", "Connection Failed"
}

type OpnsenseHealthProvider struct{}

func (p *OpnsenseHealthProvider) Name() string {
	return "opnsenseHealth"
}

func (p *OpnsenseHealthProvider) Collect(_ string) (interface{}, error) {
	if OpnsenseHealthConfig.ExpireDays == 0 {
		common.ConfInit("opnsense", &OpnsenseHealthConfig)
		if OpnsenseHealthConfig.ExpireDays == 0 {
			OpnsenseHealthConfig.ExpireDays = 7
		}
	}
	return collectOpnsenseHealthData(), nil
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "opnsenseHealth",
		EntryPoint: Main,
		Platform:   "any",
		AutoDetect: func() bool {
			if _, err := os.Stat("/usr/local/opnsense"); err == nil {
				return true
			}
			if _, err := os.Stat("/usr/local/sbin/opnsense-version"); err == nil {
				return true
			}
			return false
		},
	})
	health.Register(&OpnsenseHealthProvider{})
}
