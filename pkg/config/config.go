package config

import (
	"encoding/json"
	"os"
)

// Config holds the application configuration.
type Config struct {
	GNMIPort int `json:"gnmi_port"`
	Mock     MockConfig `json:"mock_installer"`
}

// MockConfig holds configuration for the mock route installer.
type MockConfig struct {
	Enabled    bool `json:"enabled"`
	RouteCount int  `json:"route_count"`
	ChurnRate  int  `json:"churn_rate"` // Updates per second
}

// Load reads configuration from a file.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		GNMIPort: 50099,
		Mock: MockConfig{
			Enabled:    true,
			RouteCount: 1000,
			ChurnRate:  100,
		},
	}
}
