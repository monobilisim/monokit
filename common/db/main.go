package common

type Mysql struct {
	Process_limit int
	Pmm_enabled   *bool `json:"pmm_enabled,omitempty"`
	Cluster       struct {
		Enabled             bool
		Size                int
		Check_table_day     string
		Check_table_hour    string
		Receive_queue_limit int
		Flow_control_limit  float64
	}
	Alarm struct {
		Enabled bool
	}
}

type Postgres struct {
	Limits struct {
		Process      int
		Query        int
		Conn_percent int
	}
	Alarm struct {
		Enabled bool
	}

	Wal_g_verify_hour string

	Leader_switch_hook string

	Consul struct {
		Enabled bool
	}

	Haproxy struct {
		Enabled bool
	}
}

type DbHealth struct {
	Mysql    Mysql
	Postgres Postgres
}
