package osHealth

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/disk"
)

const fstabPath = "/etc/fstab"

var pseudoFilesystems = map[string]bool{
	"autofs":      true,
	"bpf":         true,
	"cgroup":      true,
	"cgroup2":     true,
	"configfs":    true,
	"debugfs":     true,
	"devpts":      true,
	"devtmpfs":    true,
	"efivarfs":    true,
	"fusectl":     true,
	"hugetlbfs":   true,
	"mqueue":      true,
	"proc":        true,
	"pstore":      true,
	"ramfs":       true,
	"rpc_pipefs":  true,
	"securityfs":  true,
	"swap":        true,
	"sysfs":       true,
	"tmpfs":       true,
	"tracefs":     true,
	"binfmt_misc": true,
}

type FstabEntry struct {
	Source     string
	Mountpoint string
	Fstype     string
	Options    []string
}

func unescapeFstab(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}

	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func normalizeMountpoint(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func parseFstab(path string) ([]FstabEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []FstabEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		source := unescapeFstab(fields[0])
		mountpoint := normalizeMountpoint(unescapeFstab(fields[1]))
		fstype := strings.ToLower(fields[2])

		options := []string{"defaults"}
		if len(fields) >= 4 {
			options = strings.Split(fields[3], ",")
		}

		if pseudoFilesystems[fstype] || mountpoint == "none" || mountpoint == "swap" || mountpoint == "" {
			continue
		}

		isNoAuto := false
		for _, opt := range options {
			if strings.TrimSpace(opt) == "noauto" {
				isNoAuto = true
				break
			}
		}
		if isNoAuto {
			log.Debug().Str("mountpoint", mountpoint).Msg("Skipping noauto fstab entry")
			continue
		}

		isExcluded := false
		for _, excluded := range OsHealthConfig.Excluded_Mountpoints {
			if excluded != "" && strings.HasPrefix(mountpoint, excluded) {
				isExcluded = true
				break
			}
		}
		if isExcluded {
			log.Debug().Str("mountpoint", mountpoint).Msg("Skipping excluded fstab mountpoint")
			continue
		}

		entries = append(entries, FstabEntry{
			Source:     source,
			Mountpoint: mountpoint,
			Fstype:     fstype,
			Options:    options,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func mountedMountpoints() (map[string]bool, error) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return nil, err
	}

	mounted := make(map[string]bool, len(partitions))
	for _, p := range partitions {
		mounted[normalizeMountpoint(p.Mountpoint)] = true
	}
	return mounted, nil
}

func createUnmountedTable(unmounted []FstabMountInfo) string {
	var tableData [][]string
	for _, m := range unmounted {
		tableData = append(tableData, []string{m.Device, m.Mountpoint, m.Fstype})
	}
	return renderMarkdownTable([]string{"Device", "Mount Point", "Filesystem"}, tableData)
}

func FstabHealth() []FstabMountInfo {
	entries, err := parseFstab(fstabPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error().Err(err).Str("component", "osHealth").Str("operation", "FstabHealth").Msg("An error occurred while reading " + fstabPath)
		}
		return nil
	}

	if len(entries) == 0 {
		return nil
	}

	mounted, err := mountedMountpoints()
	if err != nil {
		log.Error().Err(err).Str("component", "osHealth").Str("operation", "FstabHealth").Msg("An error occurred while fetching mounted filesystems")
		return nil
	}

	var mounts []FstabMountInfo
	var unmounted []FstabMountInfo

	for _, entry := range entries {
		info := FstabMountInfo{
			Device:     entry.Source,
			Mountpoint: entry.Mountpoint,
			Fstype:     entry.Fstype,
			Mounted:    mounted[entry.Mountpoint],
		}
		mounts = append(mounts, info)

		if !info.Mounted {
			unmounted = append(unmounted, info)
		}
	}

	if len(unmounted) > 0 {
		table := createUnmountedTable(unmounted)
		fullMsg := "The following partitions are listed in " + fstabPath + " but are not mounted;\n\n" + table
		common.AlarmCheckDown("fstab_mount", fullMsg, false, "", "")
	} else {
		common.AlarmCheckUp("fstab_mount", "All partitions listed in "+fstabPath+" are mounted.", false)
	}

	return mounts
}
