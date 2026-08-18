package opnsenseHealth

import (
	"encoding/xml"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

const opnsenseConfigPath = "/conf/config.xml"

// xmlNode is a generic node used to look up names in /conf/config.xml without
// hardcoding model paths. OPNsense has moved WireGuard and IPsec between
// plugin and core models over the years, and the paths differ per version, so
// the lookups below scope by ancestor element name and search descendants
// instead of pinning an exact path. Every lookup is best-effort: when it comes
// up empty the checks fall back to raw identifiers.
type xmlNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Nodes   []xmlNode  `xml:",any"`
	Text    string     `xml:",chardata"`
}

func (n *xmlNode) text() string {
	return strings.TrimSpace(n.Text)
}

func (n *xmlNode) attr(name string) string {
	for _, a := range n.Attrs {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

func (n *xmlNode) child(name string) *xmlNode {
	for i := range n.Nodes {
		if strings.EqualFold(n.Nodes[i].XMLName.Local, name) {
			return &n.Nodes[i]
		}
	}
	return nil
}

func (n *xmlNode) childText(name string) string {
	if c := n.child(name); c != nil {
		return c.text()
	}
	return ""
}

// findAll collects every descendant (and n itself) whose local element name
// matches name.
func (n *xmlNode) findAll(name string) []*xmlNode {
	var out []*xmlNode
	var walk func(node *xmlNode)
	walk = func(node *xmlNode) {
		if strings.EqualFold(node.XMLName.Local, name) {
			out = append(out, node)
		}
		for i := range node.Nodes {
			walk(&node.Nodes[i])
		}
	}
	walk(n)
	return out
}

// wgServerConfig is a WireGuard instance as configured in OPNsense. Device is
// derived from the instance number, which is how OPNsense names the interface.
type wgServerConfig struct {
	Name     string
	Device   string
	Enabled  bool
	Instance string
}

// opnsenseNames holds everything read out of /conf/config.xml.
type opnsenseNames struct {
	Domain string

	WGPeerNames map[string]string // public key -> peer name
	// WGPersistentPeers marks peers this box dials out to, i.e. those with a
	// configured endpoint. Dial-in (road-warrior) peers are absent, which is
	// what keeps an offline laptop from being reported as a broken tunnel.
	WGPersistentPeers map[string]bool
	WGServers         []wgServerConfig
	WGDisabled        bool // WireGuard service switched off entirely

	IPSecNames map[string]string // connection UUID or "con<ikeid>" -> description
	// IPSecConfigured is false when no IPsec tunnel is defined at all, which
	// distinguishes "not in use" from "broken" if the status script fails.
	IPSecConfigured bool

	UnboundDisabled bool   // only set when config says so explicitly
	UnboundPort     string // custom listen port, empty for the default
}

func newOpnsenseNames() *opnsenseNames {
	return &opnsenseNames{
		WGPeerNames:       make(map[string]string),
		WGPersistentPeers: make(map[string]bool),
		IPSecNames:        make(map[string]string),
	}
}

// loadOpnsenseConfig parses /conf/config.xml. It never returns nil so callers
// can use it unconditionally; on failure the maps are simply empty.
func loadOpnsenseConfig() *opnsenseNames {
	names := newOpnsenseNames()

	raw, err := os.ReadFile(opnsenseConfigPath)
	if err != nil {
		log.Warn().Err(err).Msg("Could not read " + opnsenseConfigPath + ", falling back to raw identifiers")
		return names
	}

	var root xmlNode
	if err := xml.Unmarshal(raw, &root); err != nil {
		log.Warn().Err(err).Msg("Could not parse " + opnsenseConfigPath)
		return names
	}

	names.Domain = parseDomain(&root)
	parseWireGuardConfig(&root, names)
	parseIPSecNames(&root, names)
	names.UnboundDisabled = unboundExplicitlyDisabled(&root)
	names.UnboundPort = unboundPort(&root)

	log.Debug().
		Int("wgPeers", len(names.WGPeerNames)).
		Int("wgPersistentPeers", len(names.WGPersistentPeers)).
		Int("wgServers", len(names.WGServers)).
		Bool("wgDisabled", names.WGDisabled).
		Int("ipsecConns", len(names.IPSecNames)).
		Bool("ipsecConfigured", names.IPSecConfigured).
		Bool("unboundDisabled", names.UnboundDisabled).
		Str("unboundPort", names.UnboundPort).
		Msg("Parsed OPNsense config.xml")

	return names
}

func parseDomain(root *xmlNode) string {
	system := root.child("system")
	if system == nil {
		return ""
	}
	hostname := system.childText("hostname")
	domain := system.childText("domain")
	if hostname != "" && domain != "" {
		return hostname + "." + domain
	}
	return hostname
}

func parseWireGuardConfig(root *xmlNode, names *opnsenseNames) {
	for _, wg := range root.findAll("wireguard") {
		// The service-wide toggle lives under <general>. Without this, turning
		// WireGuard off would report every configured instance as missing.
		if general := wg.child("general"); general != nil {
			if v := general.childText("enabled"); v != "" && !isTruthy(v) {
				names.WGDisabled = true
			}
		}

		// Peers are <client> nodes carrying both a name and a public key. The
		// pubkey requirement keeps unrelated <client> elements out.
		for _, c := range wg.findAll("client") {
			pubkey := c.childText("pubkey")
			if pubkey == "" {
				continue
			}
			if name := c.childText("name"); name != "" {
				names.WGPeerNames[pubkey] = name
			}
			// A configured endpoint means this box initiates the tunnel, so a
			// missing handshake is a fault rather than an idle client.
			for _, key := range []string{"serveraddress", "endpoint", "peer_endpoint"} {
				if c.childText(key) != "" {
					names.WGPersistentPeers[pubkey] = true
					break
				}
			}
		}

		// Instances are <server> nodes carrying an instance number. The wrapper
		// <server> element that only holds <servers> has no instance, so it is
		// skipped.
		for _, s := range wg.findAll("server") {
			instance := s.childText("instance")
			if instance == "" || s.child("pubkey") == nil {
				continue
			}
			name := s.childText("name")
			if name == "" {
				name = "wg" + instance
			}
			names.WGServers = append(names.WGServers, wgServerConfig{
				Name:     name,
				Device:   "wg" + instance,
				Enabled:  isTruthy(s.childText("enabled")),
				Instance: instance,
			})
		}
	}
}

func parseIPSecNames(root *xmlNode, names *opnsenseNames) {
	// Connections-model tunnels are keyed by UUID in list_status.py output.
	for _, sw := range root.findAll("Swanctl") {
		for _, c := range sw.findAll("Connection") {
			uuid := c.attr("uuid")
			if uuid == "" {
				continue
			}
			names.IPSecConfigured = true
			if desc := c.childText("description"); desc != "" {
				names.IPSecNames[uuid] = desc
			}
		}
	}

	// Legacy tunnels surface as "con<ikeid>".
	for _, ipsec := range root.findAll("ipsec") {
		for _, p := range ipsec.findAll("phase1") {
			ikeid := p.childText("ikeid")
			if ikeid == "" {
				continue
			}
			names.IPSecConfigured = true
			if descr := p.childText("descr"); descr != "" {
				names.IPSecNames["con"+ikeid] = descr
				names.IPSecNames[ikeid] = descr
			}
		}
	}
}

// unboundPort returns Unbound's configured listen port, empty when it uses the
// default. A resolver moved off port 53 would otherwise look unreachable.
func unboundPort(root *xmlNode) string {
	for _, section := range []string{"unboundplus", "unbound"} {
		for _, n := range root.findAll(section) {
			for _, holder := range []*xmlNode{n.child("general"), n} {
				if holder == nil {
					continue
				}
				if port := holder.childText("port"); port != "" && port != "53" {
					return port
				}
			}
		}
	}
	return ""
}

// unboundExplicitlyDisabled reports whether config.xml positively says Unbound
// is off. An absent or ambiguous setting returns false so the resolver probe
// still runs — the check can always be turned off in opnsense.yaml.
func unboundExplicitlyDisabled(root *xmlNode) bool {
	for _, section := range []string{"unboundplus", "unbound"} {
		for _, n := range root.findAll(section) {
			for _, key := range []string{"enabled", "enable"} {
				// The flag can sit directly on the section or under <general>.
				for _, holder := range []*xmlNode{n, n.child("general")} {
					if holder == nil {
						continue
					}
					c := holder.child(key)
					if c == nil {
						continue
					}
					if c.text() == "0" {
						return true
					}
					// Present and not "0" means enabled; legacy configs use an
					// empty element for "on", so do not treat "" as disabled.
					return false
				}
			}
		}
	}
	return false
}

func isTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
