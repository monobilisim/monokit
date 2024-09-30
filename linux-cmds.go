//go:build linux
package main

import (
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/redisHealth"
    "github.com/monobilisim/monokit/rmqHealth"
)

func RedisCommandAdd() {
    var redisHealthCmd = &cobra.Command{
        Use:   "redisHealth",
        Short: "Redis Health",
        Run: redisHealth.Main,
    }

    RootCmd.AddCommand(redisHealthCmd)
}

func RmqCommandAdd() {
    var rmqHealthCmd = &cobra.Command{
        Use:   "rmqHealth",
        Short: "RabbitMQ Health",
        Run: rmqHealth.Main,
    }

    RootCmd.AddCommand(rmqHealthCmd)
}
