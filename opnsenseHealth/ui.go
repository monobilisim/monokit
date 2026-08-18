package opnsenseHealth

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3B8CC"))
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Underline(true)
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true)
)

func RenderOpnsenseHealthCLI(data *OpnsenseHealthData) string {
	var b strings.Builder

	renderSSLSection(&b, data)

	if data.WireGuard != nil {
		renderWireGuardSection(&b, data.WireGuard)
	}
	if data.IPSec != nil {
		renderIPSecSection(&b, data.IPSec)
	}
	if data.Gateways != nil {
		renderGatewaySection(&b, data.Gateways)
	}
	if data.DNS != nil {
		renderDNSSection(&b, data.DNS)
	}

	return b.String()
}

func renderSSLSection(b *strings.Builder, data *OpnsenseHealthData) {
	b.WriteString(titleStyle.Render("SSL Certificate Information") + "\n\n")

	b.WriteString(field("Subject:", data.Subject))
	b.WriteString(field("Issuer:", data.Issuer))
	b.WriteString(field("Expiry Date:", data.ExpiryDate))
	b.WriteString(field("Days Remaining:", fmt.Sprintf("%d days", data.DaysRemaining)))

	statusStyle := successStyle
	switch data.Status {
	case "Expiring Soon":
		statusStyle = warningStyle
	case "Expired", "Connection Failed", "No Certificate Found":
		statusStyle = errorStyle
	}
	b.WriteString(fmt.Sprintf("%s %s\n", textStyle.Render("Status:"), statusStyle.Render(data.Status)))
}

func renderWireGuardSection(b *strings.Builder, wg *WireGuardStatus) {
	section(b, "WireGuard")

	if wg.Error != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render("Error:"), errorStyle.Render(wg.Error)))
		return
	}
	if len(wg.Interfaces) == 0 {
		b.WriteString("  " + textStyle.Render("No WireGuard interfaces configured") + "\n")
		return
	}

	for _, iface := range wg.Interfaces {
		var state string
		style := successStyle
		switch {
		case iface.Missing:
			state, style = "MISSING", errorStyle
		case !iface.Up:
			state, style = "DOWN", errorStyle
		case len(iface.ProblemPeers) > 0:
			state, style = fmt.Sprintf("%d/%d peers without tunnel", len(iface.ProblemPeers), iface.PeerCount), errorStyle
		default:
			state = fmt.Sprintf("UP, %d peer(s) OK", iface.PeerCount)
		}

		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render(iface.Name+":"), style.Render(state)))

		for _, p := range iface.ProblemPeers {
			name := p.Name
			if name == "" {
				name = shortKey(p.PublicKey)
			}
			b.WriteString(fmt.Sprintf("      %s %s\n",
				textStyle.Render("• "+name),
				warningStyle.Render(fmt.Sprintf("%s (last handshake: %s)", p.Problem, humanAge(p.HandshakeAge)))))
		}
		var notes []string
		if iface.ExcludedPeers > 0 {
			notes = append(notes, fmt.Sprintf("%d faulty peer(s) silenced by config", iface.ExcludedPeers))
		}
		if iface.IdlePeers > 0 {
			notes = append(notes, fmt.Sprintf("%d dial-in peer(s) idle", iface.IdlePeers))
		}
		if len(notes) > 0 {
			b.WriteString(fmt.Sprintf("      %s\n", textStyle.Render(strings.Join(notes, ", "))))
		}
	}
}

func renderIPSecSection(b *strings.Builder, ipsec *IPSecStatus) {
	section(b, "IPSec")

	if ipsec.Error != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render("Error:"), errorStyle.Render(ipsec.Error)))
		return
	}
	if len(ipsec.Connections) == 0 {
		b.WriteString("  " + textStyle.Render("No IPSec connections loaded") + "\n")
		return
	}

	for _, conn := range ipsec.Connections {
		style := errorStyle
		switch {
		case conn.Healthy:
			style = successStyle
		case conn.State == ipsecConnecting:
			style = warningStyle
		}

		state := conn.State
		if conn.Excluded {
			state += " (excluded)"
			style = textStyle
		}
		if conn.RemoteAddrs != "" {
			state += " → " + conn.RemoteAddrs
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render(conn.Name+":"), style.Render(state)))
	}
}

func renderGatewaySection(b *strings.Builder, gws *GatewayStatus) {
	section(b, fmt.Sprintf("Gateways (loss limit: %.1f%%)", OpnsenseHealthConfig.Gateway.LossLimit))

	if gws.Error != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render("Error:"), errorStyle.Render(gws.Error)))
		return
	}
	if len(gws.Gateways) == 0 {
		b.WriteString("  " + textStyle.Render("No gateways configured") + "\n")
		return
	}

	for _, gw := range gws.Gateways {
		style := errorStyle
		detail := gw.Reason
		switch {
		case gw.Skipped:
			style = textStyle
			detail = gw.Reason
		case gw.Healthy:
			style = successStyle
			detail = fmt.Sprintf("%s (loss %s, delay %s)", orDash(gw.StatusText), orDash(gw.Loss), orDash(gw.Delay))
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render(gw.Name+":"), style.Render(detail)))
	}
}

func renderDNSSection(b *strings.Builder, dns *DNSStatus) {
	section(b, "DNS Resolver")

	if dns.Healthy {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			textStyle.Render(dns.Server+":"),
			successStyle.Render(fmt.Sprintf("resolved %s in %dms", dns.Query, dns.DurationMs))))
		return
	}
	b.WriteString(fmt.Sprintf("  %s %s\n",
		textStyle.Render(dns.Server+":"),
		errorStyle.Render(fmt.Sprintf("could not resolve %s (%s)", dns.Query, dns.Error))))
}

func section(b *strings.Builder, title string) {
	b.WriteString("\n" + titleStyle.Render(title) + "\n\n")
}

func field(label, value string) string {
	return fmt.Sprintf("%s %s\n", textStyle.Render(label), valueStyle.Render(value))
}
