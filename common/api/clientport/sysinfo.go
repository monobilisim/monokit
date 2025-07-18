package clientport

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// RealSysInfo is the production implementation of SysInfo.
type RealSysInfo struct{}

func (r RealSysInfo) CPUCores() int {
	n, err := cpu.Counts(true)
	if err != nil {
		return 0
	}
	return n
}

func (r RealSysInfo) RAM() string {
	m, err := mem.VirtualMemory()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%.2fGB", float64(m.Total)/1024/1024/1024)
}

func (r RealSysInfo) PrimaryIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Name != "lo" && len(iface.Addrs) > 0 {
			return iface.Addrs[0].Addr
		}
	}
	return ""
}

func (r RealSysInfo) OSPlatform() string {
	info, err := host.Info()
	if err != nil {
		return ""
	}
	return info.Platform + " " + info.PlatformVersion + " " + info.KernelVersion
}
