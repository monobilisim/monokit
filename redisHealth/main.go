package redisHealth

import (
    "fmt"
    "time"
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/common"
)


var RedisHealthConfig struct {
    Slave_count int
}

func Main(cmd *cobra.Command, args []string) {
    version := "0.1.0"
    common.ScriptName = "redisHealth"
    common.TmpDir = common.TmpDir + "redisHealth"
    common.Init()
    common.ConfInit("redis", &RedisHealthConfig)

    RedisInit()

    fmt.Println("Redis Health - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

    common.SplitSection("Main")

    RedisMaster = RedisIsMaster()

    RedisPing()

    if SystemdUnitActive("redis.service") == false && SystemdUnitActive("redis-server.service") == false {
        common.PrettyPrintStr("Service redis-server", false, "active")
    } else {
        common.PrettyPrintStr("Service redis-server", true, "active")
    }

    IsSentinel := RedisIsSentinel()
    
    if IsSentinel {
        common.SplitSection("Sentinel")
        
        if SystemdUnitActive("redis-sentinel.service") == false {
            common.PrettyPrintStr("Service redis-sentinel", false, "active")
            common.AlarmCheckDown("redis_sentinel", "Service redis-sentinel is not active")
        } else {
            common.PrettyPrintStr("Service redis-sentinel", true, "active")
            common.AlarmCheckUp("redis_sentinel", "Service redis-sentinel is now active")
        }
    
        RedisSlaveCountChange()
    }
    
    RedisReadWriteTest(IsSentinel)

}
