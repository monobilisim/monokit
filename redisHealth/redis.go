//go:build linux
package redisHealth

import (
    "github.com/redis/go-redis/v9"
    "github.com/monobilisim/monokit/common"
    "context"
    "fmt"
    "os"
    "strings"
    "bufio"
    "strconv"
)

var rdb *redis.Client
var ctx context.Context
var RedisMaster bool

func RedisInit() {
    rdb = redis.NewClient(&redis.Options{
        Addr: "localhost:" + fmt.Sprint(common.ConnsByProc("redis-server")),
        Password: RedisHealthConfig.Password,
        DB: 0,
        MaxRetries: 5,
    })
    
    ctx = context.Background()
}

func RedisSlaveCountChange() {
    if RedisMaster == false || RedisIsSentinel() == false {
        return
    }

    info, err := rdb.Info(ctx, "Replication").Result()

    if err != nil {
        common.LogError("Error while trying to gather replication info: " + err.Error())
    }

    // Go over line by line

    scanner := bufio.NewScanner(strings.NewReader(info))

    for scanner.Scan() {
        if strings.Contains(scanner.Text(), "connected_slaves:") {
            break
        }
    }


    if scanner.Text() == "connected_slaves:" + strconv.Itoa(RedisHealthConfig.Slave_count) {
        common.PrettyPrintStr("Slave Count", true, strconv.Itoa(RedisHealthConfig.Slave_count))
        common.AlarmCheckUp("redis_slave_count", "Slave count is now correct")
    } else {
        common.PrettyPrintStr("Slave Count", false, strconv.Itoa(RedisHealthConfig.Slave_count))
        common.AlarmCheckDown("redis_slave_count", "Slave count is incorrect, intended: " + strconv.Itoa(RedisHealthConfig.Slave_count) + ", actual: " + strings.Split(scanner.Text(), "connected_slaves:")[1])
    }

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
    
    // Go over line by line

    scanner := bufio.NewScanner(strings.NewReader(info))

    for scanner.Scan() {
        if scanner.Text() == "role:master" || scanner.Text() == "role:slave" {
            break
        }
    }

    if scanner.Text() == "role:master" {
        common.PrettyPrintStr("Role", true, "master")
        redisAlarmRoleChange(true)
        return true
    } else if scanner.Text() == "role:slave" {
        common.PrettyPrintStr("Role", true, "slave")
        redisAlarmRoleChange(false)
        return false
    }

    return false

}

func RedisPing() {
    // Check PING
    ping, err := rdb.Ping(ctx).Result()

    if err != nil {
        common.LogError("Error while trying to ping Redis: " + err.Error())
        common.PrettyPrintStr("Redis", false, "pingable")
        common.AlarmCheckDown("redis_ping", "Trying to ping Redis failed, Error;\n```\n" + err.Error() + "\n```")
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
            if RedisMaster == true {
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
        Addr: "localhost:" + fmt.Sprint(common.ConnsByProc("redis-sentinel")),
        Password: "",
        DB: 0,
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