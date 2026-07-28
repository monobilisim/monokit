package mongodbHealth

// MongoHealthData is the root structure holding all collected health information
// for a single mongodbHealth run, used both for rendering and for POSTing to
// the Monokit server.
type MongoHealthData struct {
	Version        string
	IsReplicaSet   bool
	ConnectionInfo ConnectionInfo
	Standalone     StandaloneInfo
	ReplicaSet     ReplicaSetInfo
	// PermissionWarning is set when a status-determining command (e.g.
	// replSetGetStatus) fails due to insufficient privileges rather than
	// a genuine standalone/connectivity condition.
	PermissionWarning string
}

// ConnectionInfo describes the outcome of the initial connect+ping.
type ConnectionInfo struct {
	Connected bool
	ServerTime string
	Error      string
}

// StandaloneInfo holds serverStatus-derived metrics that apply to any node
// (standalone or member of a replica set).
type StandaloneInfo struct {
	ConnectionsCurrent   int
	ConnectionsAvailable int
	ConnectionsPercent   float64
	ConnectionsLimit     float64
	ConnectionsExceeded  bool

	CacheUsedPercent float64
	CacheLimit       float64
	CacheExceeded    bool

	TicketsAvailableRead  int
	TicketsAvailableWrite int
	TicketsExhausted      bool
}

// MemberInfo describes a single replica set member as reported by
// replSetGetStatus.
type MemberInfo struct {
	Name       string
	StateStr   string
	Health     int
	Healthy    bool
	IsPrimary  bool
	LagSeconds float64
}

// ReplicaSetInfo holds replSetGetStatus-derived metrics and thresholds.
type ReplicaSetInfo struct {
	SetName string

	Primary         string
	PreviousPrimary string
	PrimaryChanged  bool
	PrimaryAbsent   bool

	Members             []MemberInfo
	HealthySecondaries  int
	MinSecondaries      int
	SecondaryQuorumOk   bool

	MaxLagSeconds      float64
	LagWarnSeconds      float64
	LagCriticalSeconds  float64
	LagState            string // "ok", "warn", "critical"

	OplogWindowHours   float64
	OplogWarnHours     float64
	OplogCriticalHours float64
	OplogState         string // "ok", "warn", "critical"
}
