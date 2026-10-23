package common

import "testing"

func TestZPushShouldAlarm(t *testing.T) {
	tests := []struct {
		name            string
		latest          string
		current         string
		alreadyNotified string
		want            bool
	}{
		{"newer release is announced", "2.7.7", "2.7.6", "", true},
		{"already announced release stays silent", "2.7.7", "2.7.6", "2.7.7", false},
		{"installed is up to date", "2.7.6", "2.7.6", "", false},
		{"installed is ahead of upstream", "2.7.5", "2.7.6", "", false},
		{"patch bump is announced", "2.7.10", "2.7.9", "", true},
		{"announced version older than latest is re-announced", "2.7.8", "2.7.6", "2.7.7", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zpushShouldAlarm(tt.latest, tt.current, tt.alreadyNotified); got != tt.want {
				t.Errorf("zpushShouldAlarm(%q, %q, %q) = %v, want %v", tt.latest, tt.current, tt.alreadyNotified, got, tt.want)
			}
		})
	}
}
