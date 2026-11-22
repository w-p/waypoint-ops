package model

import (
	"errors"
	"fmt"
	"sort"
	"syscall"
	"time"

	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/transcript"
)

// Options control how a snapshot is built.
type Options struct {
	// Now is the moment the snapshot describes. Events after it are ignored, which lets the
	// static test data be viewed as it stood at any time.
	Now time.Time

	// Recent is how long finished work stays in view.
	Recent time.Duration

	// Quiet is how long an open session may go without an event before it is shown as quiet.
	Quiet time.Duration

	// Alive reports whether a process is running. Nil uses the operating system.
	Alive func(pid int) bool

	// Activity is what transcript readers found, by transcript path as the events give it.
	Activity map[string]*transcript.Activity
}

// ProcessAlive reports whether pid names a running process. A process owned by another user
// still counts as alive.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Build makes a snapshot from every project's events.
func Build(projects []*events.Project, opt Options) Snapshot {
	if opt.Alive == nil {
		opt.Alive = ProcessAlive
	}
	snap := Snapshot{
		Now:     opt.Now,
		Machine: Machine{TokensToday: map[string]int64{}},
	}
	for _, p := range projects {
		snap.Projects = append(snap.Projects, ProjectInfo{
			Name:     p.Name,
			Root:     p.Root,
			Problems: p.Problems(),
		})
		b := newBuilder(p.Name)
		for _, e := range p.Events() {
			if e.TS.After(opt.Now) {
				continue
			}
			b.add(e)
		}
		b.finish(p.PIDs, opt)
		b.place(&snap, opt)
	}
	sort.SliceStable(snap.Items, func(i, j int) bool {
		a, b := snap.Items[i], snap.Items[j]
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		return a.Last.After(b.Last)
	})
	sort.SliceStable(snap.Recent, func(i, j int) bool {
		return snap.Recent[i].Last.After(snap.Recent[j].Last)
	})
	sort.SliceStable(snap.Machine.Spend, func(i, j int) bool {
		return snap.Machine.Spend[i].At.Before(snap.Machine.Spend[j].At)
	})
	return snap
}

// builder gathers one project's events into runs and sessions.
type builder struct {
	project  string
	runs     map[string]*Run
	order    []string
	sessions map[string]*Session
	ids      []string
}

func newBuilder(project string) *builder {
	return &builder{
		project:  project,
		runs:     map[string]*Run{},
		sessions: map[string]*Session{},
	}
}

func (b *builder) run(name string) *Run {
	r, ok := b.runs[name]
	if !ok {
		r = &Run{Project: b.project, Name: name}
		b.runs[name] = r
		b.order = append(b.order, name)
	}
	return r
}

func (b *builder) session(ref *events.Session) *Session {
	s, ok := b.sessions[ref.ID]
	if !ok {
		s = &Session{Project: b.project, ID: ref.ID}
		b.sessions[ref.ID] = s
		b.ids = append(b.ids, ref.ID)
	}
	s.Kind = ref.Kind
	s.Parent = ref.Parent
	return s
}

func (r *Run) unit(id string) *Unit {
	for _, u := range r.Units {
		if u.ID == id {
			return u
		}
	}
	u := &Unit{ID: id, Status: Pending}
	r.Units = append(r.Units, u)
	return u
}

func (b *builder) add(e events.Event) {
	if e.Run != "" && e.Writer == "runner" {
		b.addRun(e)
		return
	}
	if e.Session != nil {
		b.addSession(e)
	}
}

func (b *builder) addRun(e events.Event) {
	r := b.run(e.Run)
	r.Last = e.TS
	if r.Tool.Name == "" {
		r.Tool = e.Tool
	}
	switch e.Type {
	case events.RunStarted:
		r.Started = e.TS
		r.Runbook = e.Runbook
		r.Cap = e.Cap
		r.MayPush = e.MayPush
		for _, id := range e.Units {
			u := r.unit(id)
			u.Name = e.Names[id]
		}
	case events.RunWaiting:
		r.State = Paused
		r.WaitReason = e.Reason
		r.WaitDetail = e.Detail
		r.WaitingSince = e.TS
	case events.RunResumed:
		r.State = Running
		r.WaitReason = ""
		r.WaitDetail = ""
		r.WaitingSince = time.Time{}
	case events.UnitStarted:
		u := r.unit(e.Unit)
		u.Status = Working
		u.Started = e.TS
		if e.Session != nil {
			u.SessionID = e.Session.ID
		}
	case events.UnitEnded:
		u := r.unit(e.Unit)
		u.Status = e.Status
		u.Reason = e.Reason
		u.Took = e.Duration()
		u.Turns = e.Turns
		u.Model = e.Model
		u.Tokens = e.Tokens
		u.PermissionDenials = e.PermissionDenials
		if e.Session != nil {
			u.SessionID = e.Session.ID
		}
		if e.Transcript != nil {
			u.Transcript = e.Transcript.Path
		}
	case events.JobStarted:
		r.Jobs = append(r.Jobs, &Job{Unit: e.Unit, PID: e.PID, Log: e.Log, Started: e.TS})
	case events.JobEnded:
		for i := len(r.Jobs) - 1; i >= 0; i-- {
			j := r.Jobs[i]
			if j.Unit == e.Unit && !j.Ended {
				j.Ended = true
				j.ExitCode = e.ExitCode
				j.Took = e.Duration()
				j.Frozen = e.Frozen()
				break
			}
		}
	case events.RunFinished:
		r.Finished = e.TS
		r.Outcome = e.Outcome
		r.Detail = e.Detail
	}
}

func (b *builder) addSession(e events.Event) {
	s := b.session(e.Session)
	s.Last = e.TS
	if s.Tool.Name == "" {
		s.Tool = e.Tool
	}
	switch e.Type {
	case events.SessionStarted:
		s.Started = e.TS
		s.Model = e.Model
		if e.Transcript != nil {
			s.Transcript = e.Transcript.Path
			s.TranscriptFormat = e.Transcript.Format
		}
	case events.SessionCompacted:
		s.Compactions++
	case events.SessionMemory:
		s.Memory++
		s.MemoryFile = e.File
	case events.SessionEnded:
		s.Ended = e.TS
		s.Duration = e.Duration()
		s.Tokens = e.Tokens
	}
	if s.Started.IsZero() {
		s.Started = e.TS
	}
}

// finish derives run states from pid files and joins sessions to their parents.
func (b *builder) finish(pids map[string]int, opt Options) {
	for _, r := range b.runs {
		r.PID = pids[r.Name]
		r.Alive = r.PID > 0 && opt.Alive(r.PID)
		for _, j := range r.Jobs {
			j.Alive = !j.Ended && opt.Alive(j.PID)
		}
		switch {
		case !r.Finished.IsZero():
			r.State = Finished
		case !r.Alive:
			r.State = Died
			for _, u := range r.Units {
				if u.Status == Working {
					u.Status = CutOff
				}
			}
		case r.State == Paused:
		case r.Running() != nil:
			r.State = Running
		default:
			r.State = Starting
		}
	}

	for _, id := range b.ids {
		s := b.sessions[id]
		switch s.Kind {
		case events.Subagent:
			if parent, ok := b.sessions[s.Parent]; ok {
				parent.Subagents = append(parent.Subagents, s)
			}
		case events.Unit:
			if r, ok := b.runs[s.Parent]; ok {
				for _, u := range r.Units {
					if u.SessionID == s.ID {
						u.Session = s
						s.joined = true
					}
				}
			} else if parent, ok := b.sessions[s.Parent]; ok {
				parent.Units = append(parent.Units, s)
				s.joined = true
			}
		}
	}
}

// place sorts the project's runs and top-level sessions into the snapshot.
func (b *builder) place(snap *Snapshot, opt Options) {
	cutoff := opt.Now.Add(-opt.Recent)
	today := func(t time.Time) bool {
		y1, m1, d1 := t.In(opt.Now.Location()).Date()
		y2, m2, d2 := opt.Now.Date()
		return y1 == y2 && m1 == m2 && d1 == d2
	}
	spend := func(at time.Time, tool string, n int64) {
		if at.After(opt.Now.Add(-24*time.Hour)) && !at.After(opt.Now) {
			snap.Machine.Spend = append(snap.Machine.Spend, Spend{at, tool, n})
		}
	}

	for _, name := range b.order {
		r := b.runs[name]
		for _, u := range r.Units {
			if u.Tokens != nil {
				end := u.Started.Add(u.Took)
				if today(end) {
					snap.Machine.TokensToday[r.Tool.Name] += u.Tokens.Total()
				}
				spend(end, r.Tool.Name, u.Tokens.Total())
			}
		}
		for _, j := range r.Jobs {
			if j.Alive {
				snap.Machine.JobsRunning++
			}
		}
		if r.State != Finished {
			snap.Machine.RunsUnfinished++
		}
		if r.Alive {
			snap.Machine.RunnersAlive++
		}

		item := runItem(r)
		live := r.State != Finished && r.State != Died
		if !live && r.Last.Before(cutoff) {
			continue
		}
		if live || item.Flag != "" {
			snap.Items = append(snap.Items, item)
		} else {
			snap.Recent = append(snap.Recent, item)
		}
	}

	for _, id := range b.ids {
		s := b.sessions[id]
		if a := opt.Activity[s.Transcript]; a != nil && s.Transcript != "" && !s.joined {
			// The transcript says when tokens were used, not just how many at the end.
			s.Activity = a
			if a.Last.After(s.Last) {
				s.Last = a.Last
			}
			for _, sp := range a.Spend {
				if today(sp.At) {
					snap.Machine.TokensToday[s.Tool.Name] += sp.Tokens
				}
				spend(sp.At, s.Tool.Name, sp.Tokens)
			}
		} else if s.Tokens != nil && !s.joined {
			if today(s.Ended) {
				snap.Machine.TokensToday[s.Tool.Name] += s.Tokens.Total()
			}
			spend(s.Ended, s.Tool.Name, s.Tokens.Total())
		}
		if s.Kind == events.Subagent {
			parent, ok := b.sessions[s.Parent]
			if ok && parent.Open() && s.Open() && !parent.Last.Before(cutoff) {
				snap.Machine.SubagentsLive++
			}
			continue
		}
		if s.joined {
			continue
		}
		if s.Last.Before(cutoff) {
			continue
		}
		item := &Item{Project: b.project, Session: s, Last: s.Last}
		if s.Open() {
			item.Category = sessionCategory(s, opt)
			if s.Kind == events.Interactive {
				snap.Machine.SessionsOpen++
			}
			snap.Items = append(snap.Items, item)
			continue
		}
		item.Category = Quiet
		snap.Recent = append(snap.Recent, item)
	}
}

// sessionCategory places an open session: with a transcript reader, working is active and a
// finished turn is waiting; without one, any event within the quiet threshold is active.
func sessionCategory(s *Session, opt Options) Category {
	silent := opt.Now.Sub(s.Last) > opt.Quiet
	if a := s.Activity; a != nil {
		switch {
		case a.Turn == transcript.Idle:
			return Waiting
		case a.Turn == transcript.Working && !silent:
			return Active
		}
		return Quiet
	}
	if silent {
		return Quiet
	}
	return Active
}

func runItem(r *Run) *Item {
	item := &Item{Project: r.Project, Run: r, Last: r.Last}
	switch r.State {
	case Running, Starting:
		item.Category = Active
	case Paused:
		item.Category = Waiting
	default:
		item.Category = Quiet
	}

	switch {
	case r.State == Died:
		item.Flag = "runner died"
		if u := cutOff(r); u != nil {
			item.Flag = fmt.Sprintf("runner died during unit %s", u.ID)
		}
	case r.State == Finished && problemOutcomes[r.Outcome]:
		item.Flag = r.Outcome
		if u := problem(r); u != nil {
			item.Flag = fmt.Sprintf("%s: unit %s %s", r.Outcome, u.ID, u.Status)
		}
	default:
		if u := problem(r); u != nil {
			item.Flag = fmt.Sprintf("unit %s %s", u.ID, u.Status)
		}
	}
	if item.Flag != "" {
		item.Category = Flagged
	}
	return item
}

func problem(r *Run) *Unit {
	for _, u := range r.Units {
		if problemStatuses[u.Status] {
			return u
		}
	}
	return nil
}

func cutOff(r *Run) *Unit {
	for _, u := range r.Units {
		if u.Status == CutOff {
			return u
		}
	}
	return nil
}
