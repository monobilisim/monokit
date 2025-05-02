package zimbraHealth

import (
"fmt"
"strings"

"github.com/monobilisim/monokit/common"
ui "github.com/monobilisim/monokit/common/ui" // Import the ui package
)

// RenderAll renders all Zimbra health data as a single string for box display.
func (h *ZimbraHealthData) RenderAll() string {
	var sb strings.Builder

	// IP Access Section
	sb.WriteString(common.SectionTitle("Access through IP"))
	sb.WriteString("\n")
	if h.IPAccess.CheckStatus {
		// Use SimpleStatusListItem for IP Access
		expectedState := "Accessible" // Default to the failure state description (red)
		if h.IPAccess.Accessible {    // If true (success state = blocked)
			expectedState = "Blocked" // Use the success state description (green)
		}
		sb.WriteString(common.SimpleStatusListItem(
			"Direct IP Access",
			expectedState,
			h.IPAccess.Accessible, // Success = Blocked (true)
		))
	} else {
		// Use SimpleStatusListItem for failure case
		sb.WriteString(common.SimpleStatusListItem(
			"Direct IP Access",
			"Check Failed",
			false,
		))
		sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.IPAccess.Message))
	}
	sb.WriteString("\n")

	// Zimbra Services Section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Zimbra Services"))
	sb.WriteString("\n")
if len(h.Services) > 0 {
for _, service := range h.Services {
// Use the ServiceStatusListItem from the ui package
sb.WriteString(ui.ServiceStatusListItem(service.Name, service.Running))
sb.WriteString("\n")
}
} else {
		sb.WriteString("  No service status information available.\n")
	}

	// Zimbra Version Check Section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Zimbra Version Check"))
	sb.WriteString("\n")
	if h.Version.CheckStatus {
		expectedState := "Up-to-date"
		if h.Version.UpdateAvailable {
			expectedState = "Update Available" // Or keep "Up-to-date" and rely on color? Let's use "Update Available" for clarity.
		}
		sb.WriteString(common.SimpleStatusListItem(
			"Version Status",
			expectedState,
			!h.Version.UpdateAvailable, // Success if no update is available
		))
		// Optionally add installed version info if needed:
		// sb.WriteString(fmt.Sprintf("  └─ Installed: %s\n", h.Version.InstalledVersion))
	} else {
		sb.WriteString(common.SimpleStatusListItem(
			"Version Status",
			"Check Failed",
			false,
		))
		sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.Version.Message))
	}
	sb.WriteString("\n")

	// Z-Push Section (only if URL is configured)
	if h.ZPush.URL != "" {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Z-Push Status"))
		sb.WriteString("\n")
		if h.ZPush.CheckStatus {
			sb.WriteString(common.SimpleStatusListItem(
				"Z-Push Headers",
				"Detected",
				h.ZPush.HeaderFound,
			))
			sb.WriteString("\n")
			sb.WriteString(common.SimpleStatusListItem(
				"Nginx Config",
				"Present", // Use "Present" instead of "Exists"
				h.ZPush.NginxConfig,
			))
		} else {
			sb.WriteString(common.SimpleStatusListItem(
				"Z-Push Check",
				"Check Failed",
				false,
			))
			sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.ZPush.Message))
		}
		sb.WriteString("\n")
	}

	// Queued Messages Section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Queued Messages"))
	sb.WriteString("\n")
	if h.QueuedMessages.CheckStatus {
		expectedState := fmt.Sprintf("within limit (%d)", h.QueuedMessages.Count)
		if h.QueuedMessages.Exceeded {
			expectedState = fmt.Sprintf("over limit (%d)", h.QueuedMessages.Count)
		}
		sb.WriteString(common.SimpleStatusListItem(
			"Mail Queue",
			expectedState,
			!h.QueuedMessages.Exceeded, // Success if not exceeded
		))
	} else {
		sb.WriteString(common.SimpleStatusListItem(
			"Mail Queue",
			"Check Failed",
			false,
		))
		sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.QueuedMessages.Message))
	}
	sb.WriteString("\n")

	// SSL Expiration Section
	// Only show if check was performed (e.g., based on time or explicit trigger)
	// We assume if MailHost is populated, the check ran.
	if h.SSLCert.MailHost != "" {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("SSL Expiration"))
		sb.WriteString("\n")
		if h.SSLCert.CheckStatus {
			// Use SimpleStatusListItem for SSL as well
			expectedState := fmt.Sprintf("valid (%d days)", h.SSLCert.DaysUntilExpiry)
			if h.SSLCert.ExpiringSoon {
				expectedState = fmt.Sprintf("expiring soon (%d days)", h.SSLCert.DaysUntilExpiry)
			}
			sb.WriteString(common.SimpleStatusListItem(
				"SSL Certificate", // Label includes hostname implicitly via check logic
				expectedState,
				!h.SSLCert.ExpiringSoon, // Success = not expiring soon (green)
			))
		} else {
			// Use SimpleStatusListItem for failure case
			sb.WriteString(common.SimpleStatusListItem(
				"SSL Certificate",
				"Check Failed",
				false,
			))
			sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.SSLCert.Message))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
