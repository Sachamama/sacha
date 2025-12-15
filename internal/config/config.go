package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultService = "cloudwatch-logs"
	configDirName  = "sacha"
	configFileName = "config.json"
)

// Config represents the persisted user configuration.
type Config struct {
	DefaultProfile string `json:"defaultProfile,omitempty"`
	DefaultRegion  string `json:"defaultRegion,omitempty"`
	LastRegion     string `json:"lastRegion,omitempty"`
	LastService    string `json:"lastService,omitempty"`
}

// RuntimeConfig resolves configuration after applying precedence rules.
type RuntimeConfig struct {
	Profile string
	Region  string
	Service string
}

// Flags captures CLI flag values.
type Flags struct {
	Profile string
	Region  string
	Service string
}

// Env captures supported environment variables.
type Env struct {
	Profile string
	Region  string
}

// DefaultRuntime builds the runtime config when no inputs are provided.
func DefaultRuntime() RuntimeConfig {
	return RuntimeConfig{
		Service: defaultService,
	}
}

// DefaultPath returns the location of the config file using OS conventions.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(dir, configDirName, configFileName), nil
}

// Load reads the config file if present; a missing file is not an error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Save persists the config to disk, creating directories as needed.
func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Resolve merges configuration using the precedence:
// 1) CLI flags, 2) environment, 3) config file, 4) defaults.
func Resolve(flags Flags, env Env, cfg *Config) RuntimeConfig {
	result := DefaultRuntime()

	// Profile
	switch {
	case flags.Profile != "":
		result.Profile = flags.Profile
	case env.Profile != "":
		result.Profile = env.Profile
	case cfg != nil && cfg.DefaultProfile != "":
		result.Profile = cfg.DefaultProfile
	}

	// Region
	switch {
	case flags.Region != "":
		result.Region = flags.Region
	case env.Region != "":
		result.Region = env.Region
	case cfg != nil:
		if cfg.LastRegion != "" {
			result.Region = cfg.LastRegion
		} else {
			result.Region = cfg.DefaultRegion
		}
	}

	// Service
	switch {
	case flags.Service != "":
		result.Service = flags.Service
	case cfg != nil && cfg.LastService != "":
		result.Service = cfg.LastService
	default:
		result.Service = defaultService
	}

	return result
}

// FromEnv reads relevant AWS environment variables.
func FromEnv() Env {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	return Env{
		Profile: os.Getenv("AWS_PROFILE"),
		Region:  region,
	}
}
