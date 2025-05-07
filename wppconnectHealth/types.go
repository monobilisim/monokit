package wppconnectHealth

// WppConnectData holds information about WPPConnect session status
type WppConnectData struct {
	Session     string
	ContactName string
	Status      string
	Healthy     bool
}

// WppConnectHealthData holds the overall health status of WPPConnect
type WppConnectHealthData struct {
	Sessions     []WppConnectData
	TotalCount   int
	HealthyCount int
	Healthy      bool
	Version      string
}
