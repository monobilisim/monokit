//go:build linux
package main

import (
    "github.com/spf13/cobra"
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
