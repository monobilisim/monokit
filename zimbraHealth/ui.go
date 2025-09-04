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

	// Cache Information Section (if caching is enabled)
	if h.CacheInfo.Enabled {
		sb.WriteString(common.SectionTitle("Cache Information"))
		sb.WriteString("\n")

		// Cache status
		cacheStatus := "Fresh Data"
		if h.CacheInfo.FromCache {
			cacheStatus = "Cached Data"
		}
		sb.WriteString(common.SimpleStatusListItem(
			"Data Source",
			cacheStatus,
			true, // Always green for display info
		))

		// Cache interval
		sb.WriteString(common.SimpleStatusListItem(
			"Cache Interval",
			fmt.Sprintf("%d hours", h.CacheInfo.CacheInterval),
			true, // Always green for display info
		))

		// Last full check
		if h.CacheInfo.LastFullCheck != "" {
			sb.WriteString(fmt.Sprintf("  └─ Last full check: %s\n", h.CacheInfo.LastFullCheck))
		}

		// Next full check
		if h.CacheInfo.NextFullCheck != "" {
			sb.WriteString(fmt.Sprintf("  └─ Next full check: %s\n", h.CacheInfo.NextFullCheck))
		}

		sb.WriteString("\n")
	}

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
		sb.WriteString(fmt.Sprintf("\n        └─ Error: %s", h.IPAccess.Message))
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
			sb.WriteString(fmt.Sprintf("\n        └─ Error: %s", h.SSLCert.Message))
		}
		sb.WriteString("\n")
	}

	// Hosts File Monitoring Section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Hosts File Monitoring"))
	sb.WriteString("\n")
	if h.HostsFile.CheckStatus {
		expectedState := "No changes"
		if h.HostsFile.HasChanges {
			expectedState = "Changes detected"
		}
		sb.WriteString(common.SimpleStatusListItem(
			"File Status",
			expectedState,
			!h.HostsFile.HasChanges, // Success = no changes (green)
		))
		sb.WriteString("\n")

		// Show backup status
		backupState := "Not created"
		if h.HostsFile.BackupExists {
			backupState = "Created"
		}
		sb.WriteString(common.SimpleStatusListItem(
			"Backup Status",
			backupState,
			h.HostsFile.BackupExists, // Success = backup exists (green)
		))

		// Show last checked time if available
		if h.HostsFile.LastChecked != "" {
			sb.WriteString(fmt.Sprintf("\n        └─ Last checked: %s", h.HostsFile.LastChecked))
		}
	} else {
		sb.WriteString(common.SimpleStatusListItem(
			"Hosts File Check",
			"Check Failed",
			false,
		))
		sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.HostsFile.Message))
	}
	sb.WriteString("\n")

	// Login Test Section (only if enabled)
	if h.LoginTest.Enabled {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Login Test"))
		sb.WriteString("\n")
		if h.LoginTest.CheckStatus {
			expectedState := "Failed"
			if h.LoginTest.LoginSuccessful {
				expectedState = "Successful"
			}
			sb.WriteString(common.SimpleStatusListItem(
				fmt.Sprintf("Login (%s)", h.LoginTest.Username),
				expectedState,
				h.LoginTest.LoginSuccessful, // Success = login successful (green)
			))
			sb.WriteString("\n")

			// Show mailbox size info if available
			if h.LoginTest.LastMailSubject != "" && h.LoginTest.LastMailDate != "" {
				// Check if there's a warning in the message
				isWarning := strings.Contains(h.LoginTest.Message, "mailbox access issue")
				statusColor := !isWarning // Green if no warning, yellow/red if warning

				sb.WriteString(common.SimpleStatusListItem(
					"Mailbox Size",
					"Retrieved",
					statusColor,
				))
				sb.WriteString(fmt.Sprintf("  └─ %s\n", h.LoginTest.LastMailDate))

				// Show warning message if present
				if isWarning {
					sb.WriteString(fmt.Sprintf("  └─ Warning: %s\n", strings.TrimPrefix(h.LoginTest.Message, "Login successful, but ")))
				}
			} else if h.LoginTest.LoginSuccessful {
				sb.WriteString(common.SimpleStatusListItem(
					"Mailbox Access",
					"Verified",
					true, // Still green since login worked
				))
			}
		} else {
			sb.WriteString(common.SimpleStatusListItem(
				fmt.Sprintf("Login (%s)", h.LoginTest.Username),
				"Check Failed",
				false,
			))
			sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.LoginTest.Message))
		}
		sb.WriteString("\n")
	}

	// Email Send Test Section (only if enabled)
	if h.EmailSendTest.Enabled {
		sb.WriteString("\n")
		sectionTitle := "Email Send Test"
		if h.EmailSendTest.ForcedByEnv {
			sectionTitle += " (Forced by Environment Variable)"
		}
		sb.WriteString(common.SectionTitle(sectionTitle))
		sb.WriteString("\n")
		if h.EmailSendTest.CheckStatus {
			expectedState := "Failed"
			if h.EmailSendTest.SendSuccess {
				expectedState = "Successful"
			}
			sb.WriteString(common.SimpleStatusListItem(
				fmt.Sprintf("Send (%s → %s)", h.EmailSendTest.FromEmail, h.EmailSendTest.ToEmail),
				expectedState,
				h.EmailSendTest.SendSuccess, // Success = email sent successfully (green)
			))
			sb.WriteString("\n")

			// Show SMTP server info
			smtpInfo := fmt.Sprintf("%s:%d", h.EmailSendTest.SMTPServer, h.EmailSendTest.SMTPPort)
			if h.EmailSendTest.UseTLS {
				smtpInfo += " (TLS)"
			}
			sb.WriteString(common.SimpleStatusListItem(
				"SMTP Server",
				smtpInfo,
				true, // Always green for display info
			))

			// Show sent timestamp if available
			if h.EmailSendTest.SentAt != "" {
				sb.WriteString(fmt.Sprintf("  └─ Sent at: %s\n", h.EmailSendTest.SentAt))
			}

			// Show receive check results if enabled
			if h.EmailSendTest.CheckReceived {
				receiveState := "Failed"
				if h.EmailSendTest.ReceiveSuccess {
					receiveState = "Successful"
				}
				sb.WriteString(common.SimpleStatusListItem(
					fmt.Sprintf("Receive Check (%s)", h.EmailSendTest.ToEmailUsername),
					receiveState,
					h.EmailSendTest.ReceiveSuccess, // Success = email received successfully (green)
				))
				sb.WriteString("\n")

				// Show IMAP server info
				imapInfo := fmt.Sprintf("%s:%d", h.EmailSendTest.IMAPServer, h.EmailSendTest.IMAPPort)
				if h.EmailSendTest.IMAPUseTLS {
					imapInfo += " (TLS)"
				}
				sb.WriteString(common.SimpleStatusListItem(
					"IMAP Server",
					imapInfo,
					true, // Always green for display info
				))

				// Show retry configuration info
				sb.WriteString(fmt.Sprintf("  └─ Retry config: %d attempts, %d sec intervals\n", h.EmailSendTest.CheckRetries, h.EmailSendTest.CheckRetryInterval))

				// Show received timestamp or error message
				if h.EmailSendTest.ReceiveSuccess && h.EmailSendTest.ReceivedAt != "" {
					sb.WriteString(fmt.Sprintf("  └─ Received at: %s\n", h.EmailSendTest.ReceivedAt))
				} else if h.EmailSendTest.CheckMessage != "" {
					sb.WriteString(fmt.Sprintf("  └─ Check: %s\n", h.EmailSendTest.CheckMessage))
				}
			}
		} else {
			sb.WriteString(common.SimpleStatusListItem(
				fmt.Sprintf("Send (%s → %s)", h.EmailSendTest.FromEmail, h.EmailSendTest.ToEmail),
				"Check Failed",
				false,
			))
			sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.EmailSendTest.Message))
		}
		sb.WriteString("\n")
	}

	// CBPolicyd Section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("CBPolicyd Status"))
	sb.WriteString("\n")
	if h.CBPolicyd.CheckStatus {
		// Service status
		serviceState := "Stopped"
		if h.CBPolicyd.ServiceRunning {
			serviceState = "Running"
		}
		sb.WriteString(common.SimpleStatusListItem(
			"Service Status",
			serviceState,
			h.CBPolicyd.ServiceRunning, // Success = service running (green)
		))
		sb.WriteString("\n")

		// Configuration status
		configState := "Missing"
		if h.CBPolicyd.ConfigExists {
			if h.CBPolicyd.DatabaseConfigured {
				configState = "Configured"
			} else {
				configState = "No DB Config"
			}
		}
		sb.WriteString(common.SimpleStatusListItem(
			"Configuration",
			configState,
			h.CBPolicyd.ConfigExists && h.CBPolicyd.DatabaseConfigured, // Success = config exists and DB configured
		))
		sb.WriteString("\n")

		// Database connectivity (only if configured)
		if h.CBPolicyd.DatabaseConfigured {
			dbState := "Not Accessible"
			if h.CBPolicyd.DatabaseConnectable {
				dbState = fmt.Sprintf("Accessible (%s)", h.CBPolicyd.DatabaseType)
			} else {
				dbState = fmt.Sprintf("Not Accessible (%s)", h.CBPolicyd.DatabaseType)
			}
			sb.WriteString(common.SimpleStatusListItem(
				"Database",
				dbState,
				h.CBPolicyd.DatabaseConnectable, // Success = database accessible (green)
			))

			// Show database details if available
			if h.CBPolicyd.DatabaseHost != "" && h.CBPolicyd.DatabaseName != "" {
				sb.WriteString(fmt.Sprintf("\n        └─ %s@%s", h.CBPolicyd.DatabaseName, h.CBPolicyd.DatabaseHost))
			} else if h.CBPolicyd.DatabaseName != "" {
				sb.WriteString(fmt.Sprintf("\n        └─ %s", h.CBPolicyd.DatabaseName))
			}
		}
	} else {
		sb.WriteString(common.SimpleStatusListItem(
			"CBPolicyd Check",
			"Check Failed",
			false,
		))
		sb.WriteString(fmt.Sprintf("  └─ Error: %s\n", h.CBPolicyd.Message))
	}
	sb.WriteString("\n")

	return sb.String()
}

// RenderZimbraHealthCLI renders the Zimbra health information for CLI output with borders
func RenderZimbraHealthCLI(data *ZimbraHealthData, version string) string {
	// The version parameter is not directly used by ZimbraHealthData,
	// but we can ensure the data is complete before rendering
	return data.RenderAll()
}
