package ui

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestAnim(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC) }
	tests := []struct {
		name    string
		still   bool
		started bool
		first   float64
	}{
		{"running app starts at zero", false, true, 0},
		{"still app starts at the value", true, true, 0.75},
		{"single frame starts at the value", false, false, 0.75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(Options{Refresh: time.Second, Clock: clock, Still: tt.still, Color: ColorNever})
			if tt.started {
				a.Init()
			}
			if got := a.anim("gauge", 0.75); got != tt.first {
				t.Fatalf("first value = %v, want %v", got, tt.first)
			}
			for i := 0; i < 3*fps; i++ {
				a.step()
			}
			if got := a.anim("gauge", 0.75); math.Abs(got-0.75) > 0.01 {
				t.Errorf("after three seconds = %v, want 0.75", got)
			}
			if a.moving() {
				t.Errorf("still moving after three seconds")
			}
		})
	}
}

func TestLayout(t *testing.T) {
	has := func(cols []column, key string) bool {
		for _, c := range cols {
			if c.key == key {
				return true
			}
		}
		return false
	}
	width := func(cols []column) int {
		w := 1
		for _, c := range cols {
			w += c.width + 1
		}
		return w
	}
	tests := []struct {
		width   int
		focus   string
		want    []string
		dropped []string
	}{
		{210, "", []string{"project", "tool", "started", "tokens", "cond", "load"}, nil},
		{200, "", []string{"project", "tool", "started", "tokens", "cond", "load"}, nil},
		{180, "", []string{"project", "tool", "model", "state", "cond", "load"}, nil},
		{150, "pilot", []string{"tool", "model", "started"}, []string{"project"}},
		{155, "", []string{"project", "tool", "model", "started", "load"}, nil},
		{150, "", []string{"project", "name", "model", "state", "cond", "units", "started", "load"}, []string{"tool"}},
		{90, "", []string{"project", "kind", "name", "state", "cond", "units", "elapsed"}, []string{"tool", "started", "model", "load", "job", "tokens", "subagents"}},
	}
	for _, tt := range tests {
		cols := layout(tt.focus, tt.width, nil)
		for _, key := range tt.want {
			if !has(cols, key) {
				t.Errorf("width %d: %s missing", tt.width, key)
			}
		}
		for _, key := range tt.dropped {
			if has(cols, key) {
				t.Errorf("width %d: %s should be dropped", tt.width, key)
			}
		}
		if w := width(cols); w != tt.width {
			t.Errorf("width %d: board is %d wide", tt.width, w)
		}
	}
	// At every width the board fills the screen exactly, with no column below its minimum and
	// only the state column above its preferred width.
	for w := 70; w <= 260; w++ {
		cols := layout("", w, nil)
		if got := width(cols); got != w {
			t.Errorf("width %d: board is %d wide", w, got)
		}
		for _, c := range cols {
			pref := 0
			for _, all := range columns {
				if all.key == c.key {
					pref = all.width
				}
			}
			if c.width < c.least() || c.key != "state" && c.width > pref {
				t.Errorf("width %d: %s is %d wide (min %d, prefers %d)", w, c.key, c.width, c.least(), pref)
			}
		}
	}
}

func TestPanelRows(t *testing.T) {
	keys := func(rows [][]column) [][]string {
		var out [][]string
		for _, row := range rows {
			var r []string
			for _, p := range row {
				r = append(r, p.key)
			}
			out = append(out, r)
		}
		return out
	}
	tests := []struct {
		width int
		want  string
	}{
		{250, "[[status machine runs tokens]]"},
		{170, "[[status machine runs tokens]]"},
		{150, "[[status machine] [runs tokens]]"},
		{100, "[[status machine] [runs tokens]]"},
		{80, "[[status machine] [runs] [tokens]]"},
		{60, "[[status] [machine] [runs] [tokens]]"},
		{50, "[[status] [machine] [runs] [tokens]]"},
	}
	for _, tt := range tests {
		rows := panelRows(tt.width)
		if got := fmt.Sprint(keys(rows)); got != tt.want {
			t.Errorf("width %d: %s, want %s", tt.width, got, tt.want)
		}
		for _, row := range rows {
			used := 0
			for _, p := range row {
				used += p.width
				if p.width < p.least() && p.width != tt.width {
					t.Errorf("width %d: %s is %d wide, under its minimum", tt.width, p.key, p.width)
				}
			}
			if used != tt.width {
				t.Errorf("width %d: a row is %d wide", tt.width, used)
			}
		}
	}
}

// TestFrameSize checks that a frame fills the screen exactly at any size, whatever the layout:
// one row of panels or two, the detail panel below the board or beside it.
func TestFrameSize(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC) }
	a := New(Options{Refresh: time.Second, Clock: clock, Still: true, Color: ColorNever})
	for _, stats := range []bool{true, false} {
		a.stats = stats
		for _, width := range []int{30, 45, 60, 80, 100, 120, 150, 160, 200, 229, 230, 250, 320} {
			for _, height := range []int{24, 30, 40, 44, 60} {
				frameSize(t, a.Frame(width, height), width, height)
			}
		}
	}
}

func frameSize(t *testing.T, frame string, width, height int) {
	t.Helper()
	lines := strings.Split(frame, "\n")
	if len(lines) != height {
		t.Errorf("%dx%d: %d lines", width, height, len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != width {
			t.Errorf("%dx%d: line %d is %d wide", width, height, i, w)
			return
		}
	}
}
