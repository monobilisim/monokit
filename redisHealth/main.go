//go:build linux

package redisHealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/spf13/cobra"
)

func init() {
	common.RegisterComponent(common.Component{
		Name:       "redisHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectRedis,
	})
}

var RedisHealthConfig struct {
	Port        string
	Password    string
	Slave_count int
}

// RedisHealthData represents the health status of Redis
type RedisHealthData struct {
	Version     string
	LastChecked string
	Service     ServiceInfo
	Connection  ConnectionInfo
	Role        RoleInfo
	Sentinel    *SentinelInfo
}

// ServiceInfo represents Redis service status
type ServiceInfo struct {
	Active bool
}

// ConnectionInfo represents Redis connection status
type ConnectionInfo struct {
	Pingable  bool
	Writeable bool
	Readable  bool
}

// RoleInfo represents Redis role information
type RoleInfo struct {
	IsMaster bool
}

// SentinelInfo represents Redis Sentinel information
type SentinelInfo struct {
	Active        bool
	SlaveCount    int
	ExpectedCount int
}

func Main(cmd *cobra.Command, args []string) {
	version := "0.2.0"
	common.ScriptName = "redisHealth"
	common.TmpDir = common.TmpDir + "redisHealth"
	common.Init()

	if common.ConfExists("redis") {
		common.ConfInit("redis", &RedisHealthConfig)
	}

	if RedisHealthConfig.Port == "" {
		RedisHealthConfig.Port = "6379"
	}

	api.WrapperGetServiceStatus("redisHealth")

	// Create health data
	healthData = &RedisHealthData{
		Version:     version,
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Initialize Redis connection
	RedisInit()

	// Check service status
	healthData.Service.Active = common.SystemdUnitActive("redis.service") || common.SystemdUnitActive("redis-server.service")
	if !healthData.Service.Active {
		common.AlarmCheckDown("redis_server_svc", "Service redis-server is not active", false, "", "")
	} else {
		common.AlarmCheckUp("redis_server_svc", "Service redis-server is now active", false)
	}

	// Check Redis role
	healthData.Role.IsMaster = RedisIsMaster()

	// Check if Sentinel is enabled
	isSentinel := RedisIsSentinel()
	if isSentinel {
		healthData.Sentinel = &SentinelInfo{
			Active:        common.SystemdUnitActive("redis-sentinel.service"),
			ExpectedCount: RedisHealthConfig.Slave_count,
		}

		if !healthData.Sentinel.Active {
			common.AlarmCheckDown("redis_sentinel", "Service redis-sentinel is not active", false, "", "")
		} else {
			common.AlarmCheckUp("redis_sentinel", "Service redis-sentinel is now active", false)
		}

		RedisSlaveCountChange()
	}

	// Check read/write capabilities
	healthData.Connection.Pingable = true  // Set by RedisInit()
	healthData.Connection.Writeable = true // Will be set by RedisReadWriteTest
	healthData.Connection.Readable = true  // Will be set by RedisReadWriteTest
	RedisReadWriteTest(isSentinel)

	// Render the health data
	fmt.Println(RenderRedisHealth(healthData))
}

// RenderRedisHealth renders the Redis health information using the new UI components
func RenderRedisHealth(data *RedisHealthData) string {
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

	// Role Status section
	sb.WriteString("\n")
	sb.WriteString(common.SectionTitle("Role Status"))
	sb.WriteString("\n")
	roleStatus := "slave"
	if data.Role.IsMaster {
		roleStatus = "master"
	}
	sb.WriteString(common.SimpleStatusListItem(
		"Redis Role",
		roleStatus,
		true))
	sb.WriteString("\n")

	// Sentinel Status section (if enabled)
	if data.Sentinel != nil {
		sb.WriteString("\n")
		sb.WriteString(common.SectionTitle("Sentinel Status"))
		sb.WriteString("\n")
		sb.WriteString(common.SimpleStatusListItem(
			"Redis Sentinel",
			"active",
			data.Sentinel.Active))
		sb.WriteString("\n")

		// Only show slave count if this is a master
		if data.Role.IsMaster {
			slaveCountStatus := fmt.Sprintf("%d/%d", data.Sentinel.SlaveCount, data.Sentinel.ExpectedCount)
			sb.WriteString(common.SimpleStatusListItem(
				"Slave Count",
				slaveCountStatus,
				data.Sentinel.SlaveCount == data.Sentinel.ExpectedCount))
			sb.WriteString("\n")
		}
	}

	// Wrap everything in a box
	return common.DisplayBox("Redis Health", sb.String())
}
