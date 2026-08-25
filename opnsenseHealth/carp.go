package opnsenseHealth

import (
	"strings"

	"github.com/rs/zerolog/log"
)

// CarpState is the HA role of this box for a CARP VIP group.
type CarpState string

const (
	CarpMaster   CarpState = "MASTER"
	CarpBackup   CarpState = "BACKUP"
	CarpInit     CarpState = "INIT"
	CarpDisabled CarpState = "DISABLED"
)

// CarpVip is one CARP virtual IP and its runtime state.
type CarpVip struct {
	Interface string    `json:"interface"`
	Vhid      string    `json:"vhid"`
	State     CarpState `json:"state"`
	Subnet    string    `json:"subnet,omitempty"`
}

// CarpStatus is the CARP HA state of this box.
type CarpStatus struct {
	Configured bool `json:"configured"`
	// IsBackupForTunnels is true when every CARP VIP that a tunnel depends on
	// (via WireGuard's carp_depend_on) is BACKUP. When true, WireGuard and
	// IPSec alarms are suppressed — the BACKUP node legitimately has no
	// tunnels because the peer talks to the MASTER's VIP. A global "all VIPs
	// are BACKUP" test would never fire on a node that is MASTER for some VIP
	// groups and BACKUP for others.
	IsBackupForTunnels bool      `json:"isBackupForTunnels"`
	Vips               []CarpVip `json:"vips,omitempty"`
	Error              string    `json:"error,omitempty"`
}

// isVIPBackup reports whether the VIP identified by its config UUID is
// currently BACKUP. It fails safe: an unknown UUID or a VIP absent from
// ifconfig output returns false so the tunnel keeps alarming rather than a
// broken lookup silencing it.
func (s *CarpStatus) isVIPBackup(uuid string, vipVhidMap map[string]string) bool {
	vhid, ok := vipVhidMap[uuid]
	if !ok || vhid == "" {
		return false
	}
	for _, vip := range s.Vips {
		if vip.Vhid == vhid {
			return vip.State == CarpBackup
		}
	}
	return false
}

// collectCarpHealth reports the CARP state of this box from `ifconfig` output.
// It fails safe: when CARP is configured but its state cannot be read, the
// status reports IsBackupForTunnels=false so WireGuard and IPSec keep alarming
// rather than a broken check silencing every tunnel alarm.
func collectCarpHealth(names *opnsenseNames) *CarpStatus {
	if !names.CarpConfigured {
		return &CarpStatus{Configured: false}
	}

	status := &CarpStatus{Configured: true}

	ifconfig := resolveBinary("ifconfig", "/sbin/ifconfig", "/usr/sbin/ifconfig")
	if ifconfig == "" {
		status.Error = "ifconfig not found"
		log.Warn().Msg("CARP is configured but ifconfig is not available, cannot read HA state")
		return status
	}

	out, err := runCmd(ifconfig)
	if err != nil {
		status.Error = err.Error()
		log.Warn().Err(err).Msg("CARP is configured but ifconfig failed, cannot read HA state")
		return status
	}

	status.Vips = parseCarpIfconfig(string(out))

	// Only VIPs that tunnels actually depend on decide IsBackupForTunnels;
	// on a busy HA pair the box is routinely MASTER for some groups and
	// BACKUP for others, so the full VIP list proves nothing per tunnel.
	depUUIDs := make(map[string]bool)
	for _, server := range names.WGServers {
		if server.CarpDependOn != "" {
			depUUIDs[server.CarpDependOn] = true
		}
	}
	if len(depUUIDs) > 0 {
		allBackup := true
		for uuid := range depUUIDs {
			if !status.isVIPBackup(uuid, names.VIPVhidMap) {
				allBackup = false
				break
			}
		}
		status.IsBackupForTunnels = allBackup
	}

	return status
}

// parseCarpIfconfig extracts CARP lines such as
// "carp: MASTER vhid 2 advbase 1 advskew 0" from full `ifconfig` output. Each
// CARP line is a tab-indented continuation of its interface's header line
// ("carp0: flags=..."), so the current interface is tracked from the headers.
func parseCarpIfconfig(out string) []CarpVip {
	var vips []CarpVip
	var iface string

	for _, line := range strings.Split(out, "\n") {
		// Interface headers start in column 0; continuation lines are indented.
		if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			if idx := strings.Index(line, ":"); idx > 0 {
				iface = line[:idx]
			}
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "carp:" {
			continue
		}

		vip := CarpVip{Interface: iface, State: CarpState(fields[1])}
		for i := 2; i+1 < len(fields); i++ {
			if fields[i] == "vhid" {
				vip.Vhid = fields[i+1]
				break
			}
		}
		vips = append(vips, vip)
	}

	return vips
}
