package opnsenseHealth

import "testing"

func TestWgIfaceLabel(t *testing.T) {
	cases := []struct {
		name string
		desc string
		want string
	}{
		{"wg6", "ist1-ferrotech", "wg6 (ist1-ferrotech)"},
		{"wg6", "", "wg6"},
		// OPNsense defaults an unnamed instance to the device name; the label
		// must not render that as "wg6 (wg6)".
		{"wg6", "wg6", "wg6"},
	}
	for _, c := range cases {
		iface := &WireGuardInterface{Name: c.name, Description: c.desc}
		if got := wgIfaceLabel(iface); got != c.want {
			t.Errorf("wgIfaceLabel(name=%q, desc=%q) = %q, want %q", c.name, c.desc, got, c.want)
		}
	}
}
