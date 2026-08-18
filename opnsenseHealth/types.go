package opnsenseHealth

import (
	"os"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/health"
)

// Config mirrors opnsense.yaml. The tags must be mapstructure, not yaml:
// ConfInit decodes through viper, which ignores yaml tags — a mismatched key
// silently leaves the field at its zero value.
type Config struct {
	Port       int `mapstructure:"port"`
	ExpireDays int `mapstructure:"expire_days"`

	Wireguard struct {
		Enabled *bool `mapstructure:"enabled"`
		// HandshakeTimeout is the age in seconds after which a peer that keeps a
		// persistent tunnel (persistent-keepalive set) is considered down.
		HandshakeTimeout int `mapstructure:"handshake_timeout"`
		// ExcludedPeers silences individual peers, matched by name or public key.
		ExcludedPeers []string `mapstructure:"excluded_peers"`
	} `mapstructure:"wireguard"`

	Ipsec struct {
		Enabled *bool `mapstructure:"enabled"`
		// ExcludedConnections silences connections, matched by description or
		// UUID. Useful for responder-only tunnels that legitimately have no SA.
		ExcludedConnections []string `mapstructure:"excluded_connections"`
	} `mapstructure:"ipsec"`

	Gateway struct {
		Enabled *bool `mapstructure:"enabled"`
		// LossLimit is the packet loss percentage at or above which a gateway is
		// considered down.
		LossLimit float64 `mapstructure:"loss_limit"`
	} `mapstructure:"gateway"`

	Dns struct {
		Enabled *bool `mapstructure:"enabled"`
		// Server is the resolver to query, as host:port.
		Server string `mapstructure:"server"`
		// Query is the name to resolve.
		Query string `mapstructure:"query"`
	} `mapstructure:"dns"`
}

var OpnsenseHealthConfig Config

// Default values applied when the corresponding config key is absent.
const (
	defaultExpireDays       = 7
	defaultHandshakeTimeout = 300
	defaultLossLimit        = 20.0
	defaultDNSServer        = "127.0.0.1:53"
	defaultDNSQuery         = "opnsense.org"
)

// WireGuardPeer is a single peer of a WireGuard interface.
type WireGuardPeer struct {
	Interface     string `json:"interface"`
	Name          string `json:"name,omitempty"`
	PublicKey     string `json:"publicKey"`
	Endpoint      string `json:"endpoint,omitempty"`
	AllowedIPs    string `json:"allowedIps,omitempty"`
	Keepalive     int    `json:"keepalive,omitempty"`
	LastHandshake int64  `json:"lastHandshake"`       // unix seconds, 0 = never
	HandshakeAge  int64  `json:"handshakeAgeSeconds"` // -1 when never handshaked
	Persistent    bool   `json:"persistent"`          // this box dials out to the peer
	Problem       string `json:"problem,omitempty"`   // NO_HANDSHAKE / STALE
}

// WireGuardInterface is one wg device and the health of its peers.
type WireGuardInterface struct {
	Name          string          `json:"name"`
	Up            bool            `json:"up"`
	Flags         string          `json:"flags,omitempty"`
	Missing       bool            `json:"missing,omitempty"` // configured in OPNsense but no device
	ListenPort    string          `json:"listenPort,omitempty"`
	PeerCount     int             `json:"peerCount"`
	ProblemPeers  []WireGuardPeer `json:"problemPeers,omitempty"`
	ExcludedPeers int             `json:"excludedPeers,omitempty"`
	// IdlePeers counts dial-in peers with no handshake. They are reported for
	// visibility but never alarmed — an offline client is not a fault.
	IdlePeers int  `json:"idlePeers,omitempty"`
	Healthy   bool `json:"healthy"`
}

type WireGuardStatus struct {
	Interfaces []WireGuardInterface `json:"interfaces"`
	Error      string               `json:"error,omitempty"`
}

// IPSecConnection is one strongSwan connection and its Phase 1 state.
type IPSecConnection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	LocalAddrs  string `json:"localAddrs,omitempty"`
	RemoteAddrs string `json:"remoteAddrs,omitempty"`
	Version     string `json:"version,omitempty"`
	State       string `json:"state"` // ESTABLISHED / CONNECTING / NO_PHASE1 / ...
	Established int64  `json:"establishedSeconds,omitempty"`
	Healthy     bool   `json:"healthy"`
	Excluded    bool   `json:"excluded,omitempty"`
}

type IPSecStatus struct {
	Connections []IPSecConnection `json:"connections"`
	Error       string            `json:"error,omitempty"`
}

// GatewayInfo mirrors one entry of gateway_status.php.
type GatewayInfo struct {
	Name        string  `json:"name"`
	Address     string  `json:"address,omitempty"`
	Monitor     string  `json:"monitor,omitempty"`
	Status      string  `json:"status,omitempty"`
	StatusText  string  `json:"statusText,omitempty"`
	Loss        string  `json:"loss,omitempty"`
	LossPercent float64 `json:"lossPercent"`
	HasLoss     bool    `json:"hasLoss"` // false when the gateway has no monitor
	Delay       string  `json:"delay,omitempty"`
	StdDev      string  `json:"stddev,omitempty"`
	Healthy     bool    `json:"healthy"`
	Skipped     bool    `json:"skipped,omitempty"` // force_down or still pending
	Pending     bool    `json:"pending,omitempty"` // no monitoring data yet; alarm state left alone
	Reason      string  `json:"reason,omitempty"`
}

type GatewayStatus struct {
	Gateways []GatewayInfo `json:"gateways"`
	Error    string        `json:"error,omitempty"`
}

// DNSStatus is the result of a resolver probe against the local Unbound.
type DNSStatus struct {
	Server     string   `json:"server"`
	Query      string   `json:"query"`
	Resolved   []string `json:"resolved,omitempty"`
	DurationMs int64    `json:"durationMs"`
	Healthy    bool     `json:"healthy"`
	Error      string   `json:"error,omitempty"`
}

type OpnsenseHealthData struct {
	Subject       string `json:"subject"`
	Issuer        string `json:"issuer"`
	ExpiryDate    string `json:"expiryDate"`
	DaysRemaining int    `json:"daysRemaining"`
	Status        string `json:"status"` // SSL: "Valid", "Expiring Soon", "Expired", "Connection Failed"

	// Nil when the corresponding subsystem is disabled or not present on the box.
	WireGuard *WireGuardStatus `json:"wireguard,omitempty"`
	IPSec     *IPSecStatus     `json:"ipsec,omitempty"`
	Gateways  *GatewayStatus   `json:"gateways,omitempty"`
	DNS       *DNSStatus       `json:"dns,omitempty"`
}

type OpnsenseHealthProvider struct{}

func (p *OpnsenseHealthProvider) Name() string {
	return "opnsenseHealth"
}

func (p *OpnsenseHealthProvider) Collect(_ string) (interface{}, error) {
	if OpnsenseHealthConfig.ExpireDays == 0 {
		common.ConfInit("opnsense", &OpnsenseHealthConfig)
	}
	applyConfigDefaults()
	return collectOpnsenseHealthData(), nil
}

// applyConfigDefaults fills in the values omitted from opnsense.yaml.
func applyConfigDefaults() {
	if OpnsenseHealthConfig.ExpireDays == 0 {
		OpnsenseHealthConfig.ExpireDays = defaultExpireDays
	}
	if OpnsenseHealthConfig.Wireguard.HandshakeTimeout == 0 {
		OpnsenseHealthConfig.Wireguard.HandshakeTimeout = defaultHandshakeTimeout
	}
	if OpnsenseHealthConfig.Gateway.LossLimit == 0 {
		OpnsenseHealthConfig.Gateway.LossLimit = defaultLossLimit
	}
	// Dns.Server is deliberately left empty here so resolveDNSServer can fall
	// back to Unbound's own configured port before the 127.0.0.1:53 default.
	if OpnsenseHealthConfig.Dns.Query == "" {
		OpnsenseHealthConfig.Dns.Query = defaultDNSQuery
	}
}

// enabled treats an omitted config toggle as enabled, so adding a check does
// not require touching an existing opnsense.yaml.
func enabled(flag *bool) bool {
	return flag == nil || *flag
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "opnsenseHealth",
		EntryPoint: Main,
		Platform:   "any",
		AutoDetect: func() bool {
			// Require opnsense.yaml so deleting the config disables the check
			if !common.ConfExists("opnsense") {
				return false
			}
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
