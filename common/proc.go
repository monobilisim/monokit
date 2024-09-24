package common

import (
    "github.com/shirou/gopsutil/v4/process"
    "strings"
)

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
