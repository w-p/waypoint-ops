// Package ui is the terminal interface: the Bubble Tea program that rereads the projects on an
// interval and draws the overview.
package ui

import (
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/model"
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

	// Still turns animation off: gauges and graphs are drawn at their values straight away.
	Still bool
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
	cursor int

	// selected is the key of the selected item, so the selection survives a refresh.
	selected string

	// Animation: springs move each gauge towards its value; began sets the spinner's phase.
	spring  harmonica.Spring
	springs map[string]*motion
	began   time.Time
	frames  bool
}

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

		spring:  harmonica.NewSpring(harmonica.FPS(fps), 5.0, 0.6),
		springs: map[string]*motion{},
		began:   time.Now(),
	}
	a.load()
	return a
}

// load rereads every project and rebuilds the snapshot.
func (a *App) load() {
	errs := map[string]string{}
	for _, p := range a.opt.Projects {
		if err := p.Scan(); err != nil {
			errs[p.Name] = err.Error()
		}
	}
	a.snap = model.Build(a.opt.Projects, model.Options{
		Now:    a.opt.Clock(),
		Recent: a.opt.Recent,
		Quiet:  a.opt.Quiet,
	})
	for i, p := range a.snap.Projects {
		if msg, ok := errs[p.Name]; ok {
			a.snap.Projects[i].Problems = append(a.snap.Projects[i].Problems, msg)
		}
	}
	a.restore()
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
	return tea.Batch(a.next(), a.frame())
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
		a.load()
		return a, a.next()
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
		cursor:  a.cursor,
		refresh: a.opt.Refresh.String(),
		window:  a.opt.Recent,
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
	a.cursor = n
	a.restore()
	a.remember()
}
