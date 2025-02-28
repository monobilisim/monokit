//go:build !with_api

package common

import (
	"github.com/spf13/cobra"
)

// StartAPIServer is a no-op when building without API support
func StartAPIServer(cmd *cobra.Command, args []string) error {
	return nil
}
