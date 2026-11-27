package osHealth

import "testing"

func TestDiskExceededHysteresis(t *testing.T) {
	const limit = 93.0
	cases := []struct {
		name        string
		used        float64
		wasExceeded bool
		want        bool
	}{
		{"below limit, not alarmed", 92.96, false, false},
		{"reaches limit", 93.0, false, true},
		{"just under limit while alarmed stays exceeded", 92.96, true, true},
		{"inside hysteresis band while alarmed", 92.0, true, true},
		{"drops below band clears", 91.99, true, false},
		{"band does not open a new alarm", 92.5, false, false},
	}
	for _, c := range cases {
		if got := diskExceeded(c.used, limit, c.wasExceeded); got != c.want {
			t.Errorf("%s: diskExceeded(%v, %v, %v) = %v, want %v", c.name, c.used, limit, c.wasExceeded, got, c.want)
		}
	}
}

// A partition oscillating around the limit, as /data on hepiyi-prod-mongodb-main1
// did on 2026-11-25, must stay exceeded instead of closing and reopening.
func TestDiskExceededOscillationStaysOpen(t *testing.T) {
	samples := []float64{93.01, 92.96, 93.02, 92.97, 93.0, 92.95}
	state := false
	for i, s := range samples {
		state = diskExceeded(s, 93, state)
		if !state {
			t.Fatalf("sample %d (%.2f%%) closed the alarm", i, s)
		}
	}
}
