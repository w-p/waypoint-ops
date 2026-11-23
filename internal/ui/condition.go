package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/model"
	"github.com/w-p/waypoint-ops/internal/procs"
)

// fatigueStyle colors a fatigue reading: fresh and working dim, tiring and tired amber, exhausted
// red.
func (v *view) fatigueStyle(level string) lipgloss.Style {
	switch level {
	case "tiring", "tired":
		return v.s.attn
	case "exhausted":
		return v.s.bad
	}
	return v.s.dim
}

// conditionCell is the board's CONDITION cell: the fatigue reading, ≈ when worked out by this
// app, then the first flag that applies (`design/session-condition.md`).
func (v *view) conditionCell(c *model.Condition) []part {
	if c == nil || c.Fatigue == "" {
		return one("—", v.s.dim)
	}
	word := c.Fatigue
	if c.Derived {
		word = "≈ " + word
	}
	parts := []part{{word, v.fatigueStyle(c.Fatigue)}}
	if flag := conditionFlag(c); flag != "" {
		parts = append(parts, part{" · ", v.s.dim}, part{flag, v.s.wait})
	}
	return parts
}

// conditionFlag is the one flag the board shows: just compacted, then unsaved work while a
// compaction is possible, then stale drift.
func conditionFlag(c *model.Condition) string {
	switch {
	case c.JustCompacted:
		return "compacted"
	case c.CompactionPossible && c.Unsaved != "" && c.Unsaved != "none":
		return "unsaved"
	case c.Drift == "stale":
		return "stale"
	}
	return ""
}

// loadCell is the board's CPU MEM cell for a process tree.
func (v *view) loadCell(u *procs.Usage) []part {
	if u == nil {
		return one("—", v.s.dim)
	}
	cpu := "—"
	if u.HasCPU {
		cpu = percent(u.CPU)
	}
	style := v.s.text
	if u.HasCPU && u.CPU >= 50 {
		style = v.s.info
	}
	return []part{{fmt.Sprintf("%5s", cpu), style}, {fmt.Sprintf(" %5s", bytes(u.RSS)), v.s.dim}}
}

// conditionLines are the detail panel's lines for a condition: one per reading, then where it
// came from.
func (v *view) conditionLines(c *model.Condition) []string {
	if c == nil {
		return nil
	}
	now := v.snap.Now
	var out []string
	n := func(p *int, word string) string {
		if p == nil {
			return ""
		}
		return plural(*p, word)
	}
	hours := func(p *float64, format string) string {
		if p == nil {
			return ""
		}
		return fmt.Sprintf(format, *p)
	}
	join := func(head string, style lipgloss.Style, parts ...string) string {
		text := style.Render(head)
		for _, p := range parts {
			if p != "" {
				text += v.s.dim.Render(" · ") + p
			}
		}
		return text
	}

	fatigue := []string{n(c.Compactions, "compaction")}
	if c.WorkSummarized != nil {
		fatigue = append(fatigue, fmt.Sprintf("%d work items summarized away", *c.WorkSummarized))
	}
	fatigue = append(fatigue, hours(c.Hours, "%.1f h"))
	if c.Derived && !c.LastCompaction.IsZero() {
		fatigue = append(fatigue, "last compacted "+ago(c.LastCompaction, now))
	}
	if c.JustCompacted {
		fatigue = append(fatigue, v.s.wait.Render("just compacted, project.md not re-read"))
	}
	out = append(out, v.field("fatigue", join(orDash(c.Fatigue), v.fatigueStyle(c.Fatigue), fatigue...)))

	if c.Drift != "" {
		style := v.s.dim
		if levelAt(events.DriftLevels, c.Drift) > 0 {
			style = v.s.wait
		}
		out = append(out, v.field("drift", join(c.Drift, style,
			hours(c.StateCheckH, "state checked %.1f h ago"), n(c.OutsideCommits, "commit")+byOthers(c.OutsideCommits))))
	}
	if c.Unsaved != "" {
		style := v.s.dim
		if levelAt(events.UnsavedLevels, c.Unsaved) > 0 {
			style = v.s.wait
		}
		out = append(out, v.field("unsaved", join(c.Unsaved, style, n(c.UnsavedWork, "work item")+sinceMemory(c.UnsavedWork))))
	}
	context := ""
	if c.ContextK != nil {
		context = fmt.Sprintf("%dk context", *c.ContextK)
	}
	if c.CompactionPossible {
		context = strings.TrimPrefix(context+", ", ", ") + v.s.wait.Render("compaction possible")
	}
	source := "from session.condition, " + ago(c.At, now)
	if c.Derived {
		source = "≈ worked out from the transcript: fatigue only"
	}
	parts := []string{}
	if context != "" {
		parts = append(parts, context)
	}
	parts = append(parts, v.s.dim.Render(source))
	out = append(out, v.field("", strings.Join(parts, v.s.dim.Render(" · "))))
	return out
}

func byOthers(p *int) string {
	if p == nil {
		return ""
	}
	return " by others"
}

func sinceMemory(p *int) string {
	if p == nil {
		return ""
	}
	return " since the memory file"
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func levelAt(levels []string, level string) int {
	for i, l := range levels {
		if l == level {
			return i
		}
	}
	return -1
}

// loadLine is the detail panel's line for a process tree's load.
func (v *view) loadLine(u *procs.Usage, what string) string {
	if u == nil {
		return ""
	}
	cpu := "cpu —"
	if u.HasCPU {
		cpu = "cpu " + percent(u.CPU)
	}
	text := fmt.Sprintf("%s · mem %s · %s", cpu, bytes(u.RSS), plural(u.Procs, "process"))
	if u.Procs != 1 {
		text = fmt.Sprintf("%s · mem %s · %d processes", cpu, bytes(u.RSS), u.Procs)
	}
	if !u.Started.IsZero() {
		text += " · " + what + " started " + clock(u.Started)
	}
	return v.field("load", text)
}
