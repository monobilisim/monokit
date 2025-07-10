//go:build test
// +build test

package models

import "github.com/spf13/viper"

func init() {
	viper.Reset()
	viper.SetConfigName("global")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../config")    // when running tests from common/api
	viper.AddConfigPath("../../../config") // when running tests from common/api/tests
}
