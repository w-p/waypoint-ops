package events

import (
	"os"
	"path/filepath"
	"testing"
)

const started = `{"v":1,"ts":"2026-01-02T10:00:00-05:00","event":"session.started","project":"demo","tool":{"name":"claude-code","version":"1.0"},"writer":"hook","session":{"id":"s1","kind":"interactive","parent":null}}`

func write(t *testing.T, path, text string) {
	t.Helper()
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if _, err := fh.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func TestFileRead(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		events int
		bad    int
		newer  int
	}{
		{"one line", started + "\n", 1, 0, 0},
		{"partial line waits", started, 0, 0, 0},
		{"bad line skipped", "not json\n" + started + "\n", 1, 1, 0},
		{"missing type is bad", `{"v":1,"ts":"2026-01-02T10:00:00Z"}` + "\n", 0, 1, 0},
		{"newer version stops", `{"v":2,"ts":"2026-01-02T10:00:00Z","event":"x"}` + "\n" + started + "\n", 0, 0, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "s.jsonl")
			write(t, path, tt.text)
			f := &File{Path: path}
			if err := f.Read(); err != nil {
				t.Fatal(err)
			}
			if len(f.Events) != tt.events || f.Bad != tt.bad || f.Newer != tt.newer {
				t.Errorf("events %d bad %d newer %d; want %d %d %d",
					len(f.Events), f.Bad, f.Newer, tt.events, tt.bad, tt.newer)
			}
		})
	}
}

func TestFileReadAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	f := &File{Path: path}

	write(t, path, started)
	if err := f.Read(); err != nil {
		t.Fatal(err)
	}
	if len(f.Events) != 0 {
		t.Fatalf("partial line read early")
	}

	write(t, path, "\n"+started+"\n")
	if err := f.Read(); err != nil {
		t.Fatal(err)
	}
	if len(f.Events) != 2 {
		t.Fatalf("got %d events, want 2", len(f.Events))
	}
	if f.Events[0].Session == nil || f.Events[0].Session.Parent != "" {
		t.Errorf("null parent should read as empty: %+v", f.Events[0].Session)
	}
}
