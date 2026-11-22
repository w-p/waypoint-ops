// Package claude asks Claude Code which of its sessions are running on this machine, and what
// each is doing, through its own listing: `claude agents --json`. The listing is Claude Code's
// supported report of its sessions; this package only reads it.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Statuses a running session can report.
const (
	Busy    = "busy"
	Idle    = "idle"
	Waiting = "waiting"
)

// Session is one running Claude Code session as the listing reports it.
type Session struct {
	PID        int    `json:"pid"`
	SessionID  string `json:"sessionId"`
	Cwd        string `json:"cwd"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	WaitingFor string `json:"waitingFor"`
	State      string `json:"state"`
	StartedAt  int64  `json:"startedAt"`

	// Since is when this app first saw the session in its current status.
	Since time.Time `json:"-"`

	// Transcript is the session's transcript in this machine's Claude Code folder, when found.
	Transcript string `json:"-"`
}

// Started is when the session's process started.
func (s Session) Started() time.Time {
	if s.StartedAt == 0 {
		return time.Time{}
	}
	return time.UnixMilli(s.StartedAt)
}

// Blocked reports whether the session is stopped on something only a person can answer, such as
// a permission prompt.
func (s Session) Blocked() bool {
	return s.Status == Waiting
}

// List is one reading of the listing.
type List struct {
	// OK is false when the listing could not be read; then nothing can be said about which
	// sessions are running.
	OK bool
	At time.Time

	// Sessions are keyed by session id.
	Sessions map[string]Session
}

// ErrMissing means the claude command is not installed.
var ErrMissing = errors.New("claude command not found")

// Read runs the listing and parses it.
func Read(ctx context.Context, now time.Time) (List, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return List{}, ErrMissing
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "agents", "--json").Output()
	if err != nil {
		return List{}, err
	}
	return Parse(out, now)
}

// Dir is this machine's Claude Code folder: $CLAUDE_CONFIG_DIR, or ~/.claude.
func Dir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// Locate finds the transcripts of the listed sessions under dir, which holds one folder per
// working folder with a file per session id. A session already located keeps its path.
func Locate(list List, dir string) List {
	if dir == "" {
		return list
	}
	for id, s := range list.Sessions {
		if s.Transcript != "" {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(dir, "projects", "*", id+".jsonl"))
		if len(matches) > 0 {
			s.Transcript = matches[0]
			list.Sessions[id] = s
		}
	}
	return list
}

// Parse reads the listing's JSON. Unknown fields and statuses are kept as they are.
func Parse(data []byte, now time.Time) (List, error) {
	var sessions []Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return List{}, err
	}
	list := List{OK: true, At: now, Sessions: map[string]Session{}}
	for _, s := range sessions {
		if s.SessionID != "" {
			list.Sessions[s.SessionID] = s
		}
	}
	return list, nil
}

// Track carries each session's Since forward from the previous reading while its status and
// what it waits for stay the same, and its transcript path always.
func Track(prev, next List) List {
	for id, s := range next.Sessions {
		p, ok := prev.Sessions[id]
		if ok && s.Transcript == "" {
			s.Transcript = p.Transcript
		}
		if ok && p.Status == s.Status && p.WaitingFor == s.WaitingFor && !p.Since.IsZero() {
			s.Since = p.Since
		} else {
			s.Since = next.At
		}
		next.Sessions[id] = s
	}
	return next
}
