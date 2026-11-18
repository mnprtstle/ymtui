// Package config loads the user's settings from a JSON file in the XDG
// config directory. Every field is optional; missing ones keep their default.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// Config is the contents of config.json.
type Config struct {
	Volume int   `json:"volume"` // initial volume, 0-100
	Theme  Theme `json:"theme"`
}

// Theme overrides palette colors with "#RRGGBB" values. An empty string
// keeps the built-in color.
type Theme struct {
	Accent string `json:"accent"`
	Amber  string `json:"amber"`
}

// Default returns the settings used when there is no config file.
func Default() Config { return Config{Volume: 100} }

// DefaultPath is $XDG_CONFIG_HOME/ymtui/config.json, falling back to
// ~/.config/ymtui/config.json.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find config dir: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "ymtui", "config.json"), nil
}

// Load reads path. A missing file is not an error: it yields the defaults.
// A malformed file returns the defaults together with an error, so the caller
// can warn and still start.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Default(), fmt.Errorf("read config: %w", err)
	}
	// Decoding into a pre-filled struct leaves absent fields untouched.
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return Default(), fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (c Config) validate() error {
	if c.Volume < 0 || c.Volume > 100 {
		return fmt.Errorf("volume %d out of range 0-100", c.Volume)
	}
	for name, v := range map[string]string{"accent": c.Theme.Accent, "amber": c.Theme.Amber} {
		if v != "" && !hexColor.MatchString(v) {
			return fmt.Errorf("theme.%s %q is not a #RRGGBB color", name, v)
		}
	}
	return nil
}
