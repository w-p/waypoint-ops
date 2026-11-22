// Package events reads Waypoint's session-events contract: one JSON object per line, appended to
// files under a project's .waypoint/events/ folder. The contract is Waypoint's; this package only
// mirrors it.
package events

import (
	"time"
)

// Version is the contract major version this reader understands.
const Version = 1

// Run and session event types.
const (
	RunStarted       = "run.started"
	RunWaiting       = "run.waiting"
	RunResumed       = "run.resumed"
	RunFinished      = "run.finished"
	UnitStarted      = "unit.started"
	UnitEnded        = "unit.ended"
	JobStarted       = "job.started"
	JobEnded         = "job.ended"
	SessionStarted   = "session.started"
	SessionCompacted = "session.compacted"
	SessionMemory    = "session.memory_written"
	SessionEnded     = "session.ended"
)

// Session kinds.
const (
	Interactive = "interactive"
	Unit        = "unit"
	Subagent    = "subagent"
)

// Tool names the tool that wrote an event.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Session identifies the session an event is about. Parent is a session id, a run name, or empty.
type Session struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Parent string `json:"parent"`
}

// Tokens are a session's token totals.
type Tokens struct {
	Input      int64 `json:"input"`
	CacheWrite int64 `json:"cache_write"`
	CacheRead  int64 `json:"cache_read"`
	Output     int64 `json:"output"`
}

// Total is the sum of all four counts.
func (t Tokens) Total() int64 {
	return t.Input + t.CacheWrite + t.CacheRead + t.Output
}

// Transcript points at a tool's own record of a session.
type Transcript struct {
	Path   string `json:"path"`
	Format string `json:"format"`
}

// Event is one line of an event file. Which fields are set depends on Type; the contract lists
// them per event type. Fields this reader does not know are ignored, as the contract asks.
type Event struct {
	V       int       `json:"v"`
	TS      time.Time `json:"ts"`
	Type    string    `json:"event"`
	Project string    `json:"project"`
	Tool    Tool      `json:"tool"`
	Writer  string    `json:"writer"`
	Run     string    `json:"run"`
	Session *Session  `json:"session"`

	// run.started
	Runbook string            `json:"runbook"`
	Units   []string          `json:"units"`
	Names   map[string]string `json:"names"`
	Cap     int               `json:"cap"`
	MayPush []string          `json:"may_push"`
	WaitFor string            `json:"wait_for"`

	// run.waiting, run.resumed, unit.ended, run.finished
	Reason  string `json:"reason"`
	Detail  string `json:"detail"`
	Outcome string `json:"outcome"`

	// unit.*, job.*
	Unit              string      `json:"unit"`
	Status            string      `json:"status"`
	ExitCode          *int        `json:"exit_code"`
	DurationS         float64     `json:"duration_s"`
	Model             string      `json:"model"`
	Turns             int         `json:"turns"`
	Tokens            *Tokens     `json:"tokens"`
	PermissionDenials int         `json:"permission_denials"`
	Transcript        *Transcript `json:"transcript"`
	PID               int         `json:"pid"`
	Log               string      `json:"log"`
	FrozenS           float64     `json:"frozen_s"`

	// session.memory_written
	File string `json:"file"`
}

// Duration is DurationS as a time.Duration.
func (e Event) Duration() time.Duration {
	return seconds(e.DurationS)
}

// Frozen is FrozenS as a time.Duration.
func (e Event) Frozen() time.Duration {
	return seconds(e.FrozenS)
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}
