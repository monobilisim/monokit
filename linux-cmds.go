//go:build linux
package main

import (
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/pmgHealth"
    "github.com/monobilisim/monokit/rmqHealth"
    "github.com/monobilisim/monokit/redisHealth"
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

func PmgCommandAdd() {
    var pmgHealthCmd = &cobra.Command{
        Use:   "pmgHealth",
        Short: "Proxmox Mail Gateway Health",
        Run: pmgHealth.Main,
    }

    RootCmd.AddCommand(pmgHealthCmd)
}
