//go:build linux

package main

import (
	"github.com/monobilisim/monokit/mysqlHealth"
	"github.com/monobilisim/monokit/pgsqlHealth"
	"github.com/monobilisim/monokit/pmgHealth"
	"github.com/monobilisim/monokit/postalHealth"
	"github.com/monobilisim/monokit/rmqHealth"
	"github.com/monobilisim/monokit/traefikHealth"
	"github.com/monobilisim/monokit/upCheck"
	"github.com/monobilisim/monokit/vaultHealth"
	"github.com/monobilisim/monokit/zimbraLdap"
	"github.com/spf13/cobra"
)

func ZimbraCommandAdd() {
	// var zimbraHealthCmd = &cobra.Command{
	// 	Use:   "zimbraHealth",
	// 	Short: "Zimbra Health",
	// 	Run:   zimbraHealth.Main,
	// }

	var zimbraLdapCmd = &cobra.Command{
		Use:   "zimbraLdap",
		Short: "Zimbra LDAP",
		Run:   zimbraLdap.Main, // Directly use the Main function which now matches the signature
	}

	RootCmd.AddCommand(zimbraLdapCmd)
	// RootCmd.AddCommand(zimbraHealthCmd)
}

func PgsqlCommandAdd() {
	var pgsqlHealthCmd = &cobra.Command{
		Use:   "pgsqlHealth",
		Short: "PostgreSQL Health",
		Run:   pgsqlHealth.Main,
	}

	RootCmd.AddCommand(pgsqlHealthCmd)
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

func TraefikCommandAdd() {
	var traefikHealthCmd = &cobra.Command{
		Use:   "traefikHealth",
		Short: "Traefik Health",
		Run:   traefikHealth.Main,
	}

	RootCmd.AddCommand(traefikHealthCmd)
}

func VaultCommandAdd() {
	var vaultHealthCmd = &cobra.Command{
		Use:   "vaultHealth",
		Short: "Vault Health",
		Run:   vaultHealth.Main,
	}

	RootCmd.AddCommand(vaultHealthCmd)
}

func UpCheckCommandAdd() {
	var upCheckCmd = &cobra.Command{
		Use:   "upCheck",
		Short: "Generic systemd service uptime checker",
		Run:   upCheck.Main,
	}

	RootCmd.AddCommand(upCheckCmd)
}
