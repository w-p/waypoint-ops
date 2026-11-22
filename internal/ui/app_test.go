package ui

import (
	"math"
	"testing"
	"time"
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
		{200, "", []string{"project", "tool", "started", "tokens"}, nil},
		{200, "pilot", []string{"tool"}, []string{"project"}},
		{150, "", []string{"project", "name", "state", "units"}, []string{"tool", "started"}},
		{90, "", []string{"project", "name", "state", "units"}, []string{"tool", "job", "kind"}},
	}
	for _, tt := range tests {
		cols := layout(tt.focus, tt.width)
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
		if w := width(cols); w > tt.width && tt.width >= 100 {
			t.Errorf("width %d: board is %d wide", tt.width, w)
		}
	}
}
