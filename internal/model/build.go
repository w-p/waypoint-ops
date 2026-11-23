package model

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/w-p/waypoint-ops/internal/claude"
	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/procs"
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

	// Running is Claude Code's own list of its running sessions on this machine. When it was
	// read, a Claude Code session missing from it has no process, provided its transcript lives
	// in one of LocalTranscripts (path prefixes for this machine's Claude Code folder); a session
	// from another machine or user would be missing for other reasons.
	Running          claude.List
	LocalTranscripts []string

	// OtherWork shows sessions in Running that no event names and that run outside every
	// watched project (`design/other-work.md`). Those inside a watched project always show.
	OtherWork bool

	// Load is the latest sample of the machine and of the processes of open sessions and runs.
	Load procs.Reading
}

// local reports whether a transcript path is in this machine's Claude Code folder.
func (o Options) local(path string) bool {
	for _, prefix := range o.LocalTranscripts {
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// ProcessAlive reports whether pid names a running process. A process owned by another user
// still counts as alive.
func ProcessAlive(pid int) bool {
	return procs.Alive(pid)
}

// Build makes a snapshot from every project's events.
func Build(projects []*events.Project, opt Options) Snapshot {
	if opt.Alive == nil {
		opt.Alive = ProcessAlive
	}
	snap := Snapshot{
		Now:     opt.Now,
		Machine: Machine{TokensToday: map[string]int64{}, Load: opt.Load},
		Listed:  opt.Running.OK,
	}
	var builders []*builder
	for _, p := range projects {
		snap.Projects = append(snap.Projects, ProjectInfo{
			Name:     p.Name,
			Root:     p.Root,
			Problems: p.Problems(),
		})
		b := newBuilder(p.Name)
		b.root = p.Root
		for _, e := range p.Events() {
			if e.TS.After(opt.Now) {
				continue
			}
			b.add(e)
		}
		b.finish(p.PIDs, opt)
		builders = append(builders, b)
	}
	claimed := claim(builders, opt.Running)
	for _, b := range builders {
		b.place(&snap, opt)
	}
	listed(&snap, builders, claimed, opt)
	conditions(&snap, opt)
	sort.SliceStable(snap.Items, func(i, j int) bool {
		a, b := snap.Items[i], snap.Items[j]
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
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

// rank orders the board: flagged work first, then Waypoint work by category, then other work by
// category. Other work waiting on a person is flagged and so sorts with the rest.
func rank(item *Item) int {
	r := int(item.Category)
	if item.Session != nil && item.Session.Other && item.Category != Flagged {
		r += int(Quiet) + 1
	}
	return r
}

// claim lists the session ids that events account for. A running unit whose events carry no
// session id takes the earliest listed session in its project's folder that started with it, so a
// unit is never mistaken for other work.
func claim(builders []*builder, running claude.List) map[string]bool {
	claimed := map[string]bool{}
	for _, b := range builders {
		for id := range b.sessions {
			claimed[id] = true
		}
		for _, r := range b.runs {
			for _, u := range r.Units {
				if u.SessionID != "" {
					claimed[u.SessionID] = true
				}
			}
		}
	}
	if !running.OK {
		return claimed
	}
	ids := make([]string, 0, len(running.Sessions))
	for id := range running.Sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, b := range builders {
		for _, name := range b.order {
			r := b.runs[name]
			u := r.Running()
			if u == nil || u.SessionID != "" || r.State != Running {
				continue
			}
			best := ""
			for _, id := range ids {
				p := running.Sessions[id]
				if claimed[id] || !within(p.Cwd, b.root) || p.Started().Before(u.Started.Add(-time.Minute)) {
					continue
				}
				if best == "" || p.Started().Before(running.Sessions[best].Started()) {
					best = id
				}
			}
			if best != "" {
				claimed[best] = true
			}
		}
	}
	return claimed
}

// within reports whether folder is root or inside it.
func within(folder, root string) bool {
	if folder == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(root, folder)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// listed adds the running sessions no event names. One inside a watched project's folder belongs
// to that project and shows like any session; one outside them all is other work, shown under its
// folder's name when OtherWork is set.
func listed(snap *Snapshot, builders []*builder, claimed map[string]bool, opt Options) {
	if !opt.Running.OK {
		return
	}
	ids := make([]string, 0, len(opt.Running.Sessions))
	for id := range opt.Running.Sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if claimed[id] {
			continue
		}
		p := opt.Running.Sessions[id]
		project, other := filepath.Base(p.Cwd), true
		for _, b := range builders {
			if within(p.Cwd, b.root) {
				project, other = b.project, false
				break
			}
		}
		if other && !opt.OtherWork {
			continue
		}
		s := &Session{
			Project: project,
			ID:      id,
			Kind:    events.Interactive,
			Tool:    events.Tool{Name: "claude-code"},
			Process: &p,
			Listed:  true,
			Other:   other,
			Folder:  p.Cwd,
			Started: p.Started(),
			Last:    p.Since,
		}
		if s.Started.IsZero() {
			s.Started = p.Since
		}
		if p.Transcript != "" {
			s.Transcript = p.Transcript
			s.TranscriptFormat = transcript.FormatClaudeCode
		}
		if a := opt.Activity[s.Transcript]; a != nil && s.Transcript != "" {
			s.Activity = a
			if a.Last.After(s.Last) {
				s.Last = a.Last
			}
			subagents(s, a, opt.Now)
			for _, sp := range a.Spend {
				snap.spend(sp.At, s.Tool.Name, sp.Tokens, opt.Now)
			}
		}
		snap.Machine.SubagentsLive += s.Live()
		item := &Item{Project: project, Session: s, Last: s.Last}
		if p.Blocked() {
			item.Category = Flagged
			item.Attention = true
			item.Flag = waitingFlag(p)
		} else {
			item.Category = sessionCategory(s, opt)
		}
		if other {
			snap.Machine.OtherOpen++
		} else {
			snap.Machine.SessionsOpen++
		}
		snap.Items = append(snap.Items, item)
	}
}

// subagentStale is how long an unfinished subagent may go without writing before it is taken to
// have stopped without handing back (its process killed, say). A subagent running one long tool
// writes nothing meanwhile, so this is generous.
const subagentStale = time.Hour

// subagents fills a session's subagents from its transcript reader, when no events named any.
// A subagent is open until it hands back, ends its turn, or goes stale.
func subagents(s *Session, a *transcript.Activity, now time.Time) {
	if len(s.Subagents) > 0 || len(a.Subagents) == 0 {
		return
	}
	s.readSubagents = true
	for _, sub := range a.Subagents {
		child := &Session{
			Project: s.Project,
			ID:      sub.ID,
			Kind:    events.Subagent,
			Parent:  s.ID,
			Tool:    s.Tool,
			Started: sub.Started,
			Last:    sub.Last,
		}
		if sub.Done || now.Sub(sub.Last) > subagentStale {
			child.Ended = sub.Last
		}
		s.Subagents = append(s.Subagents, child)
	}
}

// spend counts tokens used at a time towards today's total and the last 24 hours.
func (snap *Snapshot) spend(at time.Time, tool string, n int64, now time.Time) {
	y1, m1, d1 := at.In(now.Location()).Date()
	y2, m2, d2 := now.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		snap.Machine.TokensToday[tool] += n
	}
	if at.After(now.Add(-24*time.Hour)) && !at.After(now) {
		snap.Machine.Spend = append(snap.Machine.Spend, Spend{at, tool, n})
	}
}

func waitingFlag(p claude.Session) string {
	if p.WaitingFor == "" {
		return "waiting on the person"
	}
	return "waiting on " + p.WaitingFor
}

// builder gathers one project's events into runs and sessions.
type builder struct {
	project  string
	root     string
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
		if e.Transcript != nil {
			u.Transcript = e.Transcript.Path
			u.TranscriptFormat = e.Transcript.Format
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
			u.TranscriptFormat = e.Transcript.Format
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
	if e.Type == events.SessionModel {
		// The model is read from the transcript after the fact, so this event can land after a
		// session.waiting or just before session.ended. It says nothing about activity or waits.
		if e.Model != "" {
			s.Model = e.Model
		}
		if s.Tool.Name == "" {
			s.Tool = e.Tool
		}
		return
	}
	s.Last = e.TS
	if s.Tool.Name == "" {
		s.Tool = e.Tool
	}
	if e.Type != events.SessionWaiting {
		// Any later event from the session shows it moved on from a wait.
		s.Waiting = ""
		s.WaitingSince = time.Time{}
	}
	switch e.Type {
	case events.SessionStarted:
		if !s.started {
			s.Started = e.TS
			s.started = true
		} else if !s.Ended.IsZero() && e.TS.After(s.Ended) {
			// Resumed after it ended: the same session, open again. Its next end reports the
			// whole session.
			s.Ended = time.Time{}
			s.Duration = 0
			s.Tokens = nil
		}
		if e.Model != "" {
			s.Model = e.Model
		}
		if e.PID > 0 {
			s.PID = e.PID
			s.pidAt = e.TS
		}
		if e.Transcript != nil {
			s.Transcript = e.Transcript.Path
			s.TranscriptFormat = e.Transcript.Format
		}
	case events.SessionWaiting:
		s.Waiting = e.Reason
		s.WaitingSince = e.TS
	case events.SessionCompacted:
		s.Compactions++
	case events.SessionMemory:
		s.Memory++
		s.MemoryFile = e.File
	case events.SessionCondition:
		s.Condition = &Condition{Condition: e.Condition, At: e.TS}
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
				if !s.joined {
					// The runner's events did not name this session: it belongs to the unit
					// that was running when it started and has no session of its own yet.
					if u := unitAt(r, s.Started); u != nil {
						u.SessionID = s.ID
						u.Session = s
					}
					s.joined = true // never shown as a session of its own
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
			if path, _ := u.TranscriptAt(); path != "" {
				u.Activity = opt.Activity[path]
			}
			if a := u.Activity; a != nil {
				// The transcript says when the unit used its tokens, not just how many at the end.
				for _, sp := range a.Spend {
					snap.spend(sp.At, r.Tool.Name, sp.Tokens, opt.Now)
				}
				if u.Session != nil {
					subagents(u.Session, a, opt.Now)
					if u.Session.readSubagents && r.State != Finished && r.State != Died {
						snap.Machine.SubagentsLive += u.Session.Live()
					}
				}
			} else if u.Tokens != nil {
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
			// A finished run's runner may linger a moment, or its pid be reused; only runners of
			// unfinished runs count.
			if r.Alive {
				snap.Machine.RunnersAlive++
			}
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
			subagents(s, a, opt.Now)
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
		if s.Open() {
			// Claude Code's own list decides when it can (this machine's sessions); otherwise the
			// pid from session.started does, as the contract says.
			decided := false
			if s.Tool.Name == "claude-code" && opt.Running.OK {
				if p, ok := opt.Running.Sessions[s.ID]; ok {
					s.Process = &p
					decided = true
				} else if s.Kind == events.Interactive && opt.local(s.Transcript) {
					s.Gone = true
					decided = true
				}
			}
			if !decided && s.Kind == events.Interactive && s.PID > 0 && !opt.Alive(s.PID) {
				s.Gone = true
			}
		}
		if s.Last.Before(cutoff) && s.Process == nil {
			continue // a running process keeps a session in view however long it has been idle
		}
		item := &Item{Project: b.project, Session: s, Last: s.Last}
		if s.Gone {
			// No end was recorded, but no process is left: it is finished work.
			item.Category = Quiet
			snap.Recent = append(snap.Recent, item)
			continue
		}
		switch p := s.Process; {
		case p != nil && p.Blocked():
			item.Category = Flagged
			item.Attention = true
			item.Flag = waitingFlag(*p)
		case p == nil && s.Waiting == events.WaitPermission:
			item.Category = Flagged
			item.Attention = true
			item.Flag = "waiting on permission prompt"
		}
		if s.Open() {
			if !item.Attention {
				item.Category = sessionCategory(s, opt)
			}
			if s.Kind == events.Interactive {
				snap.Machine.SessionsOpen++
			}
			if s.readSubagents {
				snap.Machine.SubagentsLive += s.Live()
			}
			snap.Items = append(snap.Items, item)
			continue
		}
		item.Category = Quiet
		snap.Recent = append(snap.Recent, item)
	}
}

// sessionCategory places an open session. Claude Code's own status decides first (busy is
// active, idle is waiting); then a transcript reader's turn (working is active, a finished turn
// is waiting); without either, any event within the quiet threshold is active.
func sessionCategory(s *Session, opt Options) Category {
	if p := s.Process; p != nil {
		switch p.Status {
		case claude.Busy:
			return Active
		case claude.Idle:
			return Waiting
		}
	}
	if s.Waiting == events.WaitInput {
		return Waiting
	}
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

// unitAt finds the run's last unit started at or before t that has no session yet.
func unitAt(r *Run, t time.Time) *Unit {
	var found *Unit
	for _, u := range r.Units {
		if u.SessionID == "" && u.Session == nil && !u.Started.IsZero() && !u.Started.After(t.Add(time.Minute)) {
			found = u
		}
	}
	return found
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
