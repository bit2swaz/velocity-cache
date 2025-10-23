package config

import (
	"fmt"

	"github.com/spf13/viper"
)

const (
	configName = "velocity.config"
	configType = "json"
)

// Config is the top-level configuration struct.
type Config struct {
	RemoteCache RemoteCacheConfig       `mapstructure:"remote_cache"`
	Scripts     map[string]ScriptConfig `mapstructure:"scripts"`
}

// RemoteCacheConfig holds configuration for the S3 cache.
type RemoteCacheConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Bucket  string `mapstructure:"bucket"`
	Region  string `mapstructure:"region"`
}

// ScriptConfig defines a single script to be cached.
type ScriptConfig struct {
	Command string   `mapstructure:"command"`
	Inputs  []string `mapstructure:"inputs"`
	Outputs []string `mapstructure:"outputs"`
	EnvKeys []string `mapstructure:"env_keys"`
}

// Load reads velocity.config.json from the current directory and unmarshals it into Config.
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName(configName)
	v.SetConfigType(configType)
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
