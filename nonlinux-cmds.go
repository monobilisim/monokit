//go:build !linux
package main

func RedisCommandAdd() {
    // redisHealth is not supported on anything other than Linux
    return
}
