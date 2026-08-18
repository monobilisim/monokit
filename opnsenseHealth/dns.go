package opnsenseHealth

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

const dnsTimeout = 5 * time.Second

// resolveDNSServer picks the resolver address to probe. An explicit
// dns.server in opnsense.yaml always wins; otherwise Unbound's own configured
// port is used, so a resolver moved off 53 is not mistaken for a dead one.
func resolveDNSServer(names *opnsenseNames) string {
	if s := strings.TrimSpace(OpnsenseHealthConfig.Dns.Server); s != "" {
		return s
	}
	if names.UnboundPort != "" {
		return net.JoinHostPort("127.0.0.1", names.UnboundPort)
	}
	return defaultDNSServer
}

// collectDNSHealth probes the local resolver (Unbound by default) with a real
// query, which is the only way to tell that it is actually answering rather
// than merely running. It returns nil when config.xml says Unbound is off.
func collectDNSHealth(names *opnsenseNames) *DNSStatus {
	if names.UnboundDisabled {
		log.Debug().Msg("Unbound is disabled in config.xml, skipping DNS check")
		return nil
	}

	server := resolveDNSServer(names)
	query := OpnsenseHealthConfig.Dns.Query

	status := &DNSStatus{Server: server, Query: query}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: dnsTimeout}).DialContext(ctx, network, server)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()

	start := time.Now()
	addrs, err := resolver.LookupHost(ctx, query)
	status.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		status.Healthy = false
		status.Error = err.Error()
		common.AlarmCheckDown("opnsense_dns",
			fmt.Sprintf("DNS resolution failed: %s could not resolve '%s' (%s).", server, query, err.Error()),
			false, "", "")
		return status
	}

	if len(addrs) == 0 {
		status.Healthy = false
		status.Error = "no records returned"
		common.AlarmCheckDown("opnsense_dns",
			fmt.Sprintf("DNS resolution failed: %s returned no records for '%s'.", server, query),
			false, "", "")
		return status
	}

	status.Healthy = true
	status.Resolved = addrs
	common.AlarmCheckUp("opnsense_dns",
		fmt.Sprintf("DNS resolution is working: %s resolved '%s' in %dms.", server, query, status.DurationMs),
		false)

	return status
}
