package pmgHealth

// MailStatistics represents the mail statistics for our health data
type MailStatistics struct {
	Enabled         bool    `json:"enabled"`
	Last24hSent     int     `json:"last_24h_sent"`
	Last24hReceived int     `json:"last_24h_received"`
	Last24hTotal    int     `json:"last_24h_total"`
	Prev24hSent     int     `json:"prev_24h_sent"`
	Prev24hReceived int     `json:"prev_24h_received"`
	Prev24hTotal    int     `json:"prev_24h_total"`
	Last1hSent      int     `json:"last_1h_sent"`
	Last1hReceived  int     `json:"last_1h_received"`
	Last1hTotal     int     `json:"last_1h_total"`
	Prev1hSent      int     `json:"prev_1h_sent"`
	Prev1hReceived  int     `json:"prev_1h_received"`
	Prev1hTotal     int     `json:"prev_1h_total"`
	Threshold24h    float64 `json:"threshold_24h"`
	Threshold1h     float64 `json:"threshold_1h"`
	IsNormal24h     bool    `json:"is_normal_24h"`
	IsNormal1h      bool    `json:"is_normal_1h"`
	// Redmine issue tracking for elevated traffic
	RedmineIssue24h string `json:"redmine_issue_24h,omitempty"`
	RedmineIssue1h  string `json:"redmine_issue_1h,omitempty"`
}

// QueueStatus represents the mail queue status
type QueueStatus struct {
	Count     int  `json:"count"`
	Limit     int  `json:"limit"`
	IsHealthy bool `json:"is_healthy"`
}

// VersionStatus represents version information
type VersionStatus struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	IsUpToDate     bool   `json:"is_up_to_date"`
}

// PmgHealthData represents the overall PMG health status
type PmgHealthData struct {
	IsHealthy       bool            `json:"is_healthy"`
	Status          string          `json:"status"`
	Services        map[string]bool `json:"services"`
	PostgresRunning bool            `json:"postgres_running"`
	QueueStatus     QueueStatus     `json:"queue_status"`
	MailStats       MailStatistics  `json:"mail_stats"`
	VersionStatus   VersionStatus   `json:"version_status"`
}

// PmgMailStatistics represents the PMG mail statistics API response
type PmgMailStatistics struct {
	AvgProcessingTime float64 `json:"avptime"`
	BouncesIn         int     `json:"bounces_in"`
	BouncesOut        int     `json:"bounces_out"`
	BytesIn           int64   `json:"bytes_in"`
	BytesOut          int64   `json:"bytes_out"`
	Count             int     `json:"count"`
	CountIn           int     `json:"count_in"`
	CountOut          int     `json:"count_out"`
	GreylistCount     int     `json:"glcount"`
	JunkIn            int     `json:"junk_in"`
	JunkOut           int     `json:"junk_out"`
	PregreetRejects   int     `json:"pregreet_rejects"`
	RblRejects        int     `json:"rbl_rejects"`
	SpamCountIn       int     `json:"spamcount_in"`
	SpamCountOut      int     `json:"spamcount_out"`
	SpfCount          int     `json:"spfcount"`
	VirusCountIn      int     `json:"viruscount_in"`
	VirusCountOut     int     `json:"viruscount_out"`
}
