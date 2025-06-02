package clientport

import (
	"io/fs"
	"os"
)

// OSFS implements FS for the actual filesystem.
type OSFS struct{}

func (OSFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (OSFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (OSFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}
