//go:build !windows

package healthdb

import (
	"errors"
	"os"
	"syscall"
)

// tryLockFile takes a non-blocking flock on f. ok is false when another open
// file holds it.
func tryLockFile(f *os.File) (ok bool, err error) {
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

func unlockFile(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
