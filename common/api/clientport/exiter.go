package clientport

import "os"

// OSExiter implements Exiter using os.Exit.
type OSExiter struct{}

func (OSExiter) Exit(code int) {
	os.Exit(code)
}
