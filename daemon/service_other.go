//go:build !windows

package daemon

import (
	"fmt"

	"github.com/spf13/cobra"
)

func isWindowsService() bool {
	return false
}

func runWindowsService() {
}

var InstallServiceCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Monokit Daemon as a Windows Service",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Windows service installation is only supported on Windows.")
	},
}

var RemoveServiceCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove Monokit Daemon Windows Service",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Windows service removal is only supported on Windows.")
	},
}

var StartServiceCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Monokit Daemon Windows Service",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Windows service start is only supported on Windows.")
	},
}

var StopServiceCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Monokit Daemon Windows Service",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Windows service stop is only supported on Windows.")
	},
}
