package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the velocity.yml structure
type Config struct {
	Version   int                   `yaml:"version"`
	ProjectID string                `yaml:"project_id"`
	Remote    RemoteConfig          `yaml:"remote"`
	Packages  []string              `yaml:"packages"`
	Pipeline  map[string]TaskConfig `yaml:"pipeline"` // Renamed from 'Tasks'
}

type RemoteConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Token   string `yaml:"token"`
}

type TaskConfig struct {
	Command   string   `yaml:"command"`
	Inputs    []string `yaml:"inputs"`
	Outputs   []string `yaml:"outputs"`
	DependsOn []string `yaml:"depends_on"`
	EnvKeys   []string `yaml:"env_keys"`
}

// Load reads velocity.yml, expands env vars, and parses YAML.
func Load() (*Config, error) {
	// Look for velocity.yml
	path := "velocity.yml"

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand ${VAR} environment variables in the YAML
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
