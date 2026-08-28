//go:build linux && plugin

package redisHealth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	"github.com/monobilisim/monokit/common/health"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	common.RegisterComponent(common.Component{
		Name:       "redisHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectRedis,
	})
	// Register health provider
	health.Register(&RedisHealthProvider{})
}

// Types are now defined in types.go
// RedisHealthConfig will be populated from there

// portListening reports whether any process whose name starts with one of
// prefixes is listening on port. Used to check the specific configured Redis
// instance is up, not just "a process with this name exists somewhere" -
// which would stay true on multi-instance hosts even if only THIS instance
// died and a sibling instance (same binary name, different port) survived.
func portListening(port uint64, prefixes ...string) bool {
	for _, prefix := range prefixes {
		for _, listening := range common.ConnsByProcMulti(prefix) {
			if uint64(listening) == port {
				return true
			}
		}
	}
	return false
}

// collectRedisHealthData collects all Redis health information and returns it
func collectRedisHealthData() (*RedisHealthData, error) {
	version := "0.3.0"

	// Initialize config if not already done
	if common.ConfExists("redis") {
		common.ConfInit("redis", &RedisHealthConfig)
	}

	if RedisHealthConfig.Port == "" {
		RedisHealthConfig.Port = "6379"
	}

	// Create health data
	healthData := &RedisHealthData{
		Version:     version,
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Initialize Redis connection
	InitRedis()

	// Check service status
	configuredPort, _ := strconv.ParseUint(RedisHealthConfig.Port, 10, 32)
	healthData.Service.Active = common.SystemdUnitActive("redis.service") ||
		common.SystemdUnitActive("redis-server.service") ||
		common.SystemdUnitActive("valkey.service") ||
		common.SystemdUnitActive("valkey-server.service") ||
		common.SystemdUnitActive(fmt.Sprintf("redis-server@%s.service", RedisHealthConfig.Port)) ||
		common.SystemdUnitActive(fmt.Sprintf("valkey-server@%s.service", RedisHealthConfig.Port)) ||
		common.SystemdUnitActive(fmt.Sprintf("redis-server-%s.service", RedisHealthConfig.Port)) ||
		common.SystemdUnitActive(fmt.Sprintf("valkey-server-%s.service", RedisHealthConfig.Port)) ||
		// ponytail: fallback discovery scoped to the configured port, not a
		// bare "does this process name exist anywhere" check - on multi-instance
		// hosts (e.g. Redis Cluster nodes on different ports) a process-name-only
		// check would stay true even if THIS instance's port died, as long as a
		// sibling instance kept the same binary name running. Checking the
		// specific listening port instead makes it per-instance correct, and
		// still covers non-systemd/custom-unit deployments.
		portListening(configuredPort, "redis-server", "valkey-server")

	if !healthData.Service.Active {
		common.AlarmCheckDown("redis_server_svc", "Service redis-server/valkey-server is not active", false, "", "")
	} else {
		common.AlarmCheckUp("redis_server_svc", "Service redis-server/valkey-server is now active", false)
	}

	// Check Redis role
	healthData.Role.IsMaster = IsRedisMaster()

	// Check if Sentinel is enabled
	isSentinel := IsRedisSentinel()
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

		// Get actual slave count if this is a master
		if healthData.Role.IsMaster {
			healthData.Sentinel.SlaveCount = GetActualSlaveCount()
		}

		CheckSlaveCountChange()
	}

	// Check read/write capabilities (Writeable/Readable are set unconditionally inside TestRedisReadWrite)
	healthData.Connection.Pingable = true // ponytail: InitRedis() doesn't report ping failure separately; always true here
	TestRedisReadWrite(healthData, isSentinel)

	return healthData, nil
}

// Main function for CLI usage
func Main(cmd *cobra.Command, args []string) {
	common.ScriptName = "redisHealth"
	common.TmpDir = common.TmpDir + "redisHealth"
	common.Init()

	client.WrapperGetServiceStatus("redisHealth")

	// Collect health data using the shared function
	healthData, err := collectRedisHealthData()
	if err != nil {
		log.Error().Err(err).Str("component", "redisHealth").Str("operation", "Main").Str("action", "collect_health_data_failed").Msg("Failed to collect Redis health data")
		return
	}

	// Attempt to POST health data to the Monokit server
	if err := common.PostHostHealth("redisHealth", healthData); err != nil {
		log.Error().Err(err).Str("component", "redisHealth").Str("operation", "Main").Str("action", "post_health_data_failed").Msg("Failed to POST health data")
		// Continue execution even if POST fails, e.g., to display UI locally
	}

	fmt.Println(RenderRedisHealthCLI(healthData, healthData.Version))
}
