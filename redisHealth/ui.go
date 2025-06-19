//go:build plugin

package redisHealth

import (
	"strings"

	"github.com/monobilisim/monokit/common"
)

// RenderCompact renders the Redis health information using the UI components
func (data *RedisHealthData) RenderCompact() string {
	var sb strings.Builder

	// Service Status section
	sb.WriteString(common.SectionTitle("Service Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Redis Service",
		"active",
		data.Service.Active))
	sb.WriteString("\n")

	// Connection Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Connection Status"))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Redis Connection",
		"connected",
		data.Connection.Pingable))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Redis Writeable",
		"writeable",
		data.Connection.Writeable))
	sb.WriteString("\n")
	sb.WriteString(common.SimpleStatusListItem(
		"Redis Readable",
		"readable",
		data.Connection.Readable))
	sb.WriteString("\n")

	// Role Information section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Role Information"))
	sb.WriteString("\n")
	if data.Role.IsMaster {
		sb.WriteString(common.SimpleStatusListItem(
			"Redis Role",
			"master",
			true))
	} else {
		sb.WriteString(common.SimpleStatusListItem(
			"Redis Role",
			"slave",
			true))
	}
	sb.WriteString("\n")

	// Sentinel Information section (if applicable)
	if data.Sentinel != nil {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Sentinel Information"))
		sb.WriteString("\n")
		sb.WriteString(common.SimpleStatusListItem(
			"Sentinel Service",
			"active",
			data.Sentinel.Active))
		sb.WriteString("\n")

		if data.Role.IsMaster {
			slaveCountOK := data.Sentinel.SlaveCount == data.Sentinel.ExpectedCount
			sb.WriteString(common.SimpleStatusListItem(
				"Slave Count",
				strings.Join([]string{
					string(rune(data.Sentinel.SlaveCount + '0')),
					"/",
					string(rune(data.Sentinel.ExpectedCount + '0')),
				}, ""),
				slaveCountOK))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// RenderAll renders all Redis health data as a single string with borders
func (data *RedisHealthData) RenderAll() string {
	title := "monokit redisHealth"
	if data.Version != "" {
		title += " v" + data.Version
	}

	content := data.RenderCompact()
	return common.DisplayBox(title, content)
}

// RenderRedisHealth renders the Redis health information using the UI components
// Deprecated: Use RenderCompact() method instead
func RenderRedisHealth(data *RedisHealthData) string {
	return data.RenderCompact()
}

// RenderRedisHealthCLI renders the Redis health information for CLI output with borders
func RenderRedisHealthCLI(data *RedisHealthData, version string) string {
	// Set the version if not already set
	if data.Version == "" {
		data.Version = version
	}

	return data.RenderAll()
}
