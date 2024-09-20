package redisHealth

import (
    "github.com/redis/go-redis/v9"
    "github.com/monobilisim/monokit/common"
    "context"
    "fmt"
    "os"
    "regexp"
)

var rdb *redis.Client
var ctx context.Context

func RedisInit() {
    rdb = redis.NewClient(&redis.Options{
        Addr: "localhost:6379", // TODO: Make this dynamic
        Password: "",
        DB: 0,
        MaxRetries: 5,
    })
    
    ctx = context.Background()
}

func redisAlarmRoleChange(isMaster bool) {
    // Check if file TmpPath + /redis_role exists
    if _, err := os.Stat(common.TmpDir + "/redis_role"); os.IsNotExist(err) {
        // File doesn't exist, create it and write the role and return
        
        // Write the role
        err := os.WriteFile(common.TmpDir + "/redis_role", []byte(fmt.Sprintf("%t", isMaster)), 0644)
        if err != nil {
            common.LogError("Error while writing to file: " + err.Error())
            return
        }
        
        // Return
        return
    } else {
        // File exists, read the role
        data, err := os.ReadFile(common.TmpDir + "/redis_role")
        if err != nil {
            common.LogError("Error while reading from file: " + err.Error())
            return
        }

        // Check if the role is changed
        if string(data) == fmt.Sprintf("%t", isMaster) {
            // Role is not changed, return
            return
        } else {
            // Role is changed, write the new role and return
            err := os.WriteFile(common.TmpDir + "/redis_role", []byte(fmt.Sprintf("%t", isMaster)), 0644)
            
            if err != nil {
                common.LogError("Error while writing to file: " + err.Error())
                return
            }

            if isMaster {
                common.AlarmCheckUp("redis_role", "Redis role changed to master")
            } else {
                common.AlarmCheckDown("redis_role", "Redis role changed to slave")
            }
            return
        }
    }
}

func RedisIsMaster() bool {
    // Check INFO
    info, err := rdb.Info(ctx, "Replication").Result()

    if err != nil {
        common.LogError("Redis is not working: " + err.Error())
        return false
    }

    if info != "role:master" {
        common.PrettyPrintStr("Role", true, "master")
        redisAlarmRoleChange(false)
        return false
    } else {
        common.PrettyPrintStr("Role", true, "slave")
        redisAlarmRoleChange(true)
        return true
    }
}

func RedisPing() {
    // Check PING
    ping, err := rdb.Ping(ctx).Result()

    if err != nil {
        common.LogError("Error while trying to ping Redis: " + err.Error())
        common.PrettyPrintStr("Redis", false, "pingable")
        common.AlarmCheckDown("redis_ping", "Trying to ping Redis failed, error;\n```\n" + err.Error() + "\n```")
    }

    if ping != "PONG" {
        common.PrettyPrintStr("Redis", false, "pingable")
        common.AlarmCheckDown("redis_ping", "Trying to ping Redis failed")
    } else {
        common.PrettyPrintStr("Redis", true, "pingable")
        common.AlarmCheckUp("redis_ping", "Redis is pingable again")
    }
}


func RedisReadWriteTest(isSentinel bool) {
    err := rdb.Set(ctx, "redisHealth_foo", "bar", 0).Err()

    if err != nil {
        if isSentinel {
            // Check if its master
            if RedisIsMaster() {
                common.LogError("Can't Write to Redis (sentinel): " + err.Error())
                common.PrettyPrintStr("Redis", false, "writeable")
                common.AlarmCheckDown("redis_write", "Trying to write a string to Redis failed")
                return
            } else {
                // It is a worker node, so we can't write to it
                return
            }
        } else {
            common.LogError("Can't Write to Redis: " + err.Error())
            common.PrettyPrintStr("Redis", false, "writeable")
            common.AlarmCheckDown("redis_write", "Trying to write a string to Redis failed")
            return
        }
    } else {
        common.PrettyPrintStr("Redis", true, "writeable")
        common.AlarmCheckUp("redis_write", "Redis is writeable again")
    }

    val, err := rdb.Get(ctx, "redisHealth_foo").Result()

    if err != nil {
        common.LogError("Can't Read what is written to Redis: " + err.Error())
        common.PrettyPrintStr("Redis", false, "readable")
        common.AlarmCheckDown("redis_read", "Trying to read string from Redis failed")
        return
    } else {
        common.AlarmCheckUp("redis_read", "Successfully read string from Redis")
    }

    if val != "bar" {
        common.PrettyPrintStr("Redis", false, "readable")
        common.AlarmCheckDown("redis_read_value", "The string that is read from Redis doesn't match the expected value")
    } else {
        common.PrettyPrintStr("Redis", true, "readable")
        common.AlarmCheckUp("redis_read_value", "The Redis value now matches with the expected value")
    }
}


func RedisIsSentinel() bool {
    // Check if port 26379 is open
    rdb_sentinel := redis.NewClient(&redis.Options{
        Addr: "localhost:26379", // TODO: Make this dynamic
        Password: "",
        DB: 0,
        MaxRetries: 5,
    })

    // Check INFO
    item, err := rdb_sentinel.Info(ctx, "Server").Result()
    
    if err != nil {
        return false
    }

    regex := regexp.MustCompile("^redis_mode:sentinel.*$")
        
    if regex.MatchString(item) {
        return false
    } else {
        return true
    }
}
