package common

import (
	"os"

	"github.com/spf13/viper"
)

type Common struct {
	Identifier string

	Components []struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	} `json:"components"`

	Alarm struct {
		Enabled      bool
		Interval     float64
		Webhook_urls []string
	}

	Redmine struct {
		Enabled     bool
		Project_id  string
		Tracker_id  int
		Status_id   int
		Priority_id int
		Interval    float64

		Api_key string
		Url     string
	}
}

func ConfExists(configName string) bool {
	yamlFiles := [2]string{configName + ".yaml", configName + ".yml"}

	for _, file := range yamlFiles {
		// Check if the file exists
		if _, err := os.Stat("/etc/mono/" + file); err == nil {
			return true
		}
	}

	return false
}

func ConfInit(configName string, config interface{}) interface{} {
	viper.SetConfigName(configName)
	viper.AddConfigPath("/etc/mono")
	viper.SetConfigType("yaml")

	viper.SetDefault("alarm.interval", 3)

	err := viper.ReadInConfig()

	if err != nil {
		LogError("Fatal error while trying to parse the config file: \n" + err.Error())
		panic(err)
	}

	err = viper.Unmarshal(&config)

	if err != nil {
		LogError("Fatal error while trying to unmarshal the config file: \n" + err.Error())
		panic(err)
	}

	return config
}
