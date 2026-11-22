// Package config reads the app's settings file: which project folders to watch and how to show
// them. Nothing about how events are written belongs here; that is Waypoint's contract.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Project is one watched project folder.
type Project struct {
	Path string `toml:"path"`
	Name string `toml:"name"`
}

// Config is the settings file.
type Config struct {
	Projects []Project `toml:"projects"`

	// Refresh is how often the screen rereads the event files.
	Refresh Duration `toml:"refresh"`

	// Color is "auto", "always" or "never".
	Color string `toml:"color"`

	// Recent is how long finished work stays on the overview.
	Recent Duration `toml:"recent"`

	// Quiet is how long an open session may go without an event before it is shown as quiet.
	Quiet Duration `toml:"quiet"`

	// OtherWork shows Claude Code sessions that no event names, dimmed. The o key toggles it.
	OtherWork bool `toml:"other_work"`

	// RetentionDays is how long history is kept. History is not written yet.
	RetentionDays int `toml:"retention_days"`
}

// Duration reads values like "2s" or "48h" from the settings file.
type Duration struct {
	time.Duration
}

// UnmarshalText parses a Go duration string.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

// Default is the configuration used when there is no settings file, and the base a file's values
// are laid over.
func Default() Config {
	return Config{
		Refresh:       Duration{2 * time.Second},
		Color:         "auto",
		Recent:        Duration{48 * time.Hour},
		Quiet:         Duration{15 * time.Minute},
		OtherWork:     true,
		RetentionDays: 90,
	}
}

// Path is where the settings file lives: $XDG_CONFIG_HOME/waypoint-ops/config.toml, falling back to
// ~/.config.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "waypoint-ops", "config.toml"), nil
}

// Load reads the settings file at path. A missing file is not an error: the defaults are returned
// and found is false.
func Load(path string) (cfg Config, found bool, err error) {
	cfg = Default()
	_, err = toml.DecodeFile(path, &cfg)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), false, nil
	}
	if err != nil {
		return Default(), true, fmt.Errorf("%s: %w", path, err)
	}
	if err := cfg.check(); err != nil {
		return Default(), true, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, true, nil
}

func (c Config) check() error {
	switch c.Color {
	case "auto", "always", "never":
	default:
		return fmt.Errorf("color must be auto, always or never, not %q", c.Color)
	}
	if c.Refresh.Duration < 100*time.Millisecond {
		return fmt.Errorf("refresh %s is too short", c.Refresh.Duration)
	}
	for i, p := range c.Projects {
		if p.Path == "" {
			return fmt.Errorf("project %d has no path", i+1)
		}
	}
	return nil
}
