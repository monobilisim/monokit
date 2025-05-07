package postalHealth

// PostalHealthData holds information about the Postal health status
type PostalHealthData struct {
	Status        string
	IsHealthy     bool
	Services      map[string]bool
	Containers    map[string]ContainerStatus
	MySQLStatus   map[string]bool
	MessageQueue  QueueStatus
	HeldMessages  map[string]ServerHeldMessages
	ServiceStatus map[string]bool
}

// ContainerStatus holds information about a Docker container
type ContainerStatus struct {
	Name      string
	IsRunning bool
	State     string
}

// QueueStatus holds information about message queues
type QueueStatus struct {
	Count     int
	Limit     int
	IsHealthy bool
}

// ServerHeldMessages holds information about held messages for a specific server
type ServerHeldMessages struct {
	ServerName string
	ServerID   int
	Count      int
	IsHealthy  bool
}
