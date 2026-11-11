//go:build linux

package postalHealth

import "testing"

func TestEvaluateHeld(t *testing.T) {
	cases := []struct {
		name        string
		reasons     map[string]int
		threshold   int
		wantHealthy bool
		wantAlarmed int
	}{
		{"suppression only never alarms", map[string]int{heldSuppression: 40000, heldQuarantine: 500, heldRouteHold: 500}, 100, true, 0},
		{"single send limit hold alarms below threshold", map[string]int{heldSendLimit: 1}, 100, false, 1},
		{"single suspension hold alarms below threshold", map[string]int{heldSuspended: 1}, 100, false, 1},
		{"credential below threshold", map[string]int{heldCredential: 60, heldDevelopment: 39}, 100, true, 99},
		{"credential plus unknown at threshold", map[string]int{heldCredential: 60, heldOther: 40}, 100, false, 100},
		{"unconfigured threshold disables thresholded reasons", map[string]int{heldDevelopment: 5000}, 0, true, 5000},
		{"unconfigured threshold keeps send limit alarm", map[string]int{heldSendLimit: 1}, 0, false, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			healthy, alarmed := evaluateHeld(c.reasons, c.threshold)
			if healthy != c.wantHealthy || alarmed != c.wantAlarmed {
				t.Errorf("evaluateHeld(%v, %d) = (%v, %d), want (%v, %d)", c.reasons, c.threshold, healthy, alarmed, c.wantHealthy, c.wantAlarmed)
			}
		})
	}
}
