//go:build !windows

package winHealth

// GetWindowsServices returns windows services (Windows only)
func GetWindowsServices() ([]WindowsServiceInfo, error) {
	// For non-Windows systems return empty array
	return []WindowsServiceInfo{}, nil
}
