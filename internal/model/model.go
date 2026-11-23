// Package model turns session events into the things the overview shows: runs with their units
// and jobs, interactive sessions with their subagents and spawned units, and which of them to
// highlight. It knows nothing about terminals, so a web view can reuse it.
package model

import (
	"regexp"
	"time"

	"github.com/w-p/waypoint-ops/internal/claude"
	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/transcript"
)

// Category orders items on the board: flagged first, then active, waiting, quiet.
type Category int

// Board categories, in display order.
const (
	Flagged Category = iota
	Active
	Waiting
	Quiet
)

// RunState is where a run stands.
type RunState int

// Run states.
const (
	Starting RunState = iota
	Running
	Paused
	Finished
	Died
)

// Unit statuses this app adds to the contract's own (DONE, WAITING, BLOCKED, FAILED, NO_STATUS).
const (
	Pending = "pending"
	Working = "running"
	CutOff  = "cut off"
)

// Run outcomes and unit statuses that a project reports as problems. The app highlights them;
// acting on them is the project's business.
var (
	problemOutcomes = map[string]bool{"STOPPED": true, "HALT": true, "CAP": true, "TIMEOUT": true}
	problemStatuses = map[string]bool{"BLOCKED": true, "FAILED": true}
)

// Run is one run-mode run: its units in runbook order and its machine jobs.
type Run struct {
	Project string
	Name    string
	Runbook string
	Tool    events.Tool
	Cap     int
	MayPush []string

	Started  time.Time
	Finished time.Time
	Last     time.Time

	State        RunState
	WaitReason   string
	WaitDetail   string
	WaitingSince time.Time
	Outcome      string
	Detail       string

	PID   int
	Alive bool

	Units []*Unit
	Jobs  []*Job
}

// Unit is one unit of a run.
type Unit struct {
	ID                string
	Name              string
	Status            string
	Reason            string
	SessionID         string
	Started           time.Time
	Took              time.Duration
	Turns             int
	Model             string
	Tokens            *events.Tokens
	PermissionDenials int
	Transcript        string
	TranscriptFormat  string

	// Session is the unit's hook-written session, when one exists.
	Session *Session

	// Activity is what a transcript reader found in the unit's transcript.
	Activity *transcript.Activity
}

// TranscriptAt is where the unit's transcript is and how it is written: from the runner's events,
// or else from the unit's hook-written session.
func (u *Unit) TranscriptAt() (path, format string) {
	if u.Transcript != "" {
		return u.Transcript, u.TranscriptFormat
	}
	if s := u.Session; s != nil && s.Transcript != "" {
		return s.Transcript, s.TranscriptFormat
	}
	return "", ""
}

// Job is a machine job a unit launched.
type Job struct {
	Unit     string
	PID      int
	Log      string
	Started  time.Time
	Ended    bool
	ExitCode *int
	Took     time.Duration
	Frozen   time.Duration
	Alive    bool
}

// Session is an interactive session, a unit's session, or a subagent.
type Session struct {
	Project    string
	ID         string
	Kind       string
	Parent     string
	Tool       events.Tool
	Model      string
	Transcript string

	// TranscriptFormat is how the transcript is written, as the contract names it.
	TranscriptFormat string

	// Activity is what a transcript reader found, when one read the transcript.
	Activity *transcript.Activity

	// Process is the session's entry in Claude Code's list of running sessions. Gone means the
	// list was read and the session was not in it, though no end was recorded.
	Process *claude.Session
	Gone    bool

	// PID is the tool's own process id, from session.started.
	PID int

	// Waiting is why the session last said it waits (session.waiting: input or permission), until
	// a later event from it; WaitingSince is when.
	Waiting      string
	WaitingSince time.Time

	// Listed marks a session no event names, known only from a tool's own list of its running
	// sessions; Folder is where it runs. Inside a watched project it shows like any session (its
	// project may not write events yet). Other marks one outside every watched project: other
	// work, shown dimmed.
	Listed bool
	Other  bool
	Folder string

	Started  time.Time
	Ended    time.Time
	Last     time.Time
	Duration time.Duration
	Tokens   *events.Tokens

	Compactions int
	Memory      int
	MemoryFile  string

	Subagents []*Session
	Units     []*Session

	// joined is set for a unit session matched to a run's unit.
	joined bool

	// readSubagents is set when Subagents came from the transcript rather than from events.
	readSubagents bool

	// started is set once a session.started was seen, so a resume does not move the start.
	started bool
}

// Open reports whether no end has been seen for the session.
func (s *Session) Open() bool {
	return s.Ended.IsZero()
}

// Item is one row of the board: a run or a top-level session.
type Item struct {
	Project  string
	Run      *Run
	Session  *Session
	Category Category

	// Flag says why the item is highlighted; empty when it is not.
	Flag string

	// Attention marks a flag that is a question for the person (a permission prompt, say)
	// rather than a problem.
	Attention bool

	// Last is the item's most recent activity.
	Last time.Time
}

// Key identifies the item across refreshes.
func (i *Item) Key() string {
	if i.Run != nil {
		return i.Project + "/run/" + i.Run.Name
	}
	return i.Project + "/session/" + i.Session.ID
}

// Machine is what all projects share: one machine and one subscription.
type Machine struct {
	SessionsOpen   int
	OtherOpen      int
	SubagentsLive  int
	JobsRunning    int
	RunsUnfinished int
	RunnersAlive   int

	// TokensToday sums tokens reported today, by tool name.
	TokensToday map[string]int64

	// Spend lists token totals reported in the 24 hours before now, oldest first. Tokens are only
	// reported when a unit or session ends, so each total lands at its end time.
	Spend []Spend
}

// Spend is one token total and when it was reported.
type Spend struct {
	At     time.Time
	Tool   string
	Tokens int64
}

// ProjectInfo is a project as configured, with anything wrong reading it.
type ProjectInfo struct {
	Name     string
	Root     string
	Problems []string
}

// Snapshot is everything the overview draws at one moment.
type Snapshot struct {
	Now      time.Time
	Projects []ProjectInfo

	// Items are flagged and unfinished work, in board order.
	Items []*Item

	// Recent is finished work that is not flagged, most recently finished first.
	Recent []*Item

	Machine Machine

	// Listed is set when Claude Code's list of running sessions was read for this snapshot.
	Listed bool
}

var stamp = regexp.MustCompile(`-\d{8}-\d{6}$`)

// Short is the run's name without the start time the runner appends.
func (r *Run) Short() string {
	return stamp.ReplaceAllString(r.Name, "")
}

// Elapsed is how long the run has taken so far, or took in all.
func (r *Run) Elapsed(now time.Time) time.Duration {
	if !r.Finished.IsZero() {
		return r.Finished.Sub(r.Started)
	}
	if r.State == Died {
		return r.Last.Sub(r.Started)
	}
	return now.Sub(r.Started)
}

// Tokens is the sum of every unit's tokens: the reported total for units that ended, the
// transcript's running count for one still working.
func (r *Run) Tokens() int64 {
	var total int64
	for _, u := range r.Units {
		switch {
		case u.Tokens != nil:
			total += u.Tokens.Total()
		case u.Activity != nil:
			total += u.Activity.Tokens.Total()
		}
	}
	return total
}

// Models lists the distinct models the run's units used, in unit order.
func (r *Run) Models() []string {
	var out []string
	seen := map[string]bool{}
	for _, u := range r.Units {
		if u.Model == "" || seen[u.Model] {
			continue
		}
		seen[u.Model] = true
		out = append(out, u.Model)
	}
	return out
}

// Subagents returns the subagents of the run's unit sessions.
func (r *Run) Subagents() []*Session {
	var out []*Session
	for _, u := range r.Units {
		if u.Session != nil {
			out = append(out, u.Session.Subagents...)
		}
	}
	return out
}

// Running reports the unit currently running, or nil.
func (r *Run) Running() *Unit {
	for _, u := range r.Units {
		if u.Status == Working {
			return u
		}
	}
	return nil
}

// Job returns the run's most recent job, or nil.
func (r *Run) Job() *Job {
	if len(r.Jobs) == 0 {
		return nil
	}
	return r.Jobs[len(r.Jobs)-1]
}

// Count is how many units share one status.
type Count struct {
	Status string
	N      int
}

// UnitCounts counts units by status in a fixed order: done, running, waiting, problems, cut off,
// pending, then anything else.
func (r *Run) UnitCounts() []Count {
	order := []string{"DONE", Working, "WAITING", "FAILED", "BLOCKED", CutOff, "NO_STATUS", Pending}
	n := map[string]int{}
	for _, u := range r.Units {
		n[u.Status]++
	}
	var out []Count
	for _, status := range order {
		if n[status] > 0 {
			out = append(out, Count{status, n[status]})
			delete(n, status)
		}
	}
	for status, count := range n {
		out = append(out, Count{status, count})
	}
	return out
}

// Live counts the session's subagents with no end seen, while the session itself is open.
func (s *Session) Live() int {
	if !s.Open() || s.Gone {
		return 0
	}
	live := 0
	for _, a := range s.Subagents {
		if a.Open() {
			live++
		}
	}
	return live
}
