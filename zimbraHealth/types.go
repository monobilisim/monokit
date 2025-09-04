//go:build plugin && linux

package zimbraHealth

import (
	mail "github.com/monobilisim/monokit/common/mail"
)

// ZimbraHealthData holds all the health information for Zimbra.
type ZimbraHealthData struct {
	System         SystemInfo
	IPAccess       IPAccessInfo
	Services       []ServiceInfo
	Version        VersionInfo
	ZPush          ZPushInfo
	QueuedMessages QueuedMessagesInfo
	SSLCert        SSLCertInfo
	HostsFile      HostsFileInfo     // /etc/hosts file monitoring
	WebhookTail    WebhookTailInfo   // Placeholder for potential future UI integration
	LoginTest      LoginTestInfo     // Login test results
	EmailSendTest  EmailSendTestInfo // Email send test results
	CBPolicyd      CBPolicydInfo     // CBPolicyd service and database checks
	CacheInfo      CacheInfo         // Cache system information
}

// CacheInfo holds information about the caching system
type CacheInfo struct {
	Enabled       bool   `json:"enabled"`         // True if caching is enabled
	CacheInterval int    `json:"cache_interval"`  // Hours between full checks
	LastFullCheck string `json:"last_full_check"` // Timestamp of last full check
	NextFullCheck string `json:"next_full_check"` // Timestamp of next full check
	FromCache     bool   `json:"from_cache"`      // True if current data is from cache
	CacheFile     string `json:"cache_file"`      // Path to cache file
}

// DatabaseConfig holds parsed database configuration
type DatabaseConfig struct {
	Type     string
	Host     string
	Port     string
	Database string
	Username string
	Password string
	DSN      string
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

// LoginTestInfo holds information about the login test results.
type LoginTestInfo struct {
	Enabled         bool   // True if login test is enabled in config
	Username        string // Username used for testing (for display purposes)
	LoginSuccessful bool   // True if login was successful
	LastMailSubject string // Subject of the last received mail (if any)
	LastMailDate    string // Date of the last received mail (if any)
	CheckStatus     bool   // True if the check could be performed successfully
	Message         string // Any error or status message
}

// EmailSendTestInfo holds information about the email send test results.
type EmailSendTestInfo struct {
	Enabled     bool   // True if email send test is enabled in config
	FromEmail   string // From email address used for testing (for display purposes)
	ToEmail     string // To email address used for testing (for display purposes)
	SMTPServer  string // SMTP server used for testing (for display purposes)
	SMTPPort    int    // SMTP port used for testing (for display purposes)
	UseTLS      bool   // True if TLS is used for SMTP connection
	Subject     string // Email subject used for testing
	SendSuccess bool   // True if email was sent successfully
	CheckStatus bool   // True if the check could be performed successfully
	Message     string // Any error or status message
	SentAt      string // Timestamp when email was sent (if successful)

	// Mail checking fields
	CheckReceived      bool   // True if email checking is enabled
	IMAPServer         string // IMAP server for checking received emails
	IMAPPort           int    // IMAP port for checking received emails
	IMAPUseTLS         bool   // True if TLS is used for IMAP connection
	ToEmailUsername    string // Username for IMAP login (usually same as to_email)
	ToEmailPassword    string // Password for IMAP login
	CheckRetries       int    // Number of retry attempts (default: 3)
	CheckRetryInterval int    // Seconds between retry attempts (default: 30)
	TestID             string // Unique test ID for email verification
	ReceiveSuccess     bool   // True if email was found in recipient's mailbox
	ReceivedAt         string // Timestamp when email was found (if successful)
	CheckMessage       string // Message about the email checking process
	ForcedByEnv        bool   // True if test was forced by environment variable
}

// CBPolicydInfo holds information about CBPolicyd service and database connectivity.
type CBPolicydInfo struct {
	ServiceRunning      bool   // True if cbpolicyd service is running
	ConfigExists        bool   // True if cbpolicyd.conf.in exists
	DatabaseConfigured  bool   // True if database configuration is found in config
	DatabaseConnectable bool   // True if database connection test succeeds
	DatabaseType        string // Type of database (mysql, sqlite, etc.)
	DatabaseHost        string // Database host (for display)
	DatabaseName        string // Database name (for display)
	CheckStatus         bool   // True if the check could be performed successfully
	Message             string // Any error or status message
}

// NewZimbraHealthData creates a new initialized ZimbraHealthData struct.
func NewZimbraHealthData() *ZimbraHealthData {
	return &ZimbraHealthData{
		Services: []ServiceInfo{},
	}
}

// ZimbraHealthProvider implements the health.Provider interface
type ZimbraHealthProvider struct{}

// Name returns the name of the provider
func (p *ZimbraHealthProvider) Name() string {
	return "zimbraHealth"
}

// Collect gathers Zimbra health data.
// The 'hostname' parameter is ignored for zimbraHealth as it collects local data.
func (p *ZimbraHealthProvider) Collect(_ string) (interface{}, error) {
	return collectZimbraHealthData()
}

// collectZimbraHealthData is a wrapper around the existing collectHealthData function
// to provide the interface required by the plugin system
func collectZimbraHealthData() (*ZimbraHealthData, error) {
	return collectHealthData(), nil
}

// ZimbraHealthConfig is the global instance of the zimbraHealth configuration
// This is an alias to the MailHealthConfig used by the zimbraHealth package
var ZimbraHealthConfig mail.MailHealth
