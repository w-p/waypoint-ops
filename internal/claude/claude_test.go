package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sample = `[
  {"pid": 101, "cwd": "/w/a", "kind": "interactive", "startedAt": 1795132201155, "sessionId": "s-idle", "name": "a-1", "status": "idle"},
  {"pid": 102, "cwd": "/w/b", "kind": "interactive", "startedAt": 1795132201155, "sessionId": "s-busy", "name": "b-1", "status": "busy"},
  {"pid": 103, "cwd": "/w/c", "kind": "interactive", "startedAt": 1795132201155, "sessionId": "s-wait", "name": "c-1", "status": "waiting", "waitingFor": "permission prompt", "extra": true},
  {"pid": 104, "cwd": "/w/d", "kind": "background", "id": "j1", "state": "working"}
]`

func TestParse(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	list, err := Parse([]byte(sample), now)
	if err != nil {
		t.Fatal(err)
	}
	if !list.OK || len(list.Sessions) != 3 {
		t.Fatalf("ok %v, %d sessions; want true, 3 (entries without a session id are skipped)", list.OK, len(list.Sessions))
	}
	tests := []struct {
		id      string
		status  string
		blocked bool
		waiting string
	}{
		{"s-idle", Idle, false, ""},
		{"s-busy", Busy, false, ""},
		{"s-wait", Waiting, true, "permission prompt"},
	}
	for _, tt := range tests {
		s := list.Sessions[tt.id]
		if s.Status != tt.status || s.Blocked() != tt.blocked || s.WaitingFor != tt.waiting {
			t.Errorf("%s: status %q blocked %v waiting %q", tt.id, s.Status, s.Blocked(), s.WaitingFor)
		}
	}
	if list.Sessions["s-idle"].Started().IsZero() {
		t.Errorf("start time not read")
	}
	if _, err := Parse([]byte("not json"), now); err == nil {
		t.Errorf("bad listing parsed without error")
	}
}

func TestTrack(t *testing.T) {
	t1 := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(5 * time.Second)
	first := Track(List{}, List{OK: true, At: t1, Sessions: map[string]Session{
		"a": {SessionID: "a", Status: Idle},
		"b": {SessionID: "b", Status: Busy},
	}})
	second := Track(first, List{OK: true, At: t2, Sessions: map[string]Session{
		"a": {SessionID: "a", Status: Idle},
		"b": {SessionID: "b", Status: Waiting, WaitingFor: "permission prompt"},
	}})
	if !second.Sessions["a"].Since.Equal(t1) {
		t.Errorf("unchanged status lost its start: %v", second.Sessions["a"].Since)
	}
	if !second.Sessions["b"].Since.Equal(t2) {
		t.Errorf("changed status kept the old start: %v", second.Sessions["b"].Since)
	}
}

func TestLocate(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "projects", "-w-a")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(folder, "s-idle.jsonl")
	if err := os.WriteFile(want, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	list, _ := Parse([]byte(sample), time.Now())
	list = Locate(list, dir)
	if got := list.Sessions["s-idle"].Transcript; got != want {
		t.Errorf("transcript %q, want %q", got, want)
	}
	if got := list.Sessions["s-busy"].Transcript; got != "" {
		t.Errorf("missing transcript found at %q", got)
	}
	next := Track(list, parsed(t, time.Now()))
	if got := next.Sessions["s-idle"].Transcript; got != want {
		t.Errorf("transcript not carried forward: %q", got)
	}
}

func parsed(t *testing.T, now time.Time) List {
	t.Helper()
	list, err := Parse([]byte(sample), now)
	if err != nil {
		t.Fatal(err)
	}
	return list
}
