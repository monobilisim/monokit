//go:build windows

package daemon

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const serviceName = "monokit-daemon"
const serviceDesc = "Monokit Daemon Service for running health checks continuously"

type daemonService struct{}

func (m *daemonService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	fmt.Println("Service execution started")

	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	go func() {
		freq := DaemonConfig.Frequency
		if freq <= 0 {
			freq = 60
		}

		fmt.Printf("Starting daemon loop with frequency %d\n", freq)
		ticker := time.NewTicker(time.Duration(freq) * time.Second)
		defer ticker.Stop()

		RunAll()

		for range ticker.C {
			RunAll()
		}
	}()

loop:
	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
			time.Sleep(100 * time.Millisecond)
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			fmt.Println("Received stop/shutdown request")
			break loop
		default:
			fmt.Printf("unexpected control request #%d\n", c)
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	fmt.Println("Service execution stopping")
	return
}

func isWindowsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isService
}

func runWindowsService() {
	if err := svc.Run(serviceName, &daemonService{}); err != nil {
		fmt.Printf("Service %s failed: %v\n", serviceName, err)
	}
}

var InstallServiceCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Monokit Daemon as a Windows Service",
	Run: func(cmd *cobra.Command, args []string) {
		m, err := mgr.Connect()
		if err != nil {
			fmt.Printf("Failed to connect to service manager: %v\n", err)
			return
		}
		defer m.Disconnect()

		s, err := m.OpenService(serviceName)
		if err == nil {
			s.Close()
			fmt.Printf("Service %s already exists\n", serviceName)
			return
		}

		exePath, err := os.Executable()
		if err != nil {
			fmt.Printf("Failed to get executable path: %v\n", err)
			return
		}

		s, err = m.CreateService(serviceName, exePath, mgr.Config{
			DisplayName: serviceName,
			Description: serviceDesc,
			StartType:   mgr.StartAutomatic,
		}, "daemon")
		if err != nil {
			fmt.Printf("Failed to create service: %v\n", err)
			return
		}
		defer s.Close()
		fmt.Printf("Service %s installed successfully.\n", serviceName)
	},
}

var RemoveServiceCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove Monokit Daemon Windows Service",
	Run: func(cmd *cobra.Command, args []string) {
		m, err := mgr.Connect()
		if err != nil {
			fmt.Printf("Failed to connect to service manager: %v\n", err)
			return
		}
		defer m.Disconnect()

		s, err := m.OpenService(serviceName)
		if err != nil {
			fmt.Printf("Service %s is not installed\n", serviceName)
			return
		}
		defer s.Close()

		err = s.Delete()
		if err != nil {
			fmt.Printf("Failed to remove service: %v\n", err)
			return
		}
		fmt.Printf("Service %s removed successfully.\n", serviceName)
	},
}

var StartServiceCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Monokit Daemon Windows Service",
	Run: func(cmd *cobra.Command, args []string) {
		m, err := mgr.Connect()
		if err != nil {
			fmt.Printf("Failed to connect to service manager: %v\n", err)
			return
		}
		defer m.Disconnect()

		s, err := m.OpenService(serviceName)
		if err != nil {
			fmt.Printf("Service %s is not installed\n", serviceName)
			return
		}
		defer s.Close()

		err = s.Start()
		if err != nil {
			fmt.Printf("Failed to start service: %v\n", err)
			return
		}
		fmt.Printf("Service %s started successfully.\n", serviceName)
	},
}

var StopServiceCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Monokit Daemon Windows Service",
	Run: func(cmd *cobra.Command, args []string) {
		m, err := mgr.Connect()
		if err != nil {
			fmt.Printf("Failed to connect to service manager: %v\n", err)
			return
		}
		defer m.Disconnect()

		s, err := m.OpenService(serviceName)
		if err != nil {
			fmt.Printf("Service %s is not installed\n", serviceName)
			return
		}
		defer s.Close()

		status, err := s.Control(svc.Stop)
		if err != nil {
			fmt.Printf("Failed to stop service: %v\n", err)
			return
		}
		fmt.Printf("Service %s is stopping (status: %v)...\n", serviceName, status.State)
	},
}
