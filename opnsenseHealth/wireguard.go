package opnsenseHealth

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

// Problem classifications for a peer.
const (
	peerNoHandshake = "NO_HANDSHAKE"
	peerStale       = "STALE"
)

// collectWireGuardHealth reports per-interface WireGuard health. It returns nil
// when WireGuard is not installed on the box.
//
// One alarm is raised per interface, covering both the link state and its
// peers, so a downed interface does not also fan out an alarm for every peer
// that went stale with it.
func collectWireGuardHealth(names *opnsenseNames) *WireGuardStatus {
	wgBin := resolveBinary("wg", "/usr/local/bin/wg")
	if wgBin == "" {
		log.Debug().Msg("wg binary not found, skipping WireGuard check")
		return nil
	}
	// The binary ships regardless of whether the service is switched on, so the
	// config is what decides. Without this, every configured instance would be
	// reported as a missing device on a box with WireGuard deliberately off.
	if names.WGDisabled {
		log.Debug().Msg("WireGuard is disabled in config.xml, skipping WireGuard check")
		return nil
	}

	status := &WireGuardStatus{}

	out, err := runCmd(wgBin, "show", "all", "dump")
	if err != nil {
		status.Error = err.Error()
		common.AlarmCheckDown("opnsense_wireguard", "Could not read WireGuard status: "+err.Error(), false, "", "")
		return status
	}
	common.AlarmCheckUp("opnsense_wireguard", "WireGuard status is readable again.", false)

	status.Interfaces = parseWgDump(out, names)
	status.Interfaces = appendMissingWgInterfaces(status.Interfaces, names)

	for i := range status.Interfaces {
		evaluateWgInterface(&status.Interfaces[i])
	}

	return status
}

// parseWgDump parses `wg show all dump`. Interface lines carry 5 tab-separated
// fields, peer lines 9.
func parseWgDump(out []byte, names *opnsenseNames) []WireGuardInterface {
	var ordered []*WireGuardInterface
	index := make(map[string]*WireGuardInterface)

	staleAfter := int64(OpnsenseHealthConfig.Wireguard.HandshakeTimeout)
	now := time.Now().Unix()

	ensure := func(name string) *WireGuardInterface {
		if iface, ok := index[name]; ok {
			return iface
		}
		up, flags := interfaceFlags(name)
		iface := &WireGuardInterface{Name: name, Up: up, Flags: flags}
		index[name] = iface
		ordered = append(ordered, iface)
		return iface
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")

		switch len(fields) {
		case 5:
			iface := ensure(fields[0])
			iface.ListenPort = fields[3]

		case 9:
			iface := ensure(fields[0])
			iface.PeerCount++

			peer := WireGuardPeer{
				Interface:  fields[0],
				PublicKey:  fields[1],
				Endpoint:   normalizeWgValue(fields[3]),
				AllowedIPs: normalizeWgValue(fields[4]),
			}
			peer.Name = names.WGPeerNames[peer.PublicKey]
			peer.Keepalive = parseKeepalive(fields[8])

			handshake, _ := strconv.ParseInt(strings.TrimSpace(fields[5]), 10, 64)
			peer.LastHandshake = handshake
			if handshake == 0 {
				peer.HandshakeAge = -1
			} else {
				peer.HandshakeAge = now - handshake
			}

			// Only peers this box is supposed to keep a tunnel with are judged.
			// A dial-in client that is simply switched off reports no handshake
			// too, and wg resets latest-handshake to 0 whenever the device is
			// recreated (reboot, config apply) — so alarming on every peer
			// without a handshake would lock the interface alarm down for good
			// and destroy its signal.
			peer.Persistent = peer.Keepalive > 0 || names.WGPersistentPeers[peer.PublicKey]
			if !peer.Persistent {
				if handshake == 0 {
					iface.IdlePeers++
				}
				continue
			}

			switch {
			case handshake == 0:
				peer.Problem = peerNoHandshake
			case peer.HandshakeAge > staleAfter:
				peer.Problem = peerStale
			}

			if peer.Problem == "" {
				continue
			}
			if isExcludedPeer(peer) {
				iface.ExcludedPeers++
				continue
			}
			iface.ProblemPeers = append(iface.ProblemPeers, peer)

		default:
			log.Debug().Str("line", line).Int("fields", len(fields)).Msg("Unrecognized wg dump line, skipping")
		}
	}

	ifaces := make([]WireGuardInterface, 0, len(ordered))
	for _, iface := range ordered {
		ifaces = append(ifaces, *iface)
	}
	return ifaces
}

// appendMissingWgInterfaces flags enabled OPNsense instances that have no
// device at all — the tunnel is gone rather than merely down.
func appendMissingWgInterfaces(ifaces []WireGuardInterface, names *opnsenseNames) []WireGuardInterface {
	present := make(map[string]bool, len(ifaces))
	for _, i := range ifaces {
		present[i.Name] = true
	}

	for _, server := range names.WGServers {
		if !server.Enabled || present[server.Device] {
			continue
		}
		ifaces = append(ifaces, WireGuardInterface{
			Name:    server.Device,
			Missing: true,
		})
	}
	return ifaces
}

func evaluateWgInterface(iface *WireGuardInterface) {
	alarmName := "opnsense_wireguard_" + alarmSuffix(iface.Name)

	switch {
	case iface.Missing:
		iface.Healthy = false
		common.AlarmCheckDown(alarmName,
			fmt.Sprintf("WireGuard interface %s is enabled in OPNsense but the device does not exist.", iface.Name),
			false, "", "")

	case !iface.Up:
		iface.Healthy = false
		common.AlarmCheckDown(alarmName,
			fmt.Sprintf("WireGuard interface %s is DOWN (flags: %s).", iface.Name, iface.Flags),
			false, "", "")

	case len(iface.ProblemPeers) > 0:
		iface.Healthy = false
		common.AlarmCheckDown(alarmName, wgPeerAlarmMessage(iface), false, "", "")

	default:
		iface.Healthy = true
		common.AlarmCheckUp(alarmName,
			fmt.Sprintf("WireGuard interface %s is UP and all %d peer(s) have a live tunnel.", iface.Name, iface.PeerCount),
			false)
	}
}

func wgPeerAlarmMessage(iface *WireGuardInterface) string {
	rows := make([][]string, 0, len(iface.ProblemPeers))
	for _, p := range iface.ProblemPeers {
		name := p.Name
		if name == "" {
			name = shortKey(p.PublicKey)
		}
		endpoint := p.Endpoint
		if endpoint == "" {
			endpoint = "-"
		}
		rows = append(rows, []string{name, endpoint, p.AllowedIPs, humanAge(p.HandshakeAge), p.Problem})
	}

	table := renderTable([]string{"Peer", "Endpoint", "Allowed IPs", "Last Handshake", "Problem"}, rows)
	return fmt.Sprintf("WireGuard interface %s has %d of %d peer(s) without a live tunnel;\n\n%s",
		iface.Name, len(iface.ProblemPeers), iface.PeerCount, table)
}

func isExcludedPeer(peer WireGuardPeer) bool {
	for _, ex := range OpnsenseHealthConfig.Wireguard.ExcludedPeers {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}
		if ex == peer.PublicKey || ex == peer.Interface+":"+peer.PublicKey {
			return true
		}
		if peer.Name != "" && strings.EqualFold(ex, peer.Name) {
			return true
		}
	}
	return false
}

// interfaceFlags is a variable so tests can parse a dump without shelling out
// to ifconfig.
var interfaceFlags = ifconfigFlags

// ifconfigFlags reports whether an interface is UP and RUNNING, alongside the
// raw flag list for the alarm message.
func ifconfigFlags(name string) (bool, string) {
	ifconfig := resolveBinary("ifconfig", "/sbin/ifconfig", "/usr/sbin/ifconfig")
	if ifconfig == "" {
		log.Debug().Msg("ifconfig not found, assuming WireGuard interfaces are up")
		return true, ""
	}

	out, err := runCmd(ifconfig, name)
	if err != nil {
		return false, "not found"
	}

	flags := extractIfconfigFlags(string(out))
	hasUp, hasRunning := false, false
	for _, f := range strings.Split(flags, ",") {
		switch strings.TrimSpace(f) {
		case "UP":
			hasUp = true
		case "RUNNING":
			hasRunning = true
		}
	}
	return hasUp && hasRunning, flags
}

// extractIfconfigFlags pulls the flag list out of a line such as
// "wg0: flags=80c1<UP,RUNNING,NOARP,MULTICAST> metric 0 mtu 1420".
func extractIfconfigFlags(out string) string {
	idx := strings.Index(out, "flags=")
	if idx == -1 {
		return ""
	}
	open := strings.Index(out[idx:], "<")
	if open == -1 {
		return ""
	}
	rest := out[idx+open+1:]
	end := strings.Index(rest, ">")
	if end == -1 {
		return ""
	}
	return rest[:end]
}

// normalizeWgValue turns wg's "(none)" placeholder into an empty string.
func normalizeWgValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "(none)" || s == "none" {
		return ""
	}
	return s
}

func parseKeepalive(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return v
}
