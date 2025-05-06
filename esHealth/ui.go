package esHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
)

// NewEsHealthData creates and initializes a new EsHealthData struct.
func NewEsHealthData() *EsHealthData {
	return &EsHealthData{
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
		// Allocation will be nil by default, populated if issues are found
	}
}

// RenderAll renders all health data as a single string for display.
// For now, it will call RenderCompact. This can be expanded later if a more detailed view is needed.
func (h *EsHealthData) RenderAll() string {
	return h.RenderCompact()
}

// RenderCompact renders a compact view of Elasticsearch health data.
func (h *EsHealthData) RenderCompact() string {
	var sb strings.Builder

	if h.Error != "" {
		sb.WriteString(common.SectionTitle("Elasticsearch Health Error"))
		sb.WriteString("\n")
		sb.WriteString(h.Error) // common.Colorize removed
		sb.WriteString("\n")
		return sb.String()
	}

	// --- Cluster Overview Section ---
	sb.WriteString(common.SectionTitle(fmt.Sprintf("Elasticsearch: %s", h.ClusterName)))
	sb.WriteString("\n")

	// Cluster Status
	clusterStateDisplay := strings.ToUpper(h.Status)
	if h.Status == "green" {
		clusterStateDisplay = "OK"
	}
	sb.WriteString(common.SimpleStatusListItem(
		"Cluster Status",
		clusterStateDisplay,
		h.Status == "green",
	))
	sb.WriteString("\n")

	// Node Stats
	sb.WriteString(common.SimpleStatusListItem(
		"Nodes",
		fmt.Sprintf("%d", h.NodeStats.TotalDataNodes),
		true, // Success is always true as it's informational
	))
	sb.WriteString("\n")

	// --- Shard Stats Section ---
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Shard Statistics"))
	sb.WriteString("\n")

	// Active Shards
	sb.WriteString(common.SimpleStatusListItem(
		"Active Shards (Primary / Total)",
		fmt.Sprintf("%d / %d (%.2f%%)", h.ShardStats.ActivePrimary, h.ShardStats.Active, h.ShardStats.ActivePercent),
		true, // Success is always true as it's informational
	))
	sb.WriteString("\n")

	// Initializing Shards
	sb.WriteString(common.StatusListItem(
		"Initializing Shards",
		"",  // Unit
		"0", // Limit (Expected value)
		fmt.Sprintf("%d", h.ShardStats.Initializing), // Actual
		h.ShardStats.Initializing == 0,               // Success condition
	))
	sb.WriteString("\n")

	// Relocating Shards
	sb.WriteString(common.StatusListItem(
		"Relocating Shards",
		"",  // Unit
		"0", // Limit
		fmt.Sprintf("%d", h.ShardStats.Relocating), // Actual
		h.ShardStats.Relocating == 0,               // Success
	))
	sb.WriteString("\n")

	// Unassigned Shards
	sb.WriteString(common.StatusListItem(
		"Unassigned Shards",
		"",  // Unit
		"0", // Limit
		fmt.Sprintf("%d", h.ShardStats.Unassigned), // Actual
		h.ShardStats.Unassigned == 0,               // Success
	))
	sb.WriteString("\n")

	// --- Allocation Section ---
	if h.Allocation != nil {
		sb.WriteString("\n")
		if h.Allocation.IsProblematic {
			sb.WriteString(common.SectionTitle("Shard Allocation Issues"))
			sb.WriteString("\n")
			sb.WriteString(common.SimpleStatusListItem(
				"Can Allocate",
				h.Allocation.CanAllocate, // Displays "no", "throttle", etc.
				false,                    // Marked as not success because it's problematic
			))
			sb.WriteString("\n")

			// Detailed explanation for problematic allocation
			if h.Allocation.Index != "" {
				sb.WriteString(fmt.Sprintf("  └─ Index: %s, Shard: %d (Primary: %t)\n", h.Allocation.Index, h.Allocation.Shard, h.Allocation.Primary))
				sb.WriteString(fmt.Sprintf("     State: %s\n", h.Allocation.CurrentState))
			}
			if h.Allocation.UnassignedReason != "" {
				sb.WriteString(fmt.Sprintf("  └─ Reason: %s (At: %s)\n", h.Allocation.UnassignedReason, h.Allocation.UnassignedAt))
			}
			sb.WriteString(fmt.Sprintf("  └─ Explanation: %s\n", h.Allocation.Explanation))

		} else { // Allocation is not problematic (IsProblematic is false)
			sb.WriteString(common.SectionTitle("Shard Allocation Status"))
			sb.WriteString("\n")
			sb.WriteString(common.SimpleStatusListItem(
				"Allocation Status", // Changed label
				"OK",
				true,
			))
			sb.WriteString("\n")
			// Removed the detailed explanation for OK status
			// if h.Allocation.Explanation != "" && h.Allocation.Explanation != "No unassigned shards to explain (API returned 400)." {
			// sb.WriteString(fmt.Sprintf("  └─ %s\n", h.Allocation.Explanation))
			// }
		}
	}
	// --- Footer ---
	// sb.WriteString("\n") // Removed Last Checked
	// sb.WriteString(fmt.Sprintf("Last Checked: %s", h.LastChecked)) // Removed Last Checked

	return sb.String()
}
