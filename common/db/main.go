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
		Certification_limit int
	}
	Alarm struct {
		Enabled bool
	}
}

type Postgres struct {
	Limits struct {
		Process         int
		Query           int
		Conn_percent    int
		Long_query_time int
	}
	Alarm struct {
		Enabled    bool
		Long_query bool
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

type Mongodb struct {
	Uri                     string
	Connect_timeout_seconds int
	Alarm                   struct {
		Enabled bool
	}
	Limits struct {
		Connections_percent int
		Cache_percent       int
	}
	Replicaset struct {
		// Enabled indicates this node is expected to be part of a replica
		// set. If true and monokit cannot confirm replica set membership
		// (either because replSetGetStatus fails, e.g. due to insufficient
		// permissions, or because the node genuinely reports as standalone),
		// that mismatch is treated as an error condition.
		Enabled                        bool
		Min_secondaries                int
		Lag_warn_seconds               int
		Lag_critical_seconds           int
		Oplog_window_warn_hours        float64
		Oplog_window_critical_hours    float64
		Primary_election_grace_seconds int
	}
}

type DbHealth struct {
	Mysql    Mysql
	Postgres Postgres
	Mongodb  Mongodb
}
