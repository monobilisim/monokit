package osHealth

import (
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
    "strings"
    "unicode/utf8"
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

// renderMarkdownTable renders a Markdown-style table with headers and rows.
// It pads columns based on the maximum width of header/rows for readable alignment.
func renderMarkdownTable(headers []string, rows [][]string) string {
    // Compute column widths
    colCount := len(headers)
    colWidths := make([]int, colCount)

    for i, h := range headers {
        colWidths[i] = runeLen(h)
    }
    for _, row := range rows {
        for i := 0; i < colCount && i < len(row); i++ {
            if l := runeLen(row[i]); l > colWidths[i] {
                colWidths[i] = l
            }
        }
    }

    // Helper to pad a cell to width
    pad := func(s string, width int) string {
        // Right-pad with spaces to target width
        diff := width - runeLen(s)
        if diff <= 0 {
            return s
        }
        return s + strings.Repeat(" ", diff)
    }

    var b strings.Builder

    // Header row
    b.WriteString("| ")
    for i, h := range headers {
        b.WriteString(pad(h, colWidths[i]))
        if i == colCount-1 {
            b.WriteString(" |")
        } else {
            b.WriteString(" | ")
        }
    }
    b.WriteString("\n")

    // Separator row (--- style)
    b.WriteString("|")
    for i := range headers {
        b.WriteString(strings.Repeat("-", colWidths[i]+2)) // account for spaces around cell
        if i == colCount-1 {
            b.WriteString("|")
        } else {
            b.WriteString("|")
        }
    }
    b.WriteString("\n")

    // Data rows
    for _, row := range rows {
        b.WriteString("| ")
        for i := 0; i < colCount; i++ {
            var v string
            if i < len(row) {
                v = row[i]
            }
            b.WriteString(pad(v, colWidths[i]))
            if i == colCount-1 {
                b.WriteString(" |")
            } else {
                b.WriteString(" | ")
            }
        }
        b.WriteString("\n")
    }

    return b.String()
}

// runeLen returns the rune length (display width approximation using rune count)
func runeLen(s string) int {
    return utf8.RuneCountInString(s)
}
