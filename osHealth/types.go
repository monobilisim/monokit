// This file defines the types used in the osHealth package
//
// It provides the following types:
// - OsHealth: Represents the configuration for osHealth

package osHealth

type OsHealth struct {
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

	Alarm struct {
		Enabled bool
	}
}
