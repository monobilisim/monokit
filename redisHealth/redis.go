//go:build linux && plugin

package redisHealth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var rdb *redis.Client
var ctx context.Context
var redisMaster bool

// DetectRedis detects if Redis service is available and running
func DetectRedis() bool {
	// Check if Redis (or Valkey) service is running
	if !common.SystemdUnitActive("redis.service") && !common.SystemdUnitActive("redis-server.service") && !common.SystemdUnitActive("valkey.service") && !common.SystemdUnitActive("valkey-server.service") {
		return false
	}

	// Try to initialize Redis connection
	tempRdb := redis.NewClient(&redis.Options{
		Addr:       "localhost:6379",
		Password:   "",
		DB:         0,
		MaxRetries: 1,
	})

	tempCtx := context.Background()
	ping, err := tempRdb.Ping(tempCtx).Result()
	if err == nil && ping == "PONG" {
		tempRdb.Close()
		return true
	}
	tempRdb.Close()
	return false
}

// InitRedis initializes the Redis connection and sets up context
func InitRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:       "localhost:" + fmt.Sprint(common.ConnsByProc("redis-server")),
		Password:   RedisHealthConfig.Password,
		DB:         0,
		MaxRetries: 5,
	})

	ctx = context.Background()

	ping, pingerr := rdb.Ping(ctx).Result()

	if pingerr != nil {
		rdb = redis.NewClient(&redis.Options{
			Addr:       "localhost:" + RedisHealthConfig.Port,
			Password:   RedisHealthConfig.Password,
			DB:         0,
			MaxRetries: 5,
		})

		ping, pingerr = rdb.Ping(ctx).Result()
	}

	if ping != "PONG" || pingerr != nil {
		log.Error().Err(pingerr).Str("component", "redisHealth").Str("operation", "InitRedis").Str("action", "ping_failed").Str("ping", ping).Msg("Error while trying to ping Redis")
		common.AlarmCheckDown("redis_ping", "Trying to ping Redis failed", false, "", "")
	} else {
		common.AlarmCheckUp("redis_ping", "Redis is pingable again", false)
	}
}

// CheckSlaveCountChange checks if the Redis slave count matches the expected count
func CheckSlaveCountChange() {
	if !redisMaster || !IsRedisSentinel() {
		return
	}

	info, err := rdb.Info(ctx, "Replication").Result()

	if err != nil {
		log.Error().Err(err).Str("component", "redisHealth").Str("operation", "CheckSlaveCountChange").Str("action", "info_gather_failed").Msg("Error while trying to gather replication info")
	}

	// Go over line by line
	scanner := bufio.NewScanner(strings.NewReader(info))

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "connected_slaves:") {
			break
		}
	}

	if scanner.Text() == "connected_slaves:"+strconv.Itoa(RedisHealthConfig.Slave_count) {
		common.AlarmCheckUp("redis_slave_count", "Slave count is now correct", false)
	} else {
		common.AlarmCheckDown("redis_slave_count", "Slave count is incorrect, intended: "+strconv.Itoa(RedisHealthConfig.Slave_count)+", actual: "+strings.Split(scanner.Text(), "connected_slaves:")[1], false, "", "")
	}
}

func redisAlarmRoleChange(isMaster bool) {
	// Check if file TmpPath + /redis_role exists
	if _, err := os.Stat(common.TmpDir + "/redis_role"); os.IsNotExist(err) {
		// File doesn't exist, create it and write the role and return

		// Write the role
		err := os.WriteFile(common.TmpDir+"/redis_role", []byte(fmt.Sprintf("%t", isMaster)), 0644)
		if err != nil {
			log.Error().Err(err).Str("component", "redisHealth").Str("operation", "redisAlarmRoleChange").Str("action", "file_write_failed").Msg("Error while writing to file")
			return
		}

		// Return
		return
	} else {

		// Remove the file common.TmpDir + "/redis_role.log" if it exists
		if _, err := os.Stat(common.TmpDir + "/redis_role.log"); err == nil {
			err := os.Remove(common.TmpDir + "/redis_role.log")
			if err != nil {
				log.Error().Err(err).Str("component", "redisHealth").Str("operation", "redisAlarmRoleChange").Str("action", "file_remove_failed").Msg("Error while removing redis_role.log")
			}
		}

		// File exists, read the role
		data, err := os.ReadFile(common.TmpDir + "/redis_role")
		if err != nil {
			log.Error().Err(err).Str("component", "redisHealth").Str("operation", "redisAlarmRoleChange").Str("action", "file_read_failed").Msg("Error while reading from file")
			return
		}

		// Check if the role is changed
		if string(data) == fmt.Sprintf("%t", isMaster) {
			// Role is not changed, return
			return
		} else {
			// Role is changed, write the new role and return
			err := os.WriteFile(common.TmpDir+"/redis_role", []byte(fmt.Sprintf("%t", isMaster)), 0644)

			if err != nil {
				log.Error().Err(err).Str("component", "redisHealth").Str("operation", "redisAlarmRoleChange").Str("action", "file_write_failed").Msg("Error while writing to file")
				return
			}

			if isMaster {
				message := "[" + common.ScriptName + " - " + common.Config.Identifier + "] [:check:] Redis role changed to master"
				common.Alarm(message, "", "", false)
			} else {
				message := "[" + common.ScriptName + " - " + common.Config.Identifier + "] [:red_circle:] Redis role changed to slave"
				common.Alarm(message, "", "", false)
			}
			return
		}
	}
}

// IsRedisMaster checks if the Redis instance is a master
func IsRedisMaster() bool {
	// Check INFO
	info, err := rdb.Info(ctx, "Replication").Result()

	if err != nil {
		log.Error().Err(err).Str("component", "redisHealth").Str("operation", "IsRedisMaster").Str("action", "info_gather_failed").Msg("Redis is not working")
		return false
	}

	// Go over line by line
	scanner := bufio.NewScanner(strings.NewReader(info))

	for scanner.Scan() {
		if scanner.Text() == "role:master" || scanner.Text() == "role:slave" {
			break
		}
	}

	if scanner.Text() == "role:master" {
		redisAlarmRoleChange(true)
		redisMaster = true
		return true
	} else if scanner.Text() == "role:slave" {
		redisAlarmRoleChange(false)
		redisMaster = false
		return false
	}

	return false
}

// TestRedisReadWrite tests Redis read and write capabilities
func TestRedisReadWrite(healthData *RedisHealthData, isSentinel bool) {
	err := rdb.Set(ctx, "redisHealth_foo", "bar", 0).Err()

	if err != nil && strings.Contains(err.Error(), "MOVED") {
		log.Debug().Str("component", "redisHealth").Str("operation", "TestRedisReadWrite").Str("action", "moved_request").Msg("MOVED request, trying to get the new address")
		// Get the new address
		newAddr := strings.Split(err.Error(), " ")[2]
		log.Debug().Str("component", "redisHealth").Str("operation", "TestRedisReadWrite").Str("action", "moved_request_new_address").Str("new_address", newAddr).Msg("MOVED request, new address: " + newAddr)

		// Reinitialize the client
		rdb = redis.NewClient(&redis.Options{
			Addr:       newAddr,
			Password:   RedisHealthConfig.Password,
			DB:         0,
			MaxRetries: 5,
		})

		// Run function again
		TestRedisReadWrite(healthData, isSentinel)
		return
	}

	if err != nil {
		if isSentinel {
			// Check if its master
			if redisMaster {
				log.Error().Err(err).Str("component", "redisHealth").Str("operation", "TestRedisReadWrite").Str("action", "write_failed").Msg("Can't Write to Redis (sentinel)")
				common.AlarmCheckDown("redis_write", "Trying to write a string to Redis failed", false, "", "")
				healthData.Connection.Writeable = false
				return
			} else {
				// It is a worker node, so we can't write to it
				healthData.Connection.Writeable = false
				return
			}
		} else {
			log.Error().Err(err).Str("component", "redisHealth").Str("operation", "TestRedisReadWrite").Str("action", "write_failed").Msg("Can't Write to Redis")
			common.AlarmCheckDown("redis_write", "Trying to write a string to Redis failed", false, "", "")
			healthData.Connection.Writeable = false
			return
		}
	} else {
		common.AlarmCheckUp("redis_write", "Redis is writeable again", false)
		healthData.Connection.Writeable = true
	}

	val, err := rdb.Get(ctx, "redisHealth_foo").Result()

	if err != nil {
		log.Error().Err(err).Str("component", "redisHealth").Str("operation", "TestRedisReadWrite").Str("action", "read_failed").Msg("Can't Read what is written to Redis")
		common.AlarmCheckDown("redis_read", "Trying to read string from Redis failed", false, "", "")
		healthData.Connection.Readable = false
		return
	} else {
		common.AlarmCheckUp("redis_read", "Successfully read string from Redis", false)
		healthData.Connection.Readable = true
	}

	if val != "bar" {
		common.AlarmCheckDown("redis_read_value", "The string that is read from Redis doesn't match the expected value", false, "", "")
		healthData.Connection.Readable = false
	} else {
		common.AlarmCheckUp("redis_read_value", "The Redis value now matches with the expected value", false)
		healthData.Connection.Readable = true
	}
}

// IsRedisSentinel is a function to check if Redis Sentinel is running
func IsRedisSentinel() bool {
	// Check if port 26379 is open
	rdb_sentinel := redis.NewClient(&redis.Options{
		Addr:       "localhost:" + fmt.Sprint(common.ConnsByProc("redis-sentinel")),
		Password:   "",
		DB:         0,
		MaxRetries: 5,
	})

	// Check INFO
	item, err := rdb_sentinel.Info(ctx, "Server").Result()

	if err != nil {
		return false
	}

	scanner := bufio.NewScanner(strings.NewReader(item))

	for scanner.Scan() {
		if scanner.Text() == "redis_mode:sentinel" {
			return true
		}
	}

	return false
}

// GetActualSlaveCount retrieves the actual number of connected slaves from Redis
func GetActualSlaveCount() int {
	if rdb == nil {
		return 0
	}

	info, err := rdb.Info(ctx, "Replication").Result()
	if err != nil {
		return 0
	}

	// Parse the replication info
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "connected_slaves:") {
			countStr := strings.TrimPrefix(line, "connected_slaves:")
			countStr = strings.TrimSpace(countStr)
			if count, err := strconv.Atoi(countStr); err == nil {
				return count
			}
		}
	}
	return 0
}
