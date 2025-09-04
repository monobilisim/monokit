//go:build plugin && linux

package zimbraHealth

import (
	mail "github.com/monobilisim/monokit/common/mail"
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

// collectZimbraHealthData is a wrapper around the existing collectHealthData function
// to provide the interface required by the plugin system
func collectZimbraHealthData() (*ZimbraHealthData, error) {
	return collectHealthData(), nil
}

// ZimbraHealthConfig is the global instance of the zimbraHealth configuration
// This is an alias to the MailHealthConfig used by the zimbraHealth package
var ZimbraHealthConfig mail.MailHealth
