//go:build linux

package postalHealth

import (
	"testing"
	"time"
)

func TestEvalExpiry(t *testing.T) {
	cases := []struct {
		name         string
		daysFromNow  int
		threshold    int
		wantExpiring bool
	}{
		{"already expired", -1, 10, true},
		{"below threshold", 5, 10, true},
		{"just above threshold", 11, 10, false},
		{"healthy", 30, 10, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			notAfter := time.Now().Add(time.Duration(c.daysFromNow) * 24 * time.Hour)
			_, expiringSoon := evalExpiry(notAfter, c.threshold)
			if expiringSoon != c.wantExpiring {
				t.Errorf("evalExpiry(%d days, threshold %d) expiringSoon = %v, want %v", c.daysFromNow, c.threshold, expiringSoon, c.wantExpiring)
			}
		})
	}
}
