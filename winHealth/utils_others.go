//go:build !windows

package winHealth

// GetWindowsServices returns windows services (Windows only)
func GetWindowsServices() ([]WindowsServiceInfo, error) {
	// For non-Windows systems return empty array
	return []WindowsServiceInfo{}, nil
}

// getVolumeKind is a no-op outside Windows, where drive letters and drive
// types do not exist. Every volume is reported as fixed storage, leaving the
// mount options as the only thing skipVolume can act on.
func getVolumeKind(_ string) volumeKind {
	return volumeFixed
}
