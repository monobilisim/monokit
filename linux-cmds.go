//go:build linux

package main

import (
	"github.com/monobilisim/monokit/mysqlHealth"
	"github.com/monobilisim/monokit/pmgHealth"
	"github.com/monobilisim/monokit/postalHealth"
	"github.com/monobilisim/monokit/redisHealth"
	"github.com/monobilisim/monokit/rmqHealth"
	"github.com/spf13/cobra"
)

func RedisCommandAdd() {
	var redisHealthCmd = &cobra.Command{
		Use:   "redisHealth",
		Short: "Redis Health",
		Run:   redisHealth.Main,
	}

	RootCmd.AddCommand(redisHealthCmd)
}

func MysqlCommandAdd() {
	var mysqlHealthCmd = &cobra.Command{
		Use:   "mysqlHealth",
		Short: "MySQL Health",
		Run:   mysqlHealth.Main,
	}

	RootCmd.AddCommand(mysqlHealthCmd)
}

func RmqCommandAdd() {
	var rmqHealthCmd = &cobra.Command{
		Use:   "rmqHealth",
		Short: "RabbitMQ Health",
		Run:   rmqHealth.Main,
	}

	RootCmd.AddCommand(rmqHealthCmd)
}

func PmgCommandAdd() {
	var pmgHealthCmd = &cobra.Command{
		Use:   "pmgHealth",
		Short: "Proxmox Mail Gateway Health",
		Run:   pmgHealth.Main,
	}

	RootCmd.AddCommand(pmgHealthCmd)
}

func PostalCommandAdd() {
	var postalHealthCmd = &cobra.Command{
		Use:   "postalHealth",
		Short: "Postal Health",
		Run:   postalHealth.Main,
	}

	RootCmd.AddCommand(postalHealthCmd)
}
