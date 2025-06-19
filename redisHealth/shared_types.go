package redisHealth

// Config holds configuration specific to redisHealth checks
type Config struct {
	Port        string `yaml:"port"`
	Password    string `yaml:"password"`
	Slave_count int    `yaml:"slave_count"`
}

// RedisHealthConfig is the global instance of the redisHealth configuration
var RedisHealthConfig Config

// RedisHealthData represents the health status of Redis
type RedisHealthData struct {
	Version     string         `json:"version"`
	LastChecked string         `json:"lastChecked"`
	Service     ServiceInfo    `json:"service"`
	Connection  ConnectionInfo `json:"connection"`
	Role        RoleInfo       `json:"role"`
	Sentinel    *SentinelInfo  `json:"sentinel,omitempty"`
}

// ServiceInfo represents Redis service status
type ServiceInfo struct {
	Active bool `json:"active"`
}

// ConnectionInfo represents Redis connection status
type ConnectionInfo struct {
	Pingable  bool `json:"pingable"`
	Writeable bool `json:"writeable"`
	Readable  bool `json:"readable"`
}

// RoleInfo represents Redis role information
type RoleInfo struct {
	IsMaster bool `json:"isMaster"`
}

// SentinelInfo represents Redis Sentinel information
type SentinelInfo struct {
	Active        bool `json:"active"`
	SlaveCount    int  `json:"slaveCount"`
	ExpectedCount int  `json:"expectedCount"`
}
