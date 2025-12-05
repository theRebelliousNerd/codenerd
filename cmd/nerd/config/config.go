package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user preferences
type Config struct {
	APIKey         string `json:"api_key"`
	Theme          string `json:"theme"`           // "light" or "dark"
	Context7APIKey string `json:"context7_api_key"` // Context7 API key for research
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Theme: "light",
	}
}

// ConfigDir returns the directory where config is stored
func ConfigDir() (string, error) {
	// Prefer project-local .nerd directory if present or creatable
	if cwd, err := os.Getwd(); err == nil {
		localDir := filepath.Join(cwd, ".nerd")
		if stat, err := os.Stat(localDir); (err == nil && stat.IsDir()) || os.IsNotExist(err) {
			return localDir, nil
		}
	}

	// Fallback to home-level config
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codenerd"), nil
}

// ConfigFile returns the full path to the config file
func ConfigFile() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the configuration from disk
func Load() (Config, error) {
	path, err := ConfigFile()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return DefaultConfig(), err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}

	return cfg, nil
}

// Save writes the configuration to disk
func Save(cfg Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := ConfigFile()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
