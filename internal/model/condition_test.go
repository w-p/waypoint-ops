package model

import (
	"testing"
	"time"

	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/procs"
	"github.com/w-p/waypoint-ops/internal/transcript"
)

func TestDeriveFatigue(t *testing.T) {
	tests := []struct {
		name                 string
		compactions, summary int
		hours                float64
		want                 string
	}{
		{"new", 0, 0, 1, "fresh"},
		{"two hours in", 0, 0, 2.5, "working"},
		{"one compaction", 1, 10, 0.5, "tiring"},
		{"a hundred work items summarized away", 1, 100, 3, "tired"},
		{"four compactions", 4, 20, 3, "tired"},
		{"eight compactions", 8, 20, 9, "exhausted"},
	}
	now := at(600)
	for _, tt := range tests {
		a := &transcript.Activity{Compactions: tt.compactions, WorkSummarized: tt.summary, Context: 210_400}
		c := derive(a, now.Add(-time.Duration(tt.hours*float64(time.Hour))), now)
		if c.Fatigue != tt.want {
			t.Errorf("%s: %s, want %s", tt.name, c.Fatigue, tt.want)
		}
		if !c.Derived || !c.CompactionPossible || *c.ContextK != 210 || c.Drift != "" || c.Unsaved != "" {
			t.Errorf("%s: %+v", tt.name, c)
		}
	}
}

func TestSessionCondition(t *testing.T) {
	path := "/t/s1.jsonl"
	start := func(e *events.Event) {
		e.Session = &events.Session{ID: "s1", Kind: events.Interactive}
		e.Transcript = &events.Transcript{Path: path, Format: transcript.FormatClaudeCode}
		e.PID = 42
	}
	two := 2
	record := func(e *events.Event) {
		e.Session = &events.Session{ID: "s1", Kind: events.Interactive}
		e.Condition = events.Condition{Fatigue: "tiring", Drift: "stale", Unsaved: "lots", JustCompacted: true, Compactions: &two}
	}
	activity := map[string]*transcript.Activity{path: {Last: at(30), Turn: transcript.Working, Compactions: 5}}
	load := procs.Reading{ByPID: map[int]procs.Usage{42: {CPU: 12, HasCPU: true, RSS: 1 << 20, Procs: 3, Started: at(20)}}}
	early := procs.Reading{ByPID: map[int]procs.Usage{42: {CPU: 12, HasCPU: true, RSS: 1 << 20, Procs: 3, Started: at(-1)}}}

	t.Run("a record is shown as written", func(t *testing.T) {
		p := project(nil, ev(0, events.SessionStarted, "", start), ev(10, events.SessionCondition, "", record))
		snap := Build([]*events.Project{p}, Options{Now: at(30), Recent: time.Hour, Quiet: time.Hour,
			Activity: activity, Load: early, Alive: func(int) bool { return true }})
		s := snap.Items[0].Session
		c := s.Condition
		if c == nil || c.Derived || c.Fatigue != "tiring" || c.Drift != "stale" || !c.JustCompacted || !c.At.Equal(at(10)) {
			t.Fatalf("condition %+v", c)
		}
		if snap.Machine.Tiring != 1 || snap.Machine.Tired != 0 {
			t.Errorf("counts %+v", snap.Machine)
		}
		if s.Load == nil || s.Load.Procs != 3 {
			t.Errorf("load %+v", s.Load)
		}
	})
	t.Run("without a record, fatigue is worked out from the transcript", func(t *testing.T) {
		resumed := func(e *events.Event) { start(e); e.Source = "resume" }
		p := project(nil, ev(0, events.SessionStarted, "", start), ev(20, events.SessionStarted, "", resumed))
		snap := Build([]*events.Project{p}, Options{Now: at(30), Recent: time.Hour, Quiet: time.Hour,
			Activity: activity, Load: load, Alive: func(int) bool { return true }})
		c := snap.Items[0].Session.Condition
		if c == nil || !c.Derived || c.Fatigue != "tired" {
			t.Fatalf("condition %+v", c)
		}
		if c.Hours == nil || *c.Hours != 0.2 {
			t.Errorf("hours should count from the resumed process's start (10 min ago): %v", c.Hours)
		}
		if snap.Machine.Tired != 1 {
			t.Errorf("counts %+v", snap.Machine)
		}
	})
	t.Run("a reused pid's load is not the session's", func(t *testing.T) {
		p := project(nil, ev(0, events.SessionStarted, "", start))
		reused := procs.Reading{ByPID: map[int]procs.Usage{42: {RSS: 1, Procs: 1, Started: at(25)}}}
		snap := Build([]*events.Project{p}, Options{Now: at(30), Recent: time.Hour, Quiet: time.Hour,
			Activity: activity, Load: reused, Alive: func(int) bool { return true }})
		s := snap.Items[0].Session
		if s.Load != nil {
			t.Errorf("load %+v", s.Load)
		}
		if s.Condition.Hours == nil || *s.Condition.Hours != 0.5 {
			t.Errorf("hours should count from the session's start: %v", s.Condition.Hours)
		}
	})
	t.Run("no transcript, no reading", func(t *testing.T) {
		p := project(nil, ev(0, events.SessionStarted, "", start))
		snap := Build([]*events.Project{p}, Options{Now: at(30), Recent: time.Hour, Quiet: time.Hour,
			Alive: func(int) bool { return true }})
		if c := snap.Items[0].Session.Condition; c != nil {
			t.Errorf("condition %+v", c)
		}
	})
}

func TestUnitCondition(t *testing.T) {
	path := "/t/u1.jsonl"
	list := []events.Event{
		ev(0, events.RunStarted, "r1", started("1", "2")),
		ev(1, events.UnitStarted, "r1", func(e *events.Event) {
			unit("1", "u1")(e)
			e.Transcript = &events.Transcript{Path: path, Format: transcript.FormatClaudeCode}
		}),
	}
	snap := Build([]*events.Project{project(map[string]int{"r1": 42}, list...)}, Options{
		Now: at(5), Recent: time.Hour, Quiet: time.Hour,
		Alive:    func(pid int) bool { return pid == 42 },
		Activity: map[string]*transcript.Activity{path: {Last: at(4), Turn: transcript.Working, Compactions: 1}},
		Load:     procs.Reading{ByPID: map[int]procs.Usage{42: {RSS: 1 << 30, Procs: 4}}},
	})
	r := snap.Items[0].Run
	if u := r.Running(); u == nil || u.Condition == nil || u.Condition.Fatigue != "tiring" || !u.Condition.Derived {
		t.Fatalf("running unit's condition: %+v", u)
	}
	if r.Load == nil || r.Load.Procs != 4 {
		t.Errorf("runner load %+v", r.Load)
	}
}
