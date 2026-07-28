package mongodbHealth

import (
	"fmt"
	"strings"

	"github.com/monobilisim/monokit/common"
)

// NewMongoHealthData returns a zero-valued MongoHealthData ready to be
// populated by the check functions.
func NewMongoHealthData() *MongoHealthData {
	return &MongoHealthData{}
}

// RenderCompact renders the collected health data as a compact status
// listing, without the outer title box.
func (m *MongoHealthData) RenderCompact() string {
	var b strings.Builder

	b.WriteString(common.SectionTitle("Connection"))
	b.WriteString("\n")
	connectedText := "false"
	if m.ConnectionInfo.Connected {
		connectedText = "true"
	}
	b.WriteString(common.SimpleStatusListItem("Connected", connectedText, m.ConnectionInfo.Connected))
	if m.ConnectionInfo.Error != "" {
		b.WriteString("\n")
		b.WriteString(common.SimpleStatusListItem("Error", m.ConnectionInfo.Error, false))
	}

	if !m.ConnectionInfo.Connected {
		return b.String()
	}

	b.WriteString("\n\n")
	b.WriteString(common.SectionTitle("Standalone"))
	b.WriteString("\n")
	b.WriteString(common.StatusListItem(
		"Connections",
		"< ",
		fmt.Sprintf("%.2f%%", m.Standalone.ConnectionsLimit),
		fmt.Sprintf("%.2f%% (%d/%d)", m.Standalone.ConnectionsPercent, m.Standalone.ConnectionsCurrent, m.Standalone.ConnectionsCurrent+m.Standalone.ConnectionsAvailable),
		!m.Standalone.ConnectionsExceeded,
	))
	b.WriteString("\n")
	b.WriteString(common.StatusListItem(
		"Cache usage",
		"< ",
		fmt.Sprintf("%.2f%%", m.Standalone.CacheLimit),
		fmt.Sprintf("%.2f%%", m.Standalone.CacheUsedPercent),
		!m.Standalone.CacheExceeded,
	))
	b.WriteString("\n")
	b.WriteString(common.SimpleStatusListItem(
		"WiredTiger tickets",
		fmt.Sprintf("read=%d write=%d", m.Standalone.TicketsAvailableRead, m.Standalone.TicketsAvailableWrite),
		!m.Standalone.TicketsExhausted,
	))

	if m.PermissionWarning != "" {
		b.WriteString("\n\n")
		b.WriteString(common.SimpleStatusListItem("Warning", m.PermissionWarning, false))
	}

	if m.IsReplicaSet {
		b.WriteString("\n\n")
		b.WriteString(common.SectionTitle("Replica Set: " + m.ReplicaSet.SetName))
		b.WriteString("\n")
		b.WriteString(common.SimpleStatusListItem("Primary", m.ReplicaSet.Primary, !m.ReplicaSet.PrimaryAbsent))
		if m.ReplicaSet.PrimaryChanged {
			b.WriteString("\n")
			b.WriteString(common.SimpleStatusListItem("Primary changed", fmt.Sprintf("%s -> %s", m.ReplicaSet.PreviousPrimary, m.ReplicaSet.Primary), true))
		}
		b.WriteString("\n")
		b.WriteString(common.StatusListItem(
			"Healthy secondaries",
			">= ",
			fmt.Sprintf("%d", m.ReplicaSet.MinSecondaries),
			fmt.Sprintf("%d", m.ReplicaSet.HealthySecondaries),
			m.ReplicaSet.SecondaryQuorumOk,
		))
		for _, member := range m.ReplicaSet.Members {
			if member.IsPrimary {
				continue
			}
			b.WriteString("\n")
			b.WriteString(common.SimpleStatusListItem(
				"  "+member.Name,
				fmt.Sprintf("%s (lag=%.2fs)", member.StateStr, member.LagSeconds),
				member.Healthy,
			))
		}
		b.WriteString("\n")
		b.WriteString(common.StatusListItem(
			"Replication lag",
			"< ",
			fmt.Sprintf("%.2fs", m.ReplicaSet.LagWarnSeconds),
			fmt.Sprintf("%.2fs", m.ReplicaSet.MaxLagSeconds),
			m.ReplicaSet.LagState == "ok",
		))
		b.WriteString("\n")
		b.WriteString(common.StatusListItem(
			"Oplog window",
			"> ",
			fmt.Sprintf("%.2fh", m.ReplicaSet.OplogWarnHours),
			fmt.Sprintf("%.2fh", m.ReplicaSet.OplogWindowHours),
			m.ReplicaSet.OplogState == "ok",
		))
	}

	return b.String()
}

// RenderAll renders the full report, wrapped in the standard title box.
func (m *MongoHealthData) RenderAll() string {
	title := "monokit mongodbHealth"
	if m.Version != "" {
		title += " v" + m.Version
	}
	return common.DisplayBox(title, m.RenderCompact())
}
