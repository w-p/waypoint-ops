// Package ui is the terminal interface: the Bubble Tea program that rereads the projects on an
// interval and draws the overview.
package ui

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/w-p/waypoint-ops/internal/claude"
	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/model"
	"github.com/w-p/waypoint-ops/internal/procs"
	"github.com/w-p/waypoint-ops/internal/transcript"
)

// Options configure the app.
type Options struct {
	Projects []*events.Project
	Refresh  time.Duration
	Color    string
	Recent   time.Duration
	Quiet    time.Duration

	// Clock gives the current time. It can be moved into the past to view the test data as it
	// stood then.
	Clock func() time.Time

	// Live asks Claude Code which of its sessions are running. It is off when the clock has
	// been moved into the past, because the list only describes the present.
	Live bool

	// OtherWork shows Claude Code sessions that no event names, dimmed (`design/other-work.md`).
	OtherWork bool

	// HideStats starts with the panels across the top hidden; the s key shows them.
	HideStats bool

	// Still turns animation off: gauges and graphs are drawn at their values straight away, and
	// transcripts are read before the first frame instead of in the background.
	Still bool

	// Load samples the machine's CPU and memory, and each open session's and run's, on every
	// refresh. Off when the clock has been moved into the past.
	Load bool
}

// App is the Bubble Tea model.
type App struct {
	opt    Options
	s      styles
	snap   model.Snapshot
	width  int
	height int
	focus  string
	recent bool
	other  bool
	stats  bool
	cursor int

	// selected is the key of the selected item, so the selection survives a refresh.
	selected string

	// Animation: springs move each gauge towards its value; began sets the spinner's phase.
	spring  harmonica.Spring
	springs map[string]*motion
	began   time.Time
	frames  bool

	// errs are problems scanning each project, by project name.
	errs map[string]string

	// Transcripts: one reader per path, read in the background one batch at a time, and the
	// activity the last batch found.
	readers  map[string]*transcript.Reader
	activity map[string]*transcript.Activity
	reading  bool

	// Claude Code's list of running sessions, the last reading of it, and where this machine's
	// Claude Code keeps its transcripts.
	running claude.List
	asking  bool
	local   []string

	// Load: the sampler, its latest reading, and whether a sample is being taken.
	sampler  *procs.Sampler
	usage    procs.Reading
	sampling bool
}

// usageMsg carries a load sample.
type usageMsg procs.Reading

// runningMsg carries a reading of Claude Code's running sessions.
type runningMsg struct {
	list claude.List
	err  error
}

type runningTick struct{}

// runningEvery is how often Claude Code is asked for its running sessions. Each ask starts a
// process, so it is slower than the refresh.
const runningEvery = 5 * time.Second

// activityMsg carries what a background batch of transcript reads found.
type activityMsg map[string]*transcript.Activity

// motion is one animated value.
type motion struct {
	pos, vel, target float64
}

type tick struct{}

type frame struct{}

const fps = 30

// New makes the app and reads the projects once.
func New(opt Options) *App {
	a := &App{
		opt:    opt,
		s:      newStyles(opt.Color),
		width:  180,
		height: 50,
		recent: true,
		other:  opt.OtherWork,
		stats:  !opt.HideStats,

		spring:  harmonica.NewSpring(harmonica.FPS(fps), 5.0, 0.6),
		springs: map[string]*motion{},
		began:   time.Now(),

		readers:  map[string]*transcript.Reader{},
		activity: map[string]*transcript.Activity{},
		local:    localTranscripts(),
	}
	if opt.Load {
		a.sampler = procs.NewSampler()
	}
	a.scan()
	if opt.Still && opt.Live {
		if ask := a.askRunning(); ask != nil {
			a.takeRunning(ask().(runningMsg))
		}
	}
	a.build()
	if opt.Still {
		if read := a.readTranscripts(); read != nil {
			a.apply(read().(activityMsg))
		}
		if a.sampler != nil {
			// CPU is use over time, so one frame needs two samples.
			a.sampler.Sample(a.roots())
			time.Sleep(stillSample)
			a.takeUsage(usageMsg(a.sampler.Sample(a.roots())))
		}
	}
	return a
}

// stillSample is the gap between the two load samples of a single frame.
const stillSample = 300 * time.Millisecond

// roots are the processes whose load is sampled: each open session's and each live runner's.
func (a *App) roots() []int {
	var out []int
	for _, item := range a.snap.Items {
		switch {
		case item.Run != nil && item.Run.Alive:
			out = append(out, item.Run.PID)
		case item.Session != nil && item.Session.Open() && !item.Session.Gone:
			if pid := item.Session.ProcessID(); pid > 0 {
				out = append(out, pid)
			}
		}
	}
	return out
}

// sample returns a command that takes a load sample off the screen's thread, or nil when
// sampling is off or a sample is already being taken.
func (a *App) sample() tea.Cmd {
	if a.sampler == nil || a.sampling {
		return nil
	}
	a.sampling = true
	roots := a.roots()
	sampler := a.sampler
	return func() tea.Msg {
		return usageMsg(sampler.Sample(roots))
	}
}

func (a *App) takeUsage(msg usageMsg) {
	a.sampling = false
	a.usage = procs.Reading(msg)
	a.build()
}

// load rereads every project, rebuilds the snapshot, and returns the command that reads the
// transcripts it names.
func (a *App) load() tea.Cmd {
	a.scan()
	a.build()
	return a.readTranscripts()
}

// scan rereads every project's event files.
func (a *App) scan() {
	a.errs = map[string]string{}
	for _, p := range a.opt.Projects {
		if err := p.Scan(); err != nil {
			a.errs[p.Name] = err.Error()
		}
	}
}

// build makes the snapshot from what has been read so far.
func (a *App) build() {
	a.snap = model.Build(a.opt.Projects, model.Options{
		Now:      a.opt.Clock(),
		Recent:   a.opt.Recent,
		Quiet:    a.opt.Quiet,
		Activity: a.activity,

		Running:          a.running,
		LocalTranscripts: a.local,
		OtherWork:        a.other,
		Load:             a.usage,
	})
	for i, p := range a.snap.Projects {
		if msg, ok := a.errs[p.Name]; ok {
			a.snap.Projects[i].Problems = append(a.snap.Projects[i].Problems, msg)
		}
	}
	a.restore()
}

// transcriptWindow is how long after a session ends its transcript is still read, so the token
// graph keeps its times.
const transcriptWindow = 24 * time.Hour

// readTranscripts returns a command that reads, off the screen's thread, every transcript the
// snapshot's sessions name in a format with a reader: open sessions, and those that ended within
// the transcript window. It returns nil while an earlier batch is still reading.
func (a *App) readTranscripts() tea.Cmd {
	if a.reading {
		return nil
	}
	now := a.opt.Clock()
	batch := map[string]*transcript.Reader{}
	want := func(path, format string) {
		if path == "" || !transcript.Supported(format) {
			return
		}
		r := a.readers[path]
		if r == nil {
			r = transcript.NewReader(path)
		}
		batch[path] = r
	}
	for _, item := range append(append([]*model.Item{}, a.snap.Items...), a.snap.Recent...) {
		if r := item.Run; r != nil {
			// Units' transcripts: the one running, and those that ended within the window.
			for _, u := range r.Units {
				if u.Status == model.Working || (!u.Started.IsZero() && now.Sub(u.Started.Add(u.Took)) <= transcriptWindow) {
					want(u.TranscriptAt())
				}
			}
			continue
		}
		s := item.Session
		if s == nil || (!s.Open() && now.Sub(s.Ended) > transcriptWindow) {
			continue
		}
		want(s.Transcript, s.TranscriptFormat)
	}
	a.readers = batch // readers no longer wanted are dropped
	if len(batch) == 0 {
		return nil
	}
	a.reading = true
	return func() tea.Msg {
		found := activityMsg{}
		for path, r := range batch {
			if r.Read(now) == nil {
				act := r.Activity()
				found[path] = &act
			}
		}
		return found
	}
}

// localTranscripts are the path prefixes of this machine's Claude Code folder, as events may
// write them: absolute, or under "~/".
func localTranscripts() []string {
	home, _ := os.UserHomeDir()
	dir := claude.Dir()
	if dir == "" {
		return nil
	}
	out := []string{dir + string(filepath.Separator)}
	if home != "" && dir == filepath.Join(home, ".claude") {
		out = append(out, "~/.claude/")
	}
	return out
}

// askRunning returns a command that asks Claude Code for its running sessions, or nil when
// asking is off or an ask is already out.
func (a *App) askRunning() tea.Cmd {
	if !a.opt.Live || a.asking {
		return nil
	}
	a.asking = true
	now := a.opt.Clock()
	return func() tea.Msg {
		list, err := claude.Read(context.Background(), now)
		return runningMsg{list, err}
	}
}

// takeRunning keeps a reading. A failed reading says nothing about which sessions run, so no
// session is called gone on its account; without the claude command, asking stops.
func (a *App) takeRunning(msg runningMsg) {
	a.asking = false
	if msg.err != nil {
		a.running = claude.List{}
		if errors.Is(msg.err, claude.ErrMissing) {
			a.opt.Live = false
		}
		return
	}
	a.running = claude.Locate(claude.Track(a.running, msg.list), claude.Dir())
}

func (a *App) nextRunning() tea.Cmd {
	if !a.opt.Live {
		return nil
	}
	return tea.Tick(runningEvery, func(time.Time) tea.Msg { return runningTick{} })
}

// apply takes in a finished batch of transcript reads and rebuilds the snapshot with it.
func (a *App) apply(found activityMsg) {
	a.reading = false
	a.activity = found
	a.build()
}

// all lists the selectable rows as they are drawn.
func (a *App) all() []*model.Item {
	items, done := rows(a.snap, a.focus, a.recent)
	return append(items, done...)
}

// restore moves the cursor back to the selected item after the rows changed.
func (a *App) restore() {
	list := a.all()
	for i, item := range list {
		if item.Key() == a.selected {
			a.cursor = i
			return
		}
	}
	if a.cursor >= len(list) {
		a.cursor = len(list) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
	a.remember()
}

func (a *App) remember() {
	list := a.all()
	if a.cursor >= 0 && a.cursor < len(list) {
		a.selected = list[a.cursor].Key()
	}
}

func (a *App) next() tea.Cmd {
	return tea.Tick(a.opt.Refresh, func(time.Time) tea.Msg { return tick{} })
}

// Init starts the refresh timer and the animation.
func (a *App) Init() tea.Cmd {
	a.frames = true
	return tea.Batch(a.next(), a.frame(), a.readTranscripts(), a.askRunning(), a.sample())
}

// frame schedules the next animation frame: fast while a spring is moving, slower when only the
// spinner and the pulse need drawing.
func (a *App) frame() tea.Cmd {
	every := time.Second / 10
	if a.moving() {
		every = time.Second / fps
	}
	return tea.Tick(every, func(time.Time) tea.Msg { return frame{} })
}

func (a *App) moving() bool {
	for _, m := range a.springs {
		if math.Abs(m.pos-m.target) > 0.001 || math.Abs(m.vel) > 0.001 {
			return true
		}
	}
	return false
}

// step advances every spring one frame.
func (a *App) step() {
	for _, m := range a.springs {
		m.pos, m.vel = a.spring.Update(m.pos, m.vel, m.target)
	}
}

// anim gives the current value for key and sets where it is heading. A new key starts at zero,
// so gauges fill when they first appear; with Still set it starts at its value.
func (a *App) anim(key string, target float64) float64 {
	m, ok := a.springs[key]
	if !ok {
		m = &motion{}
		if a.opt.Still || !a.frames {
			m.pos = target
		}
		a.springs[key] = m
	}
	m.target = target
	return m.pos
}

// Update handles keys, resizes and the refresh timer.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
	case tick:
		return a, tea.Batch(a.next(), a.load(), a.sample())
	case usageMsg:
		a.takeUsage(msg)
	case activityMsg:
		a.apply(msg)
	case runningTick:
		return a, a.askRunning()
	case runningMsg:
		a.takeRunning(msg)
		a.build()
		return a, a.nextRunning()
	case frame:
		a.step()
		return a, a.frame()
	case tea.KeyMsg:
		return a, a.key(msg.String())
	}
	return a, nil
}

func (a *App) key(k string) tea.Cmd {
	list := a.all()
	switch k {
	case "q", "ctrl+c":
		return tea.Quit
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}
	case "down", "j":
		if a.cursor < len(list)-1 {
			a.cursor++
		}
	case "home", "g":
		a.cursor = 0
	case "end", "G":
		a.cursor = len(list) - 1
	case "f":
		if a.cursor >= 0 && a.cursor < len(list) {
			a.focus = list[a.cursor].Project
			a.restore()
		}
	case "esc":
		a.focus = ""
		a.restore()
	case "r":
		a.recent = !a.recent
		a.restore()
	case "o":
		a.other = !a.other
		a.build()
	case "s":
		a.stats = !a.stats
	}
	a.remember()
	return nil
}

// View draws the screen.
func (a *App) View() string {
	return a.Frame(a.width, a.height)
}

// Frame draws the screen at a given size.
func (a *App) Frame(width, height int) string {
	v := view{
		s:       a.s,
		snap:    a.snap,
		width:   width,
		height:  height,
		focus:   a.focus,
		recent:  a.recent,
		other:   a.other,
		stats:   a.stats,
		cursor:  a.cursor,
		refresh: a.opt.Refresh.String(),
		window:  a.opt.Recent,
		quiet:   a.opt.Quiet,
		phase:   time.Since(a.began).Seconds(),
		anim:    a.anim,
	}
	return v.render()
}

// Focus shows only one project, as the f key does.
func (a *App) Focus(project string) {
	a.focus = project
	a.restore()
}

// Select moves the cursor to row n, as the arrow keys do.
func (a *App) Select(n int) {
	a.cursor = max(0, min(n, len(a.all())-1))
	a.remember()
}
