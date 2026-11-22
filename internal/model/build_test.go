package model

import (
	"strings"
	"testing"
	"time"

	"github.com/w-p/waypoint-ops/internal/claude"
	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/transcript"
)

var t0 = time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

func at(minutes int) time.Time {
	return t0.Add(time.Duration(minutes) * time.Minute)
}

// ev builds a synthetic event; runner events when run is set, hook events otherwise.
func ev(minutes int, typ, run string, fill func(*events.Event)) events.Event {
	e := events.Event{V: 1, TS: at(minutes), Type: typ, Project: "demo", Run: run,
		Tool: events.Tool{Name: "claude-code"}, Writer: "hook"}
	if run != "" {
		e.Writer = "runner"
	}
	if fill != nil {
		fill(&e)
	}
	return e
}

func project(pids map[string]int, list ...events.Event) *events.Project {
	p := events.NewProject("demo", "/nowhere/demo")
	p.Files["f"] = &events.File{Path: "f", Events: list}
	if pids != nil {
		p.PIDs = pids
	}
	return p
}

func started(units ...string) func(*events.Event) {
	return func(e *events.Event) { e.Units = units }
}

func unit(id, session string) func(*events.Event) {
	return func(e *events.Event) {
		e.Unit = id
		e.Session = &events.Session{ID: session, Kind: events.Unit, Parent: e.Run}
	}
}

func ended(id, status string) func(*events.Event) {
	return func(e *events.Event) {
		e.Unit = id
		e.Status = status
		e.Tokens = &events.Tokens{Input: 10, Output: 5}
	}
}

func hook(id, kind, parent string) func(*events.Event) {
	return func(e *events.Event) {
		e.Session = &events.Session{ID: id, Kind: kind, Parent: parent}
	}
}

func build(p *events.Project, now int) Snapshot {
	return Build([]*events.Project{p}, Options{
		Now:    at(now),
		Recent: 24 * time.Hour,
		Quiet:  10 * time.Minute,
		Alive:  func(pid int) bool { return pid == 42 },
	})
}

func TestRuns(t *testing.T) {
	tests := []struct {
		name     string
		pids     map[string]int
		list     []events.Event
		now      int
		state    RunState
		category Category
		flag     string
		recent   bool
	}{
		{
			name: "finished done goes to recent",
			list: []events.Event{
				ev(0, events.RunStarted, "r", started("1")),
				ev(1, events.UnitStarted, "r", unit("1", "u1")),
				ev(2, events.UnitEnded, "r", ended("1", "DONE")),
				ev(3, events.RunFinished, "r", func(e *events.Event) { e.Outcome = "DONE" }),
			},
			now: 10, state: Finished, category: Quiet, recent: true,
		},
		{
			name: "stopped on a failed unit is flagged",
			list: []events.Event{
				ev(0, events.RunStarted, "r", started("1", "2")),
				ev(1, events.UnitEnded, "r", ended("1", "DONE")),
				ev(2, events.UnitEnded, "r", ended("2", "FAILED")),
				ev(3, events.RunFinished, "r", func(e *events.Event) { e.Outcome = "STOPPED" }),
			},
			now: 10, state: Finished, category: Flagged, flag: "STOPPED: unit 2 FAILED",
		},
		{
			name: "held is not flagged",
			list: []events.Event{
				ev(0, events.RunStarted, "r", started("1")),
				ev(3, events.RunFinished, "r", func(e *events.Event) { e.Outcome = "HELD" }),
			},
			now: 10, state: Finished, category: Quiet, recent: true,
		},
		{
			name: "no finish and no pid died",
			list: []events.Event{
				ev(0, events.RunStarted, "r", started("1", "2")),
				ev(1, events.UnitEnded, "r", ended("1", "DONE")),
				ev(2, events.UnitStarted, "r", unit("2", "u2")),
			},
			now: 10, state: Died, category: Flagged, flag: "runner died during unit 2",
		},
		{
			name: "waiting with a live runner",
			pids: map[string]int{"r": 42},
			list: []events.Event{
				ev(0, events.RunStarted, "r", started("1")),
				ev(0, events.RunWaiting, "r", func(e *events.Event) { e.Reason = "wait_for" }),
			},
			now: 10, state: Paused, category: Waiting,
		},
		{
			name: "resumed and running a unit",
			pids: map[string]int{"r": 42},
			list: []events.Event{
				ev(0, events.RunStarted, "r", started("1")),
				ev(0, events.RunWaiting, "r", nil),
				ev(1, events.RunResumed, "r", nil),
				ev(2, events.UnitStarted, "r", unit("1", "u1")),
			},
			now: 10, state: Running, category: Active,
		},
		{
			name: "events after now are ignored",
			pids: map[string]int{"r": 42},
			list: []events.Event{
				ev(0, events.RunStarted, "r", started("1")),
				ev(2, events.UnitStarted, "r", unit("1", "u1")),
				ev(20, events.RunFinished, "r", func(e *events.Event) { e.Outcome = "STOPPED" }),
			},
			now: 10, state: Running, category: Active,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := build(project(tt.pids, tt.list...), tt.now)
			list := snap.Items
			if tt.recent {
				list = snap.Recent
			}
			if len(list) != 1 || list[0].Run == nil {
				t.Fatalf("items %d recent %d; want the run in %s", len(snap.Items), len(snap.Recent),
					map[bool]string{true: "recent", false: "items"}[tt.recent])
			}
			item := list[0]
			if item.Run.State != tt.state || item.Category != tt.category || item.Flag != tt.flag {
				t.Errorf("state %d category %d flag %q; want %d %d %q",
					item.Run.State, item.Category, item.Flag, tt.state, tt.category, tt.flag)
			}
		})
	}
}

func TestDiedRunCutsOffItsUnit(t *testing.T) {
	p := project(nil,
		ev(0, events.RunStarted, "r", started("1")),
		ev(1, events.UnitStarted, "r", unit("1", "u1")),
	)
	snap := build(p, 10)
	if got := snap.Items[0].Run.Units[0].Status; got != CutOff {
		t.Errorf("unit status %q, want %q", got, CutOff)
	}
}

func TestJobs(t *testing.T) {
	code := 1
	p := project(map[string]int{"r": 42},
		ev(0, events.RunStarted, "r", started("1")),
		ev(1, events.JobStarted, "r", func(e *events.Event) { e.Unit = "1"; e.PID = 42 }),
		ev(2, events.JobStarted, "r", func(e *events.Event) { e.Unit = "1"; e.PID = 7 }),
		ev(3, events.JobEnded, "r", func(e *events.Event) { e.Unit = "1"; e.ExitCode = &code; e.FrozenS = 60 }),
	)
	snap := build(p, 10)
	r := snap.Items[0].Run
	if len(r.Jobs) != 2 {
		t.Fatalf("got %d jobs", len(r.Jobs))
	}
	if r.Jobs[0].Ended || !r.Jobs[0].Alive {
		t.Errorf("first job should be running: %+v", r.Jobs[0])
	}
	if !r.Jobs[1].Ended || r.Jobs[1].Frozen != time.Minute {
		t.Errorf("second job should have ended with a minute frozen: %+v", r.Jobs[1])
	}
	if snap.Machine.JobsRunning != 1 {
		t.Errorf("jobs running %d, want 1", snap.Machine.JobsRunning)
	}
}

func TestUnitSessionJoinsItsRun(t *testing.T) {
	p := project(map[string]int{"r": 42},
		ev(0, events.RunStarted, "r", started("1")),
		ev(1, events.UnitStarted, "r", unit("1", "u1")),
		ev(2, events.SessionStarted, "", hook("u1", events.Unit, "r")),
		ev(3, events.SessionMemory, "", hook("u1", events.Unit, "r")),
		ev(4, events.SessionStarted, "", hook("a1", events.Subagent, "u1")),
	)
	snap := build(p, 10)
	if len(snap.Items) != 1 {
		t.Fatalf("got %d items, want only the run", len(snap.Items))
	}
	u := snap.Items[0].Run.Units[0]
	if u.Session == nil || u.Session.Memory != 1 {
		t.Fatalf("unit session not joined: %+v", u.Session)
	}
	if n := len(snap.Items[0].Run.Subagents()); n != 1 {
		t.Errorf("run subagents %d, want 1", n)
	}
}

func TestSessions(t *testing.T) {
	tests := []struct {
		name     string
		now      int
		items    int
		recent   int
		category Category
		open     int
		live     int
	}{
		{"recent event is active", 5, 1, 0, Active, 1, 1},
		{"quiet past the threshold", 30, 1, 0, Quiet, 1, 1},
		{"open but older than the window is dropped", 60 * 25, 0, 0, Quiet, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := project(nil,
				ev(0, events.SessionStarted, "", hook("s1", events.Interactive, "")),
				ev(1, events.SessionStarted, "", hook("a1", events.Subagent, "s1")),
				ev(2, events.SessionStarted, "", hook("a2", events.Subagent, "s1")),
				ev(3, events.SessionEnded, "", hook("a2", events.Subagent, "s1")),
				ev(4, events.SessionStarted, "", hook("p1", events.Unit, "s1")),
			)
			snap := build(p, tt.now)
			if len(snap.Items) != tt.items || len(snap.Recent) != tt.recent {
				t.Fatalf("items %d recent %d; want %d %d", len(snap.Items), len(snap.Recent), tt.items, tt.recent)
			}
			if snap.Machine.SessionsOpen != tt.open || snap.Machine.SubagentsLive != tt.live {
				t.Errorf("open %d live %d; want %d %d", snap.Machine.SessionsOpen, snap.Machine.SubagentsLive, tt.open, tt.live)
			}
			if tt.items == 0 {
				return
			}
			s := snap.Items[0].Session
			if snap.Items[0].Category != tt.category {
				t.Errorf("category %d, want %d", snap.Items[0].Category, tt.category)
			}
			if len(s.Subagents) != 2 || len(s.Units) != 1 {
				t.Errorf("subagents %d units %d; want 2 1", len(s.Subagents), len(s.Units))
			}
		})
	}
}

func TestOrder(t *testing.T) {
	p := project(map[string]int{"wait": 42},
		ev(0, events.SessionStarted, "", hook("s1", events.Interactive, "")),
		ev(1, events.RunStarted, "wait", started("1")),
		ev(1, events.RunWaiting, "wait", nil),
		ev(2, events.RunStarted, "dead", started("1")),
	)
	snap := build(p, 5)
	var got []Category
	for _, item := range snap.Items {
		got = append(got, item.Category)
	}
	want := []Category{Flagged, Active, Waiting}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestSessionActivity(t *testing.T) {
	withTranscript := func(e *events.Event) {
		e.Session = &events.Session{ID: "s1", Kind: events.Interactive}
		e.Transcript = &events.Transcript{Path: "/t/s1.jsonl", Format: transcript.FormatClaudeCode}
	}
	tests := []struct {
		name     string
		activity *transcript.Activity
		now      int
		ended    bool
		category Category
		today    int64
		spend    int
	}{
		{"no reader: events only, quiet", nil, 30, false, Quiet, 0, 0},
		{"working recently is active", &transcript.Activity{Last: at(28), Turn: transcript.Working}, 30, false, Active, 0, 0},
		{"finished turn is waiting", &transcript.Activity{Last: at(5), Turn: transcript.Idle}, 30, false, Waiting, 0, 0},
		{"working but silent is quiet", &transcript.Activity{Last: at(5), Turn: transcript.Working}, 30, false, Quiet, 0, 0},
		{"tokens land when used, not at the end",
			&transcript.Activity{Last: at(20), Turn: transcript.Idle,
				Spend: []transcript.Spend{{At: at(2), Tokens: 100}, {At: at(20), Tokens: 50}}},
			30, true, Quiet, 150, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := []events.Event{ev(0, events.SessionStarted, "", withTranscript)}
			if tt.ended {
				list = append(list, ev(25, events.SessionEnded, "", func(e *events.Event) {
					withTranscript(e)
					e.Tokens = &events.Tokens{Input: 999}
				}))
			}
			opt := Options{Now: at(tt.now), Recent: 24 * time.Hour, Quiet: 10 * time.Minute}
			if tt.activity != nil {
				opt.Activity = map[string]*transcript.Activity{"/t/s1.jsonl": tt.activity}
			}
			snap := Build([]*events.Project{project(nil, list...)}, opt)
			all := append(snap.Items, snap.Recent...)
			if len(all) != 1 {
				t.Fatalf("got %d rows, want 1", len(all))
			}
			if all[0].Category != tt.category {
				t.Errorf("category %d, want %d", all[0].Category, tt.category)
			}
			if got := snap.Machine.TokensToday["claude-code"]; tt.ended && got != tt.today {
				t.Errorf("tokens today %d, want %d (the end total must not count twice)", got, tt.today)
			}
			if tt.ended && len(snap.Machine.Spend) != tt.spend {
				t.Errorf("spend entries %d, want %d", len(snap.Machine.Spend), tt.spend)
			}
		})
	}
}

func TestRunningSessions(t *testing.T) {
	interactive := func(path string) func(*events.Event) {
		return func(e *events.Event) {
			e.Session = &events.Session{ID: "s1", Kind: events.Interactive}
			e.Transcript = &events.Transcript{Path: path, Format: transcript.FormatClaudeCode}
		}
	}
	local := "~/.claude/projects/demo/s1.jsonl"
	tests := []struct {
		name      string
		path      string
		running   claude.List
		now       int
		category  Category
		attention bool
		flag      string
		recent    bool
	}{
		{"blocked on a permission prompt is flagged for attention", local,
			claude.List{OK: true, Sessions: map[string]claude.Session{"s1": {SessionID: "s1", Status: claude.Waiting, WaitingFor: "permission prompt"}}},
			5, Flagged, true, "waiting on permission prompt", false},
		{"busy is active even when events are old", local,
			claude.List{OK: true, Sessions: map[string]claude.Session{"s1": {SessionID: "s1", Status: claude.Busy}}},
			60, Active, false, "", false},
		{"idle is waiting", local,
			claude.List{OK: true, Sessions: map[string]claude.Session{"s1": {SessionID: "s1", Status: claude.Idle}}},
			5, Waiting, false, "", false},
		{"not running and local is gone, with finished work", local,
			claude.List{OK: true, Sessions: map[string]claude.Session{}},
			5, Quiet, false, "", true},
		{"not running but from elsewhere stays open", "/other/machine/s1.jsonl",
			claude.List{OK: true, Sessions: map[string]claude.Session{}},
			5, Active, false, "", false},
		{"no listing read says nothing", local, claude.List{}, 5, Active, false, "", false},
		{"running keeps an old session in view", local,
			claude.List{OK: true, Sessions: map[string]claude.Session{"s1": {SessionID: "s1", Status: claude.Idle}}},
			60 * 30, Waiting, false, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := project(nil, ev(0, events.SessionStarted, "", interactive(tt.path)))
			snap := Build([]*events.Project{p}, Options{
				Now: at(tt.now), Recent: 24 * time.Hour, Quiet: 10 * time.Minute,
				Running: tt.running, LocalTranscripts: []string{"~/.claude/"},
			})
			rows := snap.Items
			if tt.recent {
				rows = snap.Recent
			}
			if len(rows) != 1 {
				t.Fatalf("items %d recent %d", len(snap.Items), len(snap.Recent))
			}
			item := rows[0]
			if item.Category != tt.category || item.Attention != tt.attention || item.Flag != tt.flag {
				t.Errorf("category %d attention %v flag %q; want %d %v %q", item.Category, item.Attention, item.Flag, tt.category, tt.attention, tt.flag)
			}
			wantOpen := 1
			if tt.recent {
				wantOpen = 0
			}
			if snap.Machine.SessionsOpen != wantOpen {
				t.Errorf("sessions open %d, want %d", snap.Machine.SessionsOpen, wantOpen)
			}
		})
	}
}

func TestUnitSessionWithoutIDJoinsRunningUnit(t *testing.T) {
	p := project(map[string]int{"r": 42},
		ev(0, events.RunStarted, "r", started("1", "2")),
		ev(1, events.UnitStarted, "r", unit("1", "")),
		ev(2, events.SessionStarted, "", hook("u1", events.Unit, "r")),
	)
	snap := build(p, 10)
	if len(snap.Items) != 1 {
		t.Fatalf("got %d items, want only the run (the unit session is not a row of its own)", len(snap.Items))
	}
	u := snap.Items[0].Run.Units[0]
	if u.Session == nil || u.SessionID != "u1" {
		t.Errorf("unit 1 session %q, want u1", u.SessionID)
	}
}

func TestOtherWork(t *testing.T) {
	listed := func(id, cwd, status string, startedAt time.Time) claude.Session {
		return claude.Session{SessionID: id, Cwd: cwd, Status: status, StartedAt: startedAt.UnixMilli(), Since: startedAt}
	}
	p := project(map[string]int{"r": 42},
		ev(0, events.SessionStarted, "", hook("s1", events.Interactive, "")),
		ev(1, events.RunStarted, "r", started("1")),
		ev(2, events.UnitStarted, "r", unit("1", "")),
	)
	running := claude.List{OK: true, Sessions: map[string]claude.Session{
		"s1":   listed("s1", "/nowhere/demo", claude.Busy, at(0)),
		"unit": listed("unit", "/nowhere/demo/scratch", claude.Busy, at(2)),
		"mine": listed("mine", "/nowhere/demo", claude.Idle, at(3)),
		"away": listed("away", "/elsewhere/side-project", claude.Busy, at(4)),
		"ask":  {SessionID: "ask", Cwd: "/elsewhere/tool", Status: claude.Waiting, WaitingFor: "permission prompt", Since: at(5)},
	}}
	opt := Options{Now: at(10), Recent: 24 * time.Hour, Quiet: 10 * time.Minute,
		Alive: func(pid int) bool { return pid == 42 }, Running: running}

	if snap := Build([]*events.Project{p}, opt); snap.Machine.OtherOpen != 0 || len(snap.Items) != 2 {
		t.Fatalf("hidden: other %d, items %d; want 0 and 2", snap.Machine.OtherOpen, len(snap.Items))
	}

	opt.OtherWork = true
	snap := Build([]*events.Project{p}, opt)
	var got []string
	for _, item := range snap.Items {
		if item.Session != nil && item.Session.Other {
			got = append(got, item.Project+"/"+item.Session.ID)
		}
	}
	// The blocked session sorts with flagged work; the rest follow the Waypoint rows, active
	// before waiting. The unit's session is claimed by its running unit.
	want := []string{"tool/ask", "side-project/away", "demo/mine"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("other work %v, want %v", got, want)
	}
	if snap.Items[0].Session == nil || !snap.Items[0].Attention {
		t.Errorf("blocked other work is not first and flagged for attention")
	}
	last := snap.Items[len(snap.Items)-1]
	if last.Session == nil || last.Session.ID != "mine" {
		t.Errorf("other work does not follow Waypoint work")
	}
	if snap.Machine.OtherOpen != 3 || snap.Machine.SessionsOpen != 1 {
		t.Errorf("other %d sessions %d; want 3 and 1", snap.Machine.OtherOpen, snap.Machine.SessionsOpen)
	}
}
