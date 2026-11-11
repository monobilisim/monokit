package postalHealth

// PostalHealthData holds information about the Postal health status
type PostalHealthData struct {
	Status        string
	IsHealthy     bool
	Services      map[string]bool
	Containers    map[string]ContainerStatus
	MySQLStatus   map[string]bool
	MessageQueue  QueueStatus
	HeldMessages  map[string]ServerHeldMessages
	StaleLocks    StaleLockStatus
	ServiceStatus map[string]bool
	SSLCert       SSLCertInfo
	// DBChecksNode is the node running the shared DB checks when this node skipped them.
	DBChecksNode string
}

// SSLCertInfo holds information about the Postal SMTP SSL certificate expiration.
type SSLCertInfo struct {
	Host            string
	Port            int
	DaysUntilExpiry int
	ExpiringSoon    bool
	CheckStatus     bool // true if the check itself could be performed successfully
	Message         string
}

// ContainerStatus holds information about a Docker container
type ContainerStatus struct {
	Name      string
	IsRunning bool
	State     string
	// NetworkMode is the container's Docker network mode ("host" when it shares the node's hostname).
	NetworkMode string
}

// QueueStatus holds information about message queues
type QueueStatus struct {
	Count          int // all queued messages, including ones waiting for a soft-fail retry
	Backlog        int // unlocked messages ready for BacklogMinutes+ without being picked up
	BacklogMinutes int
	Limit          int
	IsHealthy      bool
}

// StaleLockStatus holds the number of queued messages whose worker lock is older than the threshold.
// Postal never retries such messages; TidyQueuedMessagesTask deletes them after queued_message_lock_stale_days.
type StaleLockStatus struct {
	Count            int
	ThresholdMinutes int
	CheckStatus      bool // true if the query ran successfully
	IsHealthy        bool
}

// ServerHeldMessages holds information about held messages for a specific server
type ServerHeldMessages struct {
	ServerName string
	ServerID   int
	Count      int            // held messages in alarmed categories (see evaluateHeld)
	Reasons    map[string]int // held messages by reason category, including non-alarmed ones
	IsHealthy  bool
}
