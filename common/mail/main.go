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
}

type Pmg struct {
	Queue_Limit int
}

type MailHealth struct {
	Postal Postal
	Zimbra Zimbra
	Pmg    Pmg
}
