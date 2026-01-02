//go:build windows

package winHealth

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/yusufpapurcu/wmi"
)

// SoftwareLicensingProduct represents the WMI class for activation info.
// We only fetch fields we care about.
type SoftwareLicensingProduct struct {
	Name                 string
	Description          string
	LicenseStatus        uint32
	ApplicationID        string
	PartialProductKey    string
	GracePeriodRemaining uint32
}

// GetWindowsLicenseStatus checks Windows activation status via WMI
func GetWindowsLicenseStatus() LicenseInfo {
	var licenses []SoftwareLicensingProduct

	// Query for Windows OS licenses only (ApplicationID for Windows is 55c92734-d682-4d71-983e-d6ec3f16059f)
	// and only those that have a partial product key (filters out some auxiliary licenses)
	query := "SELECT Name, Description, LicenseStatus, ApplicationID, PartialProductKey, GracePeriodRemaining FROM SoftwareLicensingProduct WHERE ApplicationID = '55c92734-d682-4d71-983e-d6ec3f16059f' AND PartialProductKey <> null"

	err := wmi.Query(query, &licenses)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query Windows License info via WMI")
		return LicenseInfo{
			Status:      "Unknown",
			Description: "Error querying WMI",
			IsLicensed:  false,
		}
	}

	if len(licenses) == 0 {
		// Fallback or just report unknown
		return LicenseInfo{
			Status:      "Unknown",
			Description: "No active Windows license found",
			IsLicensed:  false,
		}
	}

	// We might find multiple (e.g. various editions). Usually the active one is what we want.
	// We look for one with LicenseStatus = 1 (Licensed).
	// If not found, show the first non-licensed one (e.g. Notification mode).
	var bestLicense SoftwareLicensingProduct
	found := false

	for _, lic := range licenses {
		if lic.LicenseStatus == 1 {
			bestLicense = lic
			found = true
			break
		}
	}

	if !found {
		bestLicense = licenses[0]
	}

	info := LicenseInfo{
		Description: bestLicense.Name, // Often "Windows(R) Operating System..."
		IsLicensed:  bestLicense.LicenseStatus == 1,
	}

	// Map status codes
	// 0=Unlicensed, 1=Licensed, 2=OOBGrace, 3=OOTGrace, 4=NonGenuineGrace, 5=Notification, 6=ExtendedGrace
	switch bestLicense.LicenseStatus {
	case 0:
		info.Status = "Unlicensed"
	case 1:
		info.Status = "Licensed"
	case 2:
		info.Status = "OOB Grace"
	case 3:
		info.Status = "OOT Grace"
	case 4:
		info.Status = "Non-Genuine Grace"
	case 5:
		info.Status = "Notification"
	case 6:
		info.Status = "Extended Grace"
	default:
		info.Status = fmt.Sprintf("Unknown (%d)", bestLicense.LicenseStatus)
	}

	// Set Remaining time
	info.RemainingDays = -1 // Default to -1 (Permanent or Unknown)

	if bestLicense.GracePeriodRemaining > 0 {
		days := bestLicense.GracePeriodRemaining / (60 * 24)
		info.Remaining = fmt.Sprintf("%d days", days)
		info.RemainingDays = int(days)
		// If less than 1 day
		if days == 0 {
			hours := bestLicense.GracePeriodRemaining / 60
			info.Remaining = fmt.Sprintf("%d hours", hours)
			info.RemainingDays = 0
		}
	} else {
		if info.IsLicensed {
			info.Remaining = "Permanent"
			info.RemainingDays = -1
		} else {
			info.Remaining = "Expired/Invalid"
			info.RemainingDays = 0
		}
	}

	// Clean up description
	if info.Description == "" {
		info.Description = bestLicense.Description
	} else {
		// sometimes Name contains the full description
		// e.g. "Windows(R), ServerStandard edition"
		// keeping it as is or truncating if too long
	}

	return info
}
