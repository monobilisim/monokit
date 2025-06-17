package types

// RKE2Info holds the information collected by the RKE2 checker plugin
// and is used for communication between the plugin and the host.
type RKE2Info struct {
	IsRKE2Environment bool   `json:"isRke2Environment"`
	ClusterName       string `json:"clusterName"`
	CurrentVersion    string `json:"currentVersion"`
	IsMasterNode      bool   `json:"isMasterNode"`
	Error             string `json:"error,omitempty"`
}
