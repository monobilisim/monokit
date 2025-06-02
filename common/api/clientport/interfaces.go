package clientport

import (
	"io/fs"
	"net/http"
)

// HTTPDoer is an interface to abstract http.Client for testability.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// SysInfo abstracts OS and platform-related queries.
type SysInfo interface {
	CPUCores() int
	RAM() string
	PrimaryIP() string
	OSPlatform() string
}

// FS abstracts filesystem operations for testability.
type FS interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
}

// Exiter lets us stub out os.Exit.
type Exiter interface {
	Exit(code int)
}
