package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all settings loaded from ~/.scout/config.toml.
type Config struct {
	Ignore IgnoreConfig `toml:"ignore"`
}

// IgnoreConfig holds user-defined exclusion rules.
type IgnoreConfig struct {
	Dirs       []string `toml:"dirs"`
	Extensions []string `toml:"extensions"`
}

// Load reads ~/.scout/config.toml.
// If the file does not exist, a zero-value Config is returned without error.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	p := filepath.Join(home, ".scout", "config.toml")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return Config{}, nil
	}

	var cfg Config
	if _, err := toml.DecodeFile(p, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
