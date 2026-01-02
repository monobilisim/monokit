package common

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog/log"
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

		Api_key     string
		Url         string
		Display_url string
	}
}

func ConfExists(configName string) bool {
	yamlFiles := [2]string{configName + ".yaml", configName + ".yml"}
	var configPaths []string

	if runtime.GOOS == "windows" {
		exePath, err := os.Executable()
		if err == nil {
			configPaths = append(configPaths, filepath.Dir(exePath)+"\\config")
			configPaths = append(configPaths, filepath.Dir(exePath))
		}
		configPaths = append(configPaths, "C:\\ProgramData\\mono")
	} else {
		configPaths = append(configPaths, "/etc/mono")
	}

	for _, path := range configPaths {
		for _, file := range yamlFiles {
			// Check if the file exists
			if _, err := os.Stat(filepath.Join(path, file)); err == nil {
				return true
			}
		}
	}

	return false
}

func ConfInit(configName string, config interface{}) interface{} {
	viper.SetConfigName(configName)

	if runtime.GOOS == "windows" {
		// On Windows, look in the directory of the executable, and ProgramData
		exePath, err := os.Executable()
		if err == nil {
			viper.AddConfigPath(filepath.Dir(exePath) + "\\config")
			viper.AddConfigPath(filepath.Dir(exePath))
		}
		viper.AddConfigPath("C:\\ProgramData\\mono")
	} else {
		viper.AddConfigPath("/etc/mono")
	}

	viper.SetConfigType("yaml")

	viper.SetDefault("alarm.interval", 3)
	// Daemon-specific defaults
	if configName == "daemon" {
		viper.SetDefault("monokit_upgrade", true)
	}

	err := viper.ReadInConfig()

	if err != nil {
		log.Error().Str("configName", configName).Err(err).Msg("Fatal error while trying to parse the config file")
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
		log.Error().Str("configName", configName).Err(err).Msg("Fatal error while trying to unmarshal the config file")
		panic(err)
	}

	return config
}

func GetRedmineDisplayUrl() string {
	if Config.Redmine.Display_url != "" {
		return Config.Redmine.Display_url
	}
	return strings.Replace(Config.Redmine.Url, "://api.", "://", 1)
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
