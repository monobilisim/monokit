package common

type Postal struct {
    Message_Threshold int
    Held_Threshold int
    Check_Message bool
}

type Zimbra struct {
    Z_Url string
    Restart bool
    Queue_Limit int
    Restart_Limit int
    Webhook_tail struct {
        Logfile string
        Filter string
        Stream string
        Topic string
    }
}

type Pmg struct {
    Queue_Limit int
}

type MailHealth struct {
    Postal Postal
    Zimbra Zimbra
    Pmg Pmg
}
