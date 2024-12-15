//go:build !linux

package daemon

func RedisCommandExecute() {
	// redisHealth is not supported on anything other than Linux
	return
}

func MysqlCommandExecute() {
	// mysqlHealth is not supported on anything other than Linux
	return
}

func RmqCommandExecute() {
	// rmqHealth is not supported on anything other than Linux
	return
}

func PmgCommandExecute() {
	// pmgHealth is not supported on anything other than Linux
	return
}

func PostalCommandExecute() {
	// postalHealth is not supported on anything other than Linux
	return
}

func TraefikCommandExecute() {
	// traefikHealth is not supported on anything other than Linux
	return
}
