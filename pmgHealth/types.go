package pmgHealth

// PmgHealthData holds information about the Proxmox Mail Gateway health status
type PmgHealthData struct {
	Status          string
	IsHealthy       bool
	Services        map[string]bool
	PostgresRunning bool
	QueueStatus     struct {
		Count     int
		Limit     int
		IsHealthy bool
	}
	VersionStatus struct {
		CurrentVersion string
		LatestVersion  string
		IsUpToDate     bool
	}
}
