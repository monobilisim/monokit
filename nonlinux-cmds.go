//go:build !linux

package main

func MysqlCommandAdd() {
	// mysqlHealth is not supported on anything other than Linux
}

func PgsqlCommandAdd() {
	// pgsqlHealth is not supported on anything other than Linux
}

func RmqCommandAdd() {
	// rmqHealth is not supported on anything other than Linux
}

func PmgCommandAdd() {
	// pmgHealth is not supported on anything other than Linux
}

func PostalCommandAdd() {
	// postalHealth is not supported on anything other than Linux
}

func TraefikCommandAdd() {
	// traefikHealth is not supported on anything other than Linux
}

func ZimbraCommandAdd() {
	// zimbraHealth is not supported on anything other than Linux
}

func VaultCommandAdd() {
	// vaultHealth is not supported on anything other than Linux
}

func UpCheckCommandAdd() {
    // upCheck is not supported on anything other than Linux
}
