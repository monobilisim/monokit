package common

type Mysql struct {
	Process_limit int
	Cluster       struct {
		Enabled          bool
		Size             int
		Check_table_day  string
		Check_table_hour string
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
	Leader_switch_hook string
}

type DbHealth struct {
	Mysql    Mysql
	Postgres Postgres
}
