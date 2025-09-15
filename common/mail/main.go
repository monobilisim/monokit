package common

type Postal struct {
	Message_Threshold int
	Held_Threshold    int
	Check_Message     bool
}

type Zimbra struct {
	Z_Url            string `mapstructure:"z_url" yaml:"z_url"`
	Restart          bool   `mapstructure:"restart" yaml:"restart"`
	Queue_Limit      int    `mapstructure:"queue_limit" yaml:"queue_limit"`
	Restart_Limit    int    `mapstructure:"restart_limit" yaml:"restart_limit"`
	Restart_Interval int    `mapstructure:"restart_interval" yaml:"restart_interval"`
	Cache_interval   int    `mapstructure:"cache_interval" yaml:"cache_interval"`
	Webhook_tail     struct {
		Logfile     string `mapstructure:"logfile" yaml:"logfile"`
		Quota_limit int    `mapstructure:"quota_limit" yaml:"quota_limit"`
		Stream      string `mapstructure:"stream" yaml:"stream"`
		Topic       string `mapstructure:"topic" yaml:"topic"`
	} `mapstructure:"webhook_tail" yaml:"webhook_tail"`
	Zmfixperms struct {
		Topic  string `mapstructure:"topic" yaml:"topic"`
		Stream string `mapstructure:"stream" yaml:"stream"`
	} `mapstructure:"zmfixperms" yaml:"zmfixperms"`
	Login_test struct {
		Enabled  bool   `mapstructure:"enabled" yaml:"enabled"`
		Username string `mapstructure:"username" yaml:"username"`
		Password string `mapstructure:"password" yaml:"password"`
	} `mapstructure:"login_test" yaml:"login_test"`
	Email_send_test struct {
		Enabled              bool   `mapstructure:"enabled" yaml:"enabled"`
		From_email           string `mapstructure:"from_email" yaml:"from_email"`
		To_email             string `mapstructure:"to_email" yaml:"to_email"`
		Smtp_server          string `mapstructure:"smtp_server" yaml:"smtp_server"`
		Smtp_port            int    `mapstructure:"smtp_port" yaml:"smtp_port"`
		Use_tls              bool   `mapstructure:"use_tls" yaml:"use_tls"`
		Subject              string `mapstructure:"subject" yaml:"subject"`
		Check_received       bool   `mapstructure:"check_received" yaml:"check_received"`
		Imap_server          string `mapstructure:"imap_server" yaml:"imap_server"`
		Imap_port            int    `mapstructure:"imap_port" yaml:"imap_port"`
		Imap_use_tls         bool   `mapstructure:"imap_use_tls" yaml:"imap_use_tls"`
		To_email_username    string `mapstructure:"to_email_username" yaml:"to_email_username"`
		To_email_password    string `mapstructure:"to_email_password" yaml:"to_email_password"`
		Check_retries        int    `mapstructure:"check_retries" yaml:"check_retries"`
		Check_retry_interval int    `mapstructure:"check_retry_interval" yaml:"check_retry_interval"`
	} `mapstructure:"email_send_test" yaml:"email_send_test"`
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
