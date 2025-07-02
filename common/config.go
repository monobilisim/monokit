package common

import (
	"os"
	"strings"

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

	// Get all settings and expand environment variables recursively
	allSettings := viper.AllSettings()
	expandEnvInMap(allSettings)

	// Reset viper with expanded values
	for key, value := range allSettings {
		viper.Set(key, value)
	}

	err = viper.Unmarshal(&config)

	if err != nil {
		LogError("Fatal error while trying to unmarshal the config file: \n" + err.Error())
		panic(err)
	}

	return config
}

// expandEnvInMap recursively expands environment variables in nested map structures
func expandEnvInMap(data map[string]interface{}) {
	for key, value := range data {
		switch v := value.(type) {
		case string:
			if strings.Contains(v, "${") || strings.Contains(v, "$") {
				data[key] = os.ExpandEnv(v)
			}
		case map[string]interface{}:
			expandEnvInMap(v)
		case []interface{}:
			for i, item := range v {
				if strItem, ok := item.(string); ok {
					if strings.Contains(strItem, "${") || strings.Contains(strItem, "$") {
						v[i] = os.ExpandEnv(strItem)
					}
				}
			}
		}
	}
}
