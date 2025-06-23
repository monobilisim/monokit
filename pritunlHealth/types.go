package pritunlHealth

import (
	"github.com/monobilisim/monokit/common"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// PritunlHealthProvider implements the health.Provider interface.
type PritunlHealthProvider struct{}

// Name returns the name of the provider
func (p *PritunlHealthProvider) Name() string {
	return "pritunlHealth"
}

// Collect gathers Pritunl health data.
func (p *PritunlHealthProvider) Collect(hostname string) (interface{}, error) {
	// Initialize config if not already done
	if PritunlHealthConfig.Url == "" {
		if common.ConfExists("pritunl") {
			common.ConfInit("pritunl", &PritunlHealthConfig)
		}
		// Apply default URL after attempting to load config
		if PritunlHealthConfig.Url == "" {
			PritunlHealthConfig.Url = "mongodb://localhost:27017"
		}
	}

	return collectPritunlHealthData()
}

// Config holds configuration specific to pritunlHealth checks
type PritunlHealth struct {
	Url          string   `yaml:"url"`
	Allowed_orgs []string `yaml:"allowed_orgs"`
}

// PritunlHealthConfig is the global instance of the pritunlHealth configuration.
var PritunlHealthConfig PritunlHealth

// Client represents a connected Pritunl client
type Client struct {
	User_id      bson.ObjectID
	Real_address string
}
