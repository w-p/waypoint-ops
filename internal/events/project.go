package events

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Project follows every event file in one project folder.
type Project struct {
	Name string
	Root string

	// Missing is set when the project folder does not exist.
	Missing bool

	// Files are keyed by path.
	Files map[string]*File

	// PIDs holds each run's pid file, keyed by run folder name; 0 when there is none.
	PIDs map[string]int
}

// NewProject follows the project at root. An empty name falls back to the folder's name.
func NewProject(name, root string) *Project {
	if name == "" {
		name = filepath.Base(root)
	}
	return &Project{
		Name:  name,
		Root:  root,
		Files: map[string]*File{},
		PIDs:  map[string]int{},
	}
}

// Dir is the project's events folder.
func (p *Project) Dir() string {
	return filepath.Join(p.Root, ".waypoint", "events")
}

// Scan finds new event files, reads what was appended to every file, and rereads pid files.
// Errors on single files are returned joined; the rest of the project is still read.
func (p *Project) Scan() error {
	if _, err := os.Stat(p.Root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			p.Missing = true
			return nil
		}
		return err
	}
	p.Missing = false

	runs, err := filepath.Glob(filepath.Join(p.Dir(), "runs", "*", "events.jsonl"))
	if err != nil {
		return err
	}
	sessions, err := filepath.Glob(filepath.Join(p.Dir(), "sessions", "*.jsonl"))
	if err != nil {
		return err
	}

	var errs []error
	seen := map[string]bool{}
	for _, path := range append(runs, sessions...) {
		seen[path] = true
		f, ok := p.Files[path]
		if !ok {
			f = &File{Path: path}
			p.Files[path] = f
		}
		if err := f.Read(); err != nil {
			errs = append(errs, err)
		}
	}
	// A writer prunes its old files; forget them too.
	for path := range p.Files {
		if !seen[path] {
			delete(p.Files, path)
		}
	}

	p.PIDs = map[string]int{}
	for _, path := range runs {
		dir := filepath.Dir(path)
		pid, err := readPID(filepath.Join(dir, "pid"))
		if err != nil {
			errs = append(errs, err)
		}
		p.PIDs[filepath.Base(dir)] = pid
	}
	return errors.Join(errs...)
}

// Events returns every event read so far across the project's files, oldest first.
func (p *Project) Events() []Event {
	var all []Event
	for _, f := range p.Files {
		all = append(all, f.Events...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].TS.Before(all[j].TS)
	})
	return all
}

// Problems lists what is wrong with the project's files, for a person.
func (p *Project) Problems() []string {
	var out []string
	if p.Missing {
		out = append(out, p.Root+": folder not found")
	}
	for _, f := range p.Files {
		if msg := f.Problem(); msg != "" {
			out = append(out, msg)
		}
	}
	sort.Strings(out)
	return out
}

// readPID returns the pid in a run's pid file, or 0 when there is no file.
func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0, nil
	}
	return strconv.Atoi(text)
}
