package model

import (
	"time"

	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/procs"
	"github.com/w-p/waypoint-ops/internal/transcript"
)

// Condition is a session's condition: the latest session.condition record its project wrote, or,
// for a session without one, a fatigue reading worked out from its transcript
// (`design/session-condition.md`).
type Condition struct {
	events.Condition

	// At is when the record was written, or when the transcript was last read.
	At time.Time

	// Derived marks a reading worked out by this app from the transcript. Only Fatigue and
	// CompactionPossible are set then.
	Derived bool

	// LastCompaction is when the transcript shows the latest compaction, for derived readings.
	LastCompaction time.Time
}

// fatigueRule is one row of the fatigue table: the level, reached when every measure named is at
// least its value.
type fatigueRule struct {
	level          string
	compactions    int
	workSummarized int
	hours          float64
}

// fatigueRules are the pilot's thresholds on 2026-11-23 (`FATIGUE` in its events hook), first
// match wins. The pilot recalibrates its own from slips; these follow when the developer asks.
var fatigueRules = []fatigueRule{
	{level: "exhausted", compactions: 8},
	{level: "tired", compactions: 4},
	{level: "tired", workSummarized: 100},
	{level: "tiring", compactions: 1},
	{level: "working", hours: 2},
}

// compactionPossibleK is the context size, in thousands of tokens, past which the pilot says a
// compaction can come: every compaction it saw fired past 200k.
const compactionPossibleK = 200

// derive works out a fatigue reading from what a transcript shows. Hours count from started: the
// session's process start where known, so a resume starts the clock again as the hook's does.
func derive(a *transcript.Activity, started, now time.Time) *Condition {
	compactions, summarized := a.Compactions, a.WorkSummarized
	contextK := int((a.Context + 500) / 1000)
	c := &Condition{
		Condition: events.Condition{
			Compactions:        &compactions,
			WorkSummarized:     &summarized,
			ContextK:           &contextK,
			CompactionPossible: contextK >= compactionPossibleK,
		},
		At:             a.Last,
		Derived:        true,
		LastCompaction: a.LastCompaction,
	}
	hours := -1.0
	if !started.IsZero() {
		hours = now.Sub(started).Hours()
		rounded := float64(int(hours*10+0.5)) / 10
		c.Hours = &rounded
	}
	c.Fatigue = events.FatigueLevels[0]
	for _, r := range fatigueRules {
		if compactions >= r.compactions && summarized >= r.workSummarized && (r.hours == 0 || hours >= r.hours) {
			c.Fatigue = r.level
			break
		}
	}
	return c
}

// levelRank is a reading's place in its list of levels, or -1 when it is not one.
func levelRank(levels []string, level string) int {
	for i, l := range levels {
		if l == level {
			return i
		}
	}
	return -1
}

// FatigueRank is the fatigue reading's place from fresh (0) to exhausted (4), or -1 when unknown.
func (c *Condition) FatigueRank() int {
	if c == nil {
		return -1
	}
	return levelRank(events.FatigueLevels, c.Fatigue)
}

// condition settles a session's condition: its own record if it has one, else one derived from
// its transcript.
func condition(s *Session, load map[int]procs.Usage, now time.Time) {
	if s == nil || s.Condition != nil || s.Activity == nil || !s.Open() {
		return
	}
	started := s.Started
	if u, ok := s.usage(load); ok && !u.Started.IsZero() {
		started = u.Started
	}
	s.Condition = derive(s.Activity, started, now)
}

// usage is the sampled load of the session's process, unless that pid now names a process that
// started after the session named it: the pid was reused, and the load is someone else's.
func (s *Session) usage(load map[int]procs.Usage) (procs.Usage, bool) {
	named := s.pidAt
	if p := s.Process; p != nil && p.PID > 0 {
		named = p.Started()
		if named.IsZero() {
			named = p.Since
		}
	}
	u, ok := load[s.ProcessID()]
	return u, ok && sameProcess(u, named)
}

// sameProcess reports whether a sampled process can be the one an event named at a time: it must
// not have started after it (a minute's grace for clocks).
func sameProcess(u procs.Usage, named time.Time) bool {
	return u.Started.IsZero() || named.IsZero() || !u.Started.After(named.Add(time.Minute))
}

// ProcessID is the session's process: from the tool's own list of running sessions, else the pid
// its session.started gave.
func (s *Session) ProcessID() int {
	if s.Process != nil && s.Process.PID > 0 {
		return s.Process.PID
	}
	return s.PID
}

// conditions settles the condition and load of everything open in the snapshot, and counts the
// open sessions by fatigue.
func conditions(snap *Snapshot, opt Options) {
	byPID := opt.Load.ByPID
	for _, item := range snap.Items {
		if r := item.Run; r != nil {
			if u, ok := byPID[r.PID]; ok && r.Alive && sameProcess(u, r.Started) {
				r.Load = &u
			}
			for _, unit := range r.Units {
				switch {
				case unit.Session != nil && unit.Session.Condition != nil:
					unit.Condition = unit.Session.Condition
				case unit.Status == Working && unit.Activity != nil:
					started := unit.Started
					if unit.Session != nil {
						if u, ok := unit.Session.usage(byPID); ok && !u.Started.IsZero() {
							started = u.Started
						}
					}
					unit.Condition = derive(unit.Activity, started, opt.Now)
				}
			}
			continue
		}
		s := item.Session
		if s == nil || !s.Open() || s.Gone {
			continue
		}
		condition(s, byPID, opt.Now)
		if u, ok := s.usage(byPID); ok {
			s.Load = &u
		}
		if s.Other && !opt.OtherWork {
			continue
		}
		switch s.Condition.FatigueRank() {
		case 2:
			snap.Machine.Tiring++
		case 3:
			snap.Machine.Tired++
		case 4:
			snap.Machine.Exhausted++
		}
	}
}
