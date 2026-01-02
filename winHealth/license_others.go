//go:build !windows

package winHealth

func GetWindowsLicenseStatus() LicenseInfo {
	return LicenseInfo{
		Status:        "N/A",
		Description:   "Not supported on this OS",
		IsLicensed:    false,
		RemainingDays: -1,
	}
}
