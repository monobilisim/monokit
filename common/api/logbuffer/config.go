package logbuffer

import (
	"time"

	"github.com/spf13/viper"
)

// Config holds the configuration for the log buffer.
type Config struct {
	BatchSize     int           `mapstructure:"batch_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
	MaxBacklog    int           `mapstructure:"max_backlog"`
}

// LoadConfig loads the log buffer configuration from the "server" config file.
func LoadConfig() Config {
	// Set default values
	viper.SetDefault("logbuffer.batch_size", 100)
	viper.SetDefault("logbuffer.flush_interval", 3*time.Second)
	viper.SetDefault("logbuffer.max_backlog", 10000)

	return Config{
		BatchSize:     viper.GetInt("logbuffer.batch_size"),
		FlushInterval: viper.GetDuration("logbuffer.flush_interval"),
		MaxBacklog:    viper.GetInt("logbuffer.max_backlog"),
	}
}
