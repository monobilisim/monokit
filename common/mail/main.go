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
