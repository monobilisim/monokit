package osHealth

import (
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// SystemdUnit represents a systemd unit
type SystemdUnit struct {
	Name        string
	Description string
	LoadState   string
	ActiveState string
	SubState    string
	Type        string
	State       string
}

// GetDiskPartitions returns disk partitions
func GetDiskPartitions(all bool) ([]disk.PartitionStat, error) {
	return disk.Partitions(all)
}

// GetDiskUsage returns disk usage for a path
func GetDiskUsage(path string) (*disk.UsageStat, error) {
	return disk.Usage(path)
}

// GetVirtualMemory returns virtual memory stats
func GetVirtualMemory() (*mem.VirtualMemoryStat, error) {
	return mem.VirtualMemory()
}

// GetLoadAvg returns load average
func GetLoadAvg() (*load.AvgStat, error) {
	return load.Avg()
}

// GetCPUCount returns the number of CPUs
func GetCPUCount() (int, error) {
	cpus, err := cpu.Counts(true)
	return cpus, err
}

// GetUptime returns system uptime in seconds
func GetUptime() (uint64, error) {
	uptime, err := host.Uptime()
	return uptime, err
}

// GetSystemdUnits returns systemd units (Linux only)
func GetSystemdUnits() ([]SystemdUnit, error) {
	// This is a stub - will be implemented with actual systemd calls on Linux
	var units []SystemdUnit
	
	// For non-Linux systems return empty array
	return units, nil
}
