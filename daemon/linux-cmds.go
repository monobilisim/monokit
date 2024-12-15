//go:build linux

package daemon

import (
	"github.com/monobilisim/monokit/mysqlHealth"
	"github.com/monobilisim/monokit/pmgHealth"
	"github.com/monobilisim/monokit/postalHealth"
	"github.com/monobilisim/monokit/redisHealth"
	"github.com/monobilisim/monokit/rmqHealth"
	"github.com/monobilisim/monokit/traefikHealth"
	"github.com/spf13/cobra"
)

func RedisCommandExecute() {
	var redisHealthCmd = &cobra.Command{
		Run:   redisHealth.Main,
        DisableFlagParsing: true,
	}

    redisHealthCmd.Execute()
}

func MysqlCommandExecute() {
	var mysqlHealthCmd = &cobra.Command{
		Run:   mysqlHealth.Main,
        DisableFlagParsing: true,
	}

    mysqlHealthCmd.Execute()
}

func RmqCommandExecute() {
	var rmqHealthCmd = &cobra.Command{
		Run:   rmqHealth.Main,
        DisableFlagParsing: true,
	}

	rmqHealthCmd.Execute()
}

func PmgCommandExecute() {
	var pmgHealthCmd = &cobra.Command{
		Run:   pmgHealth.Main,
        DisableFlagParsing: true,
	}

	pmgHealthCmd.Execute()
}

func PostalCommandExecute() {
	var postalHealthCmd = &cobra.Command{
		Run:   postalHealth.Main,
        DisableFlagParsing: true,
	}

	postalHealthCmd.Execute()
}

func TraefikCommandExecute() {
	var traefikHealthCmd = &cobra.Command{
		Run:   traefikHealth.Main,
        DisableFlagParsing: true,
	}

	traefikHealthCmd.Execute()
}
