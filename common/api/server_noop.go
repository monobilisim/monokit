//go:build !with_api

package common

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ServerMain is a no-op function when API is not included
func ServerMain(cmd *cobra.Command, args []string) {
	fmt.Println("API server is not included in this build. Please rebuild with -tags with_api to use this feature.")
}
