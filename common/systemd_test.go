//go:build linux

package common

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSystemdUnitExists_SysVInitScript covers packages that ship only a SysV
// init script (Percona XtraDB Cluster 5.7 provides /etc/init.d/mysql and no
// unit file): systemctl manages the generated mysql.service, so upCheck must
// not report the service as MISSING.
func TestSystemdUnitExists_SysVInitScript(t *testing.T) {
	const unit = "monokit-sysvtest.service"

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "monokit-sysvtest"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	old := SysVInitDir
	SysVInitDir = dir
	defer func() { SysVInitDir = old }()

	if !SystemdUnitExists(unit) {
		t.Fatal("expected true when a SysV init script for the unit exists")
	}
	if SystemdUnitExists("this-unit-does-not-exist-xyz123.service") {
		t.Fatal("expected false when neither a unit file nor an init script exists")
	}
}

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
