// This file defines the types used in the osHealth package
//
// It provides the following types:
// - OsHealth: Represents the configuration for osHealth

package winHealth

type WinHealth struct {
	Filesystems          []string
	Excluded_Mountpoints []string
	System_Load_And_Ram  bool
	Part_use_limit       float64
	Top_Processes        struct {
		Load_enabled   bool
		Load_processes int
		Ram_enabled    bool
		Ram_processes  int
	}

	Load struct {
		Issue_Interval   float64
		Issue_Multiplier float64
		Limit_Multiplier float64
	}

	Ram_Limit float64

	Services struct {
		Enabled bool
		Include []string
		Exclude []string
		Status  []string
	}

	Alarm struct {
		Enabled bool
	}

	License struct {
		Expiration_Limit int `json:"expiration_limit" mapstructure:"expiration_limit"`
	} `json:"license" mapstructure:"license"`
}
