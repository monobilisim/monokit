//go:build windows

package winHealth

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// GetWindowsServices returns windows services (Windows only)
func GetWindowsServices() ([]WindowsServiceInfo, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	names, err := m.ListServices()
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var serviceInfos []WindowsServiceInfo
	for _, name := range names {
		s, err := m.OpenService(name)
		if err != nil {
			continue
		}

		status, err := s.Query()
		if err != nil {
			s.Close()
			continue
		}

		// Get config to see Display Name and Start Type
		cfg, err := s.Config()
		s.Close()
		if err != nil {
			continue
		}

		// Filter criteria: match systemd logic "active" or "failed"
		// For Windows, "active" -> Running.
		// "failed" -> Stopped with non-zero exit code (hard to check easily without tracking, but we can check if Auto start and Stopped)

		isActive := status.State == windows.SERVICE_RUNNING || status.State == windows.SERVICE_START_PENDING
		isFailed := status.State == windows.SERVICE_STOPPED && cfg.StartType == mgr.StartAutomatic

		if isActive || isFailed {
			state := "stopped"
			if isActive {
				state = "running"
			}

			statusStr := fmt.Sprintf("%d", status.State)
			if isFailed {
				statusStr = "failed (auto start but stopped)"
			}

			// Use Display Name if available
			dispName := cfg.DisplayName
			if dispName == "" {
				dispName = name
			}

			serviceInfos = append(serviceInfos, WindowsServiceInfo{
				Name:        name,
				DisplayName: dispName,
				Status:      statusStr,
				State:       state,
			})
		}
	}

	return serviceInfos, nil
}

// getVolumeKind resolves the drive type of a mountpoint such as "E:" through
// the Windows API. gopsutil reports every drive letter it can read, including
// CD-ROM, removable and network drives, and its PartitionStat carries no
// drive type, so the lookup is repeated here.
func getVolumeKind(mountpoint string) volumeKind {
	root := mountpoint
	if !strings.HasSuffix(root, "\\") && !strings.HasSuffix(root, "/") {
		root += "\\" // GetDriveType expects a root directory, e.g. "E:\"
	}

	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return volumeUnknown
	}

	switch windows.GetDriveType(rootPtr) {
	case windows.DRIVE_FIXED:
		return volumeFixed
	case windows.DRIVE_REMOVABLE:
		return volumeRemovable
	case windows.DRIVE_CDROM:
		return volumeCdrom
	case windows.DRIVE_REMOTE:
		return volumeNetwork
	case windows.DRIVE_RAMDISK:
		return volumeRamdisk
	default:
		return volumeUnknown
	}
}
