package opnsenseHealth

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const cmdTimeout = 15 * time.Second

// resolveBinary returns a usable path for name, preferring $PATH and falling
// back to the absolute locations OPNsense installs into. An empty result means
// the tool is not present, which is how the callers detect a disabled feature.
func resolveBinary(name string, fallbacks ...string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, f := range fallbacks {
		if info, err := os.Stat(f); err == nil && !info.IsDir() {
			return f
		}
	}
	return ""
}

// runCmd runs bin with a timeout and folds stderr into the returned error so a
// failing OPNsense script explains itself in the alarm message.
func runCmd(bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			err = fmt.Errorf("%w: %s", err, msg)
		}
		return out, err
	}
	return out, nil
}

// trimToJSON drops anything before the first JSON object or array. The PHP CLI
// writes deprecation notices to stdout rather than stderr, and Python warnings
// can do the same, so a strict decode of raw stdout would turn an OPNsense
// upgrade into a fleet-wide false alarm. Returns nil when there is no JSON.
func trimToJSON(out []byte) []byte {
	for i, c := range out {
		if c == '{' || c == '[' {
			return out[i:]
		}
	}
	return nil
}

// renderTable renders a Markdown-style table for use in alarm messages.
func renderTable(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len([]rune(h))
	}
	for _, row := range rows {
		for i := 0; i < len(widths) && i < len(row); i++ {
			if l := len([]rune(row[i])); l > widths[i] {
				widths[i] = l
			}
		}
	}

	pad := func(s string, width int) string {
		if diff := width - len([]rune(s)); diff > 0 {
			return s + strings.Repeat(" ", diff)
		}
		return s
	}

	writeRow := func(b *strings.Builder, cells []string) {
		b.WriteString("|")
		for i := range headers {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			b.WriteString(" " + pad(cell, widths[i]) + " |")
		}
		b.WriteString("\n")
	}

	var b strings.Builder
	writeRow(&b, headers)

	b.WriteString("|")
	for i := range headers {
		b.WriteString(" " + strings.Repeat("-", widths[i]) + " |")
	}
	b.WriteString("\n")

	for _, row := range rows {
		writeRow(&b, row)
	}
	return b.String()
}

// alarmSuffix normalizes an identifier so it is safe and stable as part of an
// alarm key.
func alarmSuffix(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// shortKey truncates a WireGuard public key for display.
func shortKey(k string) string {
	if len(k) <= 12 {
		return k
	}
	return k[:12] + "..."
}

// humanAge formats a handshake age for alarm messages.
func humanAge(seconds int64) string {
	if seconds < 0 {
		return "never"
	}
	d := time.Duration(seconds) * time.Second
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", seconds)
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}
