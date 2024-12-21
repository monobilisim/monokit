//go:build !linux

package main

func RedisCommandAdd() {
	// redisHealth is not supported on anything other than Linux
	return
}

func MysqlCommandAdd() {
	// mysqlHealth is not supported on anything other than Linux
	return
}

func PgsqlCommandAdd() {
    // pgsqlHealth is not supported on anything other than Linux
    return
}

func RmqCommandAdd() {
	// rmqHealth is not supported on anything other than Linux
	return
}

func PmgCommandAdd() {
	// pmgHealth is not supported on anything other than Linux
	return
}

func PostalCommandAdd() {
	// postalHealth is not supported on anything other than Linux
	return
}

func TraefikCommandAdd() {
	// traefikHealth is not supported on anything other than Linux
	return
}

func ZimbraCommandAdd() {
    // zimbraHealth is not supported on anything other than Linux
    return
}
