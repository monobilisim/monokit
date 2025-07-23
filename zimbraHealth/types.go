package zimbraHealth

// ZimbraHealthData holds all the health information for Zimbra.
type ZimbraHealthData struct {
	System         SystemInfo
	IPAccess       IPAccessInfo
	Services       []ServiceInfo
	Version        VersionInfo
	ZPush          ZPushInfo
	QueuedMessages QueuedMessagesInfo
	SSLCert        SSLCertInfo
	HostsFile      HostsFileInfo   // /etc/hosts file monitoring
	WebhookTail    WebhookTailInfo // Placeholder for potential future UI integration
}

// SystemInfo represents basic system information relevant to Zimbra checks.
type SystemInfo struct {
	Hostname    string
	ProductPath string // /opt/zimbra or /opt/zextras
	LastChecked string
}

// IPAccessInfo holds the status of external IP accessibility check.
type IPAccessInfo struct {
	IPAddress   string
	Accessible  bool   // True if the check passed (IP is NOT accessible directly)
	CheckStatus bool   // True if the check could be performed successfully
	Message     string // Any error or status message
}

// ServiceInfo holds the status of a single Zimbra service.
type ServiceInfo struct {
	Name    string
	Running bool
}

// ServiceState tracks persistent service health across runs
type ServiceState struct {
	Name            string `json:"name"`
	LastFailure     string `json:"last_failure,omitempty"`
	RestartAttempts int    `json:"restart_attempts,omitempty"`
	RecoveredAt     string `json:"recovered_at,omitempty"`
	Status          string `json:"status"` // "Running", "Stopped", "Disappeared"
}

// VersionInfo holds Zimbra version details.
type VersionInfo struct {
	InstalledVersion string
	LatestVersion    string // If available from check
	UpdateAvailable  bool
	CheckStatus      bool   // True if the check could be performed successfully
	Message          string // Any error or status message
}

// ZPushInfo holds the status of Z-Push functionality.
type ZPushInfo struct {
	URL         string
	HeaderFound bool   // True if Z-Push headers were detected
	NginxConfig bool   // True if /etc/nginx-php-fpm.conf exists
	CheckStatus bool   // True if the check could be performed successfully
	Message     string // Any error or status message
}

// QueuedMessagesInfo holds information about the mail queue.
type QueuedMessagesInfo struct {
	Count       int
	Limit       int
	Exceeded    bool
	CheckStatus bool   // True if the check could be performed successfully
	Message     string // Any error or status message
}

// SSLCertInfo holds information about the SSL certificate expiration.
type SSLCertInfo struct {
	MailHost        string
	DaysUntilExpiry int
	ExpiringSoon    bool   // True if days < threshold (e.g., 10)
	CheckStatus     bool   // True if the check could be performed successfully
	Message         string // Any error or status message
}

// HostsFileInfo holds information about /etc/hosts file monitoring.
type HostsFileInfo struct {
	BackupExists bool   // True if backup file exists in TmpDir
	HasChanges   bool   // True if current file differs from backup
	LastChecked  string // Timestamp of last check
	CheckStatus  bool   // True if the check could be performed successfully
	Message      string // Any error or status message
	BackupPath   string // Path to the backup file
}

// WebhookTailInfo holds information related to webhook tailing (currently not displayed in UI).
type WebhookTailInfo struct {
	Logfile    string
	QuotaLimit int
	// Add fields here if UI representation is needed later
}

// NewZimbraHealthData creates a new initialized ZimbraHealthData struct.
func NewZimbraHealthData() *ZimbraHealthData {
	return &ZimbraHealthData{
		Services: []ServiceInfo{},
	}
}
