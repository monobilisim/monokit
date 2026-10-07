package healthdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// ErrLockTimeout is returned by Lock when another process kept the lock for
// the whole timeout.
var ErrLockTimeout = errors.New("healthdb: timed out waiting for lock")

var unsafeLockChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// Lock takes an exclusive lock on (module, key) that is shared by every
// monokit process using this database, waiting up to timeout for it.
//
// Use it around read-decide-act sequences that span external calls, e.g.
// "no issue ID stored -> create the issue in Redmine -> store its ID".
// Overlapping runs of the same check otherwise all see the empty state and
// all act on it. The lock is held by the OS on an open file, so it is released
// when the process exits or dies; the lock file itself is left in place.
//
// Locks are per open file, not per process: taking the same lock twice in one
// process without unlocking in between waits on itself until timeout.
func Lock(module, key string, timeout time.Duration) (unlock func(), err error) {
	dir := filepath.Join(filepath.Dir(getDefaultDBPath()), "locks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("healthdb: create lock dir: %w", err)
	}
	path := filepath.Join(dir, unsafeLockChars.ReplaceAllString(module+"-"+key, "_")+".lock")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("healthdb: open lock file: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		ok, err := tryLockFile(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("healthdb: lock %s: %w", path, err)
		}
		if ok {
			return func() {
				unlockFile(f)
				f.Close()
			}, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, ErrLockTimeout
		}
		time.Sleep(100 * time.Millisecond)
	}
}
