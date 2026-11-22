// Command waypoint-ops shows, in one terminal view, what every agent session and run is doing
// across the projects it watches. It only reads; it never writes into a project.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/w-p/waypoint-ops/internal/config"
	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/ui"
)

const usage = `waypoint-ops: one view of every agent session and run across your projects.

Usage:
  waypoint-ops [flags] [project folder ...]

With folders given, those are watched; otherwise the projects in the settings file are.

Flags:
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "waypoint-ops:", err)
		os.Exit(1)
	}
}

func run() error {
	defaultPath, err := config.Path()
	if err != nil {
		return err
	}
	path := flag.String("config", defaultPath, "settings file")
	refresh := flag.Duration("refresh", 0, "how often to reread the projects (default from settings, 2s)")
	noColor := flag.Bool("no-color", false, "turn colors off (as does NO_COLOR)")
	color := flag.String("color", "", "auto, always or never (default from settings, auto)")
	recent := flag.Duration("recent", 0, "how long finished work stays in view (default from settings, 48h)")
	at := flag.String("at", "", "view as of this time, e.g. 2026-11-21T14:02:00-05:00; the clock runs on from there")
	once := flag.Bool("once", false, "print one frame and exit")
	width := flag.Int("width", 180, "frame width with -once")
	height := flag.Int("height", 50, "frame height with -once")
	focus := flag.String("focus", "", "start focused on this project")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, found, err := config.Load(*path)
	if err != nil {
		return err
	}
	if *refresh > 0 {
		cfg.Refresh.Duration = *refresh
	}
	if *recent > 0 {
		cfg.Recent.Duration = *recent
	}
	if *color != "" {
		cfg.Color = *color
	}
	if *noColor || os.Getenv("NO_COLOR") != "" {
		cfg.Color = ui.ColorNever
	}

	var projects []*events.Project
	if flag.NArg() > 0 {
		for _, dir := range flag.Args() {
			abs, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			projects = append(projects, events.NewProject("", abs))
		}
	} else {
		for _, p := range cfg.Projects {
			projects = append(projects, events.NewProject(p.Name, p.Path))
		}
	}
	if len(projects) == 0 {
		where := *path
		if !found {
			where += " (not found)"
		}
		return fmt.Errorf("no projects to watch: give project folders, or list them in %s:\n\n"+
			"  [[projects]]\n  path = \"/path/to/project\"", where)
	}

	clock := time.Now
	if *at != "" {
		start, err := time.Parse(time.RFC3339, *at)
		if err != nil {
			return fmt.Errorf("-at: %w", err)
		}
		began := time.Now()
		clock = func() time.Time { return start.Add(time.Since(began)) }
	}

	app := ui.New(ui.Options{
		Projects: projects,
		Refresh:  cfg.Refresh.Duration,
		Color:    cfg.Color,
		Recent:   cfg.Recent.Duration,
		Quiet:    cfg.Quiet.Duration,
		Clock:    clock,
	})
	if *focus != "" {
		app.Focus(*focus)
	}
	if *once {
		fmt.Println(app.Frame(*width, *height))
		return nil
	}
	_, err = tea.NewProgram(app, tea.WithAltScreen()).Run()
	return err
}
