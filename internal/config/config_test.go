package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		text     string // empty: no file
		found    bool
		fails    bool
		refresh  time.Duration
		projects int
	}{
		{"no file uses defaults", "", false, false, 2 * time.Second, 0},
		{"values override defaults", "refresh = \"5s\"\n[[projects]]\npath = \"/a\"\n", true, false, 5 * time.Second, 1},
		{"bad color", "color = \"sometimes\"\n", true, true, 2 * time.Second, 0},
		{"project without path", "[[projects]]\nname = \"x\"\n", true, true, 2 * time.Second, 0},
		{"bad duration", "refresh = \"soon\"\n", true, true, 2 * time.Second, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if tt.text != "" {
				if err := os.WriteFile(path, []byte(tt.text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cfg, found, err := Load(path)
			if (err != nil) != tt.fails {
				t.Fatalf("err %v, want failure %v", err, tt.fails)
			}
			if found != tt.found || cfg.Refresh.Duration != tt.refresh || len(cfg.Projects) != tt.projects {
				t.Errorf("found %v refresh %s projects %d; want %v %s %d",
					found, cfg.Refresh.Duration, len(cfg.Projects), tt.found, tt.refresh, tt.projects)
			}
		})
	}
}
