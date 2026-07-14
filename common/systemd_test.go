//go:build linux

package common

import "testing"

// TestSystemdUnitActive_UnknownUnit is a non-destructive sanity check that
// the socket-activation fallback added to SystemdUnitActive doesn't produce
// false positives for units (and their ".socket" companion) that don't
// exist at all.
//
// The fallback itself (ssh.service inactive + ssh.socket active -> true)
// was manually verified against docker.service/docker.socket on a live
// systemd host: `systemctl stop docker.service` leaves docker.socket
// active, and SystemdUnitActive("docker.service") correctly returned true
// via the fallback. Not committed as an automated test because it requires
// root and mutates a real system service.
func TestSystemdUnitActive_UnknownUnit(t *testing.T) {
	if SystemdUnitActive("this-unit-does-not-exist-xyz123.service") {
		t.Fatal("expected false for a unit and socket that don't exist")
	}
}
