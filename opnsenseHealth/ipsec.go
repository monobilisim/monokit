package opnsenseHealth

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

const ipsecStatusScript = "/usr/local/opnsense/scripts/ipsec/list_status.py"

// Phase 1 states reported by strongSwan.
const (
	ipsecEstablished = "ESTABLISHED"
	ipsecConnecting  = "CONNECTING"
	ipsecNoPhase1    = "NO_PHASE1"
)

// flexString decodes a JSON value that list_status.py may emit as a string, a
// number, or a list of strings depending on the strongSwan version.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*f = ""
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexString(s)
		return nil
	}

	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*f = flexString(strings.Join(list, ", "))
		return nil
	}

	// Numbers and anything else fall through to their raw form.
	*f = flexString(strings.Trim(trimmed, `"`))
	return nil
}

func (f flexString) int64() int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(string(f)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

type ipsecRawSA struct {
	State       flexString `json:"state"`
	Established flexString `json:"established"`
	LocalHost   flexString `json:"local-host"`
	RemoteHost  flexString `json:"remote-host"`
	Version     flexString `json:"version"`
}

// flexSAs accepts the SA list either as a JSON array or as an object keyed by
// uniqueid. A shape change here would otherwise drop the connection entirely
// and silently turn the whole IPSec check into a no-op.
type flexSAs []ipsecRawSA

func (s *flexSAs) UnmarshalJSON(data []byte) error {
	var list []ipsecRawSA
	if err := json.Unmarshal(data, &list); err == nil {
		*s = list
		return nil
	}

	var keyed map[string]ipsecRawSA
	if err := json.Unmarshal(data, &keyed); err != nil {
		return err
	}
	// Sort by key so the reported state is stable between runs.
	keys := make([]string, 0, len(keyed))
	for k := range keyed {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]ipsecRawSA, 0, len(keyed))
	for _, k := range keys {
		out = append(out, keyed[k])
	}
	*s = out
	return nil
}

type ipsecRawConn struct {
	LocalAddrs  flexString `json:"local-addrs"`
	RemoteAddrs flexString `json:"remote-addrs"`
	Version     flexString `json:"version"`
	SAs         flexSAs    `json:"sas"`
}

// collectIPSecHealth reports Phase 1 state per connection. It returns nil when
// the OPNsense IPsec status script is not available.
//
// Only Phase 1 is checked: a connection whose IKE SA is established is
// considered up, because alarming on individual child SAs is far too noisy.
func collectIPSecHealth(names *opnsenseNames) *IPSecStatus {
	if _, err := os.Stat(ipsecStatusScript); err != nil {
		log.Debug().Msg(ipsecStatusScript + " not found, skipping IPSec check")
		return nil
	}

	python := resolveBinary("python3", "/usr/local/bin/python3")
	if python == "" {
		log.Debug().Msg("python3 not found, skipping IPSec check")
		return nil
	}

	status := &IPSecStatus{}

	out, err := runCmd(python, ipsecStatusScript)
	if err != nil {
		// list_status.py ships with OPNsense core whether or not IPsec is in
		// use, and it fails when charon is not running. Treating that as a
		// fault would alarm every box that does not use IPsec, so it only
		// counts when a tunnel is actually configured.
		if !names.IPSecConfigured {
			log.Debug().Err(err).Msg("IPSec status unavailable and no tunnel configured, skipping IPSec check")
			return nil
		}
		status.Error = err.Error()
		common.AlarmCheckDown("opnsense_ipsec", "Could not read IPSec status: "+err.Error(), false, "", "")
		return status
	}

	// No output at all means no connections are loaded, not a broken script.
	payload := trimToJSON(out)
	if trimmed := strings.TrimSpace(string(payload)); trimmed == "" || trimmed == "{}" || trimmed == "[]" {
		common.AlarmCheckUp("opnsense_ipsec", "IPSec status is readable again.", false)
		return status
	}

	// Decode per connection so a single unexpected field does not discard the
	// status of every other tunnel. The connection ID doubles as the alarm key
	// and the GUI-name lookup, so a list payload has to identify its entries by
	// a self-declared field: a positional fallback would re-key every alarm as
	// soon as one tunnel appears or disappears.
	entries, err := decodeJSONEntries(payload, []string{"id", "name", "connection"}, "ipsec_")
	if err != nil {
		status.Error = err.Error()
		common.AlarmCheckDown("opnsense_ipsec", "Could not parse IPSec status output: "+err.Error(), false, "", "")
		return status
	}

	var undecodable []string

	for _, entry := range entries {
		id := entry.key

		var raw ipsecRawConn
		if err := json.Unmarshal(entry.raw, &raw); err != nil {
			// Skipping silently would leave a dead tunnel unmonitored while the
			// UI claimed there was nothing to report, so this is surfaced.
			log.Warn().Err(err).Str("connection", id).Msg("Could not parse IPSec connection, skipping")
			undecodable = append(undecodable, id)
			continue
		}

		conn := IPSecConnection{
			ID:          id,
			Name:        ipsecName(id, names),
			LocalAddrs:  string(raw.LocalAddrs),
			RemoteAddrs: string(raw.RemoteAddrs),
			Version:     string(raw.Version),
		}
		conn.State, conn.Established = phase1State(raw.SAs)
		conn.Healthy = conn.State == ipsecEstablished
		conn.Excluded = isExcludedIPSecConn(conn)

		status.Connections = append(status.Connections, conn)
		evaluateIPSecConnection(conn)
	}

	if len(undecodable) > 0 {
		status.Error = fmt.Sprintf("could not parse %d connection(s): %s",
			len(undecodable), strings.Join(undecodable, ", "))
		common.AlarmCheckDown("opnsense_ipsec", "IPSec status is partially unreadable: "+status.Error, false, "", "")
	} else {
		common.AlarmCheckUp("opnsense_ipsec", "IPSec status is readable again.", false)
	}

	return status
}

// phase1State picks the most favourable IKE SA state for a connection, since a
// rekey can leave a stale SA alongside the live one.
func phase1State(sas []ipsecRawSA) (string, int64) {
	if len(sas) == 0 {
		return ipsecNoPhase1, 0
	}

	var fallback string
	for _, sa := range sas {
		state := strings.ToUpper(strings.TrimSpace(string(sa.State)))
		if state == ipsecEstablished {
			return ipsecEstablished, sa.Established.int64()
		}
		if fallback == "" || state == ipsecConnecting {
			fallback = state
		}
	}
	if fallback == "" {
		fallback = ipsecNoPhase1
	}
	return fallback, 0
}

func evaluateIPSecConnection(conn IPSecConnection) {
	// Key on the stable connection ID; descriptions change in the GUI.
	alarmName := "opnsense_ipsec_" + alarmSuffix(conn.ID)

	peer := conn.RemoteAddrs
	if peer == "" {
		peer = "unknown peer"
	}

	// Clear rather than skip, so newly excluding a connection releases any
	// alarm it is already holding down.
	if conn.Excluded {
		common.AlarmCheckUp(alarmName,
			fmt.Sprintf("IPSec tunnel '%s' is excluded from health checks.", conn.Name), false)
		return
	}

	if conn.Healthy {
		common.AlarmCheckUp(alarmName,
			fmt.Sprintf("IPSec tunnel '%s' (%s) Phase 1 is ESTABLISHED.", conn.Name, peer),
			false)
		return
	}

	detail := conn.State
	if conn.State == ipsecNoPhase1 {
		detail = "no Phase 1 SA"
	}
	common.AlarmCheckDown(alarmName,
		fmt.Sprintf("IPSec tunnel '%s' Phase 1 is not established (%s).\nLocal: %s\nRemote: %s\nIKE: %s",
			conn.Name, detail, orDash(conn.LocalAddrs), orDash(conn.RemoteAddrs), orDash(conn.Version)),
		false, "", "")
}

// ipsecName resolves a connection ID to its GUI description, falling back to
// the raw ID.
func ipsecName(id string, names *opnsenseNames) string {
	if name, ok := names.IPSecNames[id]; ok {
		return name
	}
	return id
}

func isExcludedIPSecConn(conn IPSecConnection) bool {
	for _, ex := range OpnsenseHealthConfig.Ipsec.ExcludedConnections {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}
		if ex == conn.ID || strings.EqualFold(ex, conn.Name) {
			return true
		}
	}
	return false
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
