package common

import (
    "github.com/spf13/viper"
)

type Common struct {
    Identifier string

    Alarm struct {
        Enabled bool
        Interval float64
        Webhook_urls []string
    }
    
    Redmine struct {
        Enabled bool
        Project_id string
        Tracker_id int
        Status_id int
        Priority_id int

        Api_key string
        Url string
    }
}


func ConfInit(configName string, config interface{}) interface{} {
    viper.SetConfigName(configName)
    viper.AddConfigPath("/etc/mono.sh")
    viper.SetConfigType("yaml")

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
