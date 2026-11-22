package ui

import (
	"fmt"
	"strings"

	"github.com/w-p/waypoint-ops/internal/model"
)

// detailTitle names the selected item in the detail panel's border.
func (v *view) detailTitle(item *model.Item) string {
	switch {
	case item == nil:
		return "detail"
	case item.Run != nil:
		r := item.Run
		return strings.Join([]string{r.Name, r.Project, r.Runbook}, " · ")
	default:
		s := item.Session
		return strings.Join([]string{"session " + s.ID, s.Project, s.Tool.Name + " " + s.Tool.Version}, " · ")
	}
}

// detail draws the panel body for the selected item.
func (v *view) detail(item *model.Item) []string {
	if item == nil {
		return []string{v.s.dim.Render(" nothing selected")}
	}
	if item.Run != nil {
		return v.runDetail(item)
	}
	return v.sessionDetail(item)
}

// field is one labelled line of the detail pane.
func (v *view) field(label, text string) string {
	return " " + v.s.dim.Render(fit(label, 10, false)) + text
}

func (v *view) runDetail(item *model.Item) []string {
	r := item.Run
	now := v.snap.Now
	var lines []string

	switch r.State {
	case model.Finished:
		outcome := v.s.good.Render(r.Outcome)
		if item.Flag != "" {
			outcome = v.s.flag.Render(r.Outcome)
		}
		lines = append(lines, v.field("outcome", outcome+"   "+r.Detail))
		lines = append(lines, v.field("finished", fmt.Sprintf("%s (%s) · took %s",
			clock(r.Finished), ago(r.Finished, now), span(r.Elapsed(now)))))
	case model.Died:
		lines = append(lines, v.field("state", v.s.flag.Render(item.Flag)))
		pid := "no pid file"
		if r.PID > 0 {
			pid = fmt.Sprintf("pid %d is not running", r.PID)
		}
		lines = append(lines, v.field("runner", fmt.Sprintf("%s · last event %s (%s)",
			pid, clock(r.Last), ago(r.Last, now))))
	case model.Paused:
		lines = append(lines, v.field("state", v.s.wait.Render(v.runState(item))))
		lines = append(lines, v.field("waiting", fmt.Sprintf("%s since %s (%s)",
			r.WaitReason, clock(r.WaitingSince), span(now.Sub(r.WaitingSince)))))
		lines = append(lines, v.field("runner", fmt.Sprintf("pid %d alive", r.PID)))
	default:
		lines = append(lines, v.field("state", v.stateStyle(item).Render(v.runState(item))))
		lines = append(lines, v.field("runner", fmt.Sprintf("pid %d alive", r.PID)))
	}

	var limits []string
	limits = append(limits, "started "+clock(r.Started))
	if r.Cap > 0 {
		limits = append(limits, fmt.Sprintf("cap %d", r.Cap))
	}
	if len(r.MayPush) > 0 {
		limits = append(limits, "may push: "+strings.Join(r.MayPush, ", "))
	}
	limits = append(limits, "tool "+r.Tool.Name+" "+r.Tool.Version)
	lines = append(lines, v.field("run", strings.Join(limits, " · ")))
	lines = append(lines, v.unitTable(r)...)

	for _, j := range r.Jobs {
		lines = append(lines, v.field("job", v.jobLine(j)))
	}
	return lines
}

func (v *view) jobLine(j *model.Job) string {
	parts := []string{"unit " + j.Unit, fmt.Sprintf("pid %d", j.PID)}
	switch {
	case j.Alive:
		parts = append(parts, v.s.good.Render("running "+span(v.snap.Now.Sub(j.Started))))
	case j.Ended:
		code := "exit ?"
		if j.ExitCode != nil {
			code = fmt.Sprintf("exit %d", *j.ExitCode)
		}
		parts = append(parts, code, "ran "+span(j.Took))
		if j.Frozen > 0 {
			parts = append(parts, "frozen "+span(j.Frozen))
		}
	default:
		parts = append(parts, v.s.wait.Render("pid gone, no end recorded"))
	}
	parts = append(parts, "started "+clock(j.Started))
	if j.Log != "" {
		parts = append(parts, "log "+j.Log)
	}
	return strings.Join(parts, " · ")
}

func (v *view) unitTable(r *model.Run) []string {
	type col struct {
		title string
		width int
		right bool
	}
	cols := []col{
		{"#", 3, false},
		{"UNIT", 30, false},
		{"STATUS", 11, false},
		{"STARTED", 11, false},
		{"TOOK", 8, true},
		{"TURNS", 5, true},
		{"TOKENS", 6, true},
		{"MEMORY", 6, true},
		{"REASON", 0, false},
	}
	used := 1
	for _, c := range cols {
		used += c.width + 1
	}
	inner := v.width - 2
	cols[len(cols)-1].width = max(10, inner-used)

	var head []string
	for _, c := range cols {
		head = append(head, fit(c.title, c.width, c.right))
	}
	lines := []string{v.s.headBar.Render(fit(" "+strings.Join(head, " "), inner, false))}

	now := v.snap.Now
	for _, u := range r.Units {
		took, turns, tokens, memory := "", "", "", ""
		switch {
		case u.Status == model.Working:
			took = span(now.Sub(u.Started))
		case u.Took > 0:
			took = span(u.Took)
		}
		if u.Turns > 0 {
			turns = fmt.Sprint(u.Turns)
		}
		if u.Tokens != nil {
			tokens = count(u.Tokens.Total())
		}
		if u.Session != nil {
			memory = fmt.Sprint(u.Session.Memory)
		}
		started := ""
		if !u.Started.IsZero() {
			started = clock(u.Started)
		}
		word := statusWords[u.Status]
		if word == "" {
			word = strings.ToLower(u.Status)
		}
		glyph := statusGlyphs[u.Status]
		if glyph == "" {
			glyph = "?"
		}
		values := []string{u.ID, u.Name, glyph + " " + word, started, took, turns, tokens, memory, u.Reason}
		var cells []string
		for i, c := range cols {
			text := fit(values[i], c.width, c.right)
			switch i {
			case 2:
				text = v.statusStyle(u.Status).Render(text)
			case 3:
				text = v.s.dim.Render(text)
			}
			cells = append(cells, text)
		}
		lines = append(lines, " "+strings.Join(cells, " "))
	}
	return lines
}

func (v *view) sessionDetail(item *model.Item) []string {
	s := item.Session
	now := v.snap.Now
	var lines []string

	lines = append(lines, v.field("state", v.stateStyle(item).Render(sessionState(s, now))))
	when := "started " + clock(s.Started)
	if s.Open() {
		when += " · open for " + span(now.Sub(s.Started))
	} else {
		when += " · ended " + clock(s.Ended) + " · took " + span(s.Ended.Sub(s.Started))
	}
	lines = append(lines, v.field("time", when))
	lines = append(lines, v.field("model", shortModel(s.Model)+"   ("+s.Model+")"))
	if s.Tokens != nil {
		t := s.Tokens
		lines = append(lines, v.field("tokens", fmt.Sprintf("%s in all · input %s · cache write %s · cache read %s · output %s",
			count(t.Total()), count(t.Input), count(t.CacheWrite), count(t.CacheRead), count(t.Output))))
	} else {
		lines = append(lines, v.field("tokens", v.s.dim.Render("reported when the session ends")))
	}
	memory := plural(s.Memory, "write")
	if s.MemoryFile != "" {
		memory += " · last " + s.MemoryFile
	}
	lines = append(lines, v.field("memory", memory))
	lines = append(lines, v.field("compacted", plural(s.Compactions, "time")))
	if len(s.Subagents) > 0 {
		lines = append(lines, v.field("subagents", fmt.Sprintf("%d (%d live)", len(s.Subagents), s.Live())))
	}
	if len(s.Units) > 0 {
		open := 0
		for _, u := range s.Units {
			if u.Open() {
				open++
			}
		}
		lines = append(lines, v.field("units", fmt.Sprintf("%s spawned · %d running · %d ended",
			plural(len(s.Units), "unit"), open, len(s.Units)-open)))
	}
	if s.Transcript != "" {
		lines = append(lines, v.field("transcript", s.Transcript))
	}
	return lines
}
