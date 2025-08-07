package common

type Postal struct {
	Message_Threshold int
	Held_Threshold    int
	Check_Message     bool
}

type Zimbra struct {
	Z_Url            string
	Restart          bool
	Queue_Limit      int
	Restart_Limit    int
	Restart_Interval int // minutes between restart attempts, default 3
	Cache_interval   int // hours between full health checks, default 12
	Webhook_tail     struct {
		Logfile     string
		Quota_limit int
		Stream      string
		Topic       string
	}
	Zmfixperms struct {
		Topic  string
		Stream string
	}
	Login_test struct {
		Enabled  bool
		Username string
		Password string
	}
	Email_send_test struct {
		Enabled              bool
		From_email           string
		To_email             string
		Smtp_server          string
		Smtp_port            int
		Use_tls              bool
		Subject              string
		Check_received       bool
		Imap_server          string
		Imap_port            int
		Imap_use_tls         bool
		To_email_username    string
		To_email_password    string
		Check_retries        int // Number of retry attempts (default: 3)
		Check_retry_interval int // Seconds between retry attempts (default: 30)
	}
}

type Pmg struct {
	Queue_Limit      int
	Email_monitoring struct {
		Enabled          bool `default:"false"`
		Threshold_factor struct {
			Daily  float64
			Hourly float64
		}
	}
	Blacklist_check struct {
		Enabled    bool     `default:"false"`
		IP         string   // IP address to check, if empty will auto-detect
		Ignorelist []string // List of blacklist names to ignore
	}
}

type MailHealth struct {
	Postal Postal
	Zimbra Zimbra
	Pmg    Pmg
}
