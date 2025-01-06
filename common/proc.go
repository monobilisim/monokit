package common

import (
    "os"
    "strings"
    "github.com/shirou/gopsutil/v4/process"
)

func ProcGrep(procName string, ignoreCurrentProc bool, getFullProcName bool) bool {
    var name string
    procs, _ := process.Processes()
    pid := os.Getpid()

    for _, proc := range procs {
        
        name, _ = proc.Name()
        if getFullProcName {
            name, _ = proc.Cmdline()
        }
        
        if strings.Contains(name, procName) {
            if ignoreCurrentProc {
                if int(proc.Pid) == int(pid) { 
                    continue
                }
            }
            return true
        }
    }
    return false
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
