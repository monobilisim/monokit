package common

import (
	"os"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

func ProcGrep(procName string, ignoreCurrentProc bool) bool {
	procs, _ := process.Processes()
	pid := os.Getpid()

	for _, proc := range procs {
		cmdline, _ := proc.Cmdline()
		pname, _ := proc.Name()

		if ignoreCurrentProc && int(proc.Pid) == int(pid) {
			continue
		}

		// Require exact match for: binary is "monokit" AND cmdline contains "monokit daemon"
		// Avoid substring matches (e.g., "daemon-test")
		if pname == "monokit" && isDaemonCmd(cmdline) {
			return true
		}
	}
	return false
}

// isDaemonCmd returns true if cmdline contains "monokit daemon" as a separate word
func isDaemonCmd(cmdline string) bool {
	// Accept only if cmdline starts with "monokit daemon" or is exactly that
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "monokit daemon" {
		return true
	}
	// Allow for flags after daemon
	return strings.HasPrefix(cmdline, "monokit daemon ")
}

func ConnsByProc(prefix string) uint32 {

	procs, _ := process.Processes()

	for _, proc := range procs {
		procName, _ := proc.Name()

		if strings.HasPrefix(procName, prefix) {
			conn, _ := proc.Connections()
			for _, c := range conn {
				if c.Laddr.Port != 0 {
					return c.Laddr.Port
				} else {
					continue
				}
			}
		}
	}
	return 0000
}

func ConnsByProcMulti(prefix string) []uint32 {

	var ports []uint32

	procs, _ := process.Processes()

	for _, proc := range procs {
		procName, _ := proc.Name()

		if strings.HasPrefix(procName, prefix) {
			conn, _ := proc.Connections()
			for _, c := range conn {
				if c.Laddr.Port != 0 {
					ports = append(ports, c.Laddr.Port)
				} else {
					continue
				}
			}
		}
	}
	return ports
}
