package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/w-p/waypoint-ops/internal/claude"
	"github.com/w-p/waypoint-ops/internal/events"
	"github.com/w-p/waypoint-ops/internal/model"
	"github.com/w-p/waypoint-ops/internal/transcript"
)

// column is one board column. Like a flex item it has the width it prefers, the least it can
// take (min; 0 means it never shrinks) and its share of any spare room (grow).
type column struct {
	key   string
	title string
	width int
	min   int
	grow  int
	right bool
}

// columns are the board's columns, left to right: key, title, width, min, grow, right-aligned.
// The layout shrinks them toward their minimums on a narrow terminal and drops columns only when
// even those do not fit; the state column takes any spare width.
var columns = []column{
	{"mark", "", 1, 0, 0, false},
	{"project", "PROJECT", 12, 8, 0, false},
	{"kind", "KIND", 8, 0, 0, false},
	{"name", "NAME", 15, 8, 0, false},
	{"tool", "TOOL", 11, 6, 0, false},
	{"model", "MODEL", 9, 7, 0, false},
	{"state", "STATE", 28, 22, 1, false},
	{"cond", "CONDITION", 20, 11, 0, false},
	{"units", "UNITS", 22, 14, 0, false},
	{"subagents", "SUBAGENTS", 12, 9, 0, false},
	{"job", "JOB", 16, 10, 0, false},
	{"load", "  CPU   MEM", 11, 0, 0, false},
	{"started", "STARTED", 11, 0, 0, false},
	{"elapsed", "ELAPSED", 8, 7, 0, true},
	{"tokens", "TOKENS", 6, 0, 0, true},
}

// least is the narrowest a column can be.
func (c column) least() int {
	if c.min == 0 {
		return c.width
	}
	return c.min
}

// part is a run of text in one style; a cell is one or more parts.
type part struct {
	text  string
	style lipgloss.Style
}

type cells map[string][]part

func one(text string, style lipgloss.Style) []part {
	return []part{{text, style}}
}

func plainText(parts []part) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.text)
	}
	return b.String()
}

// dropOrder is the order columns leave a board too narrow for every column at its minimum, least
// needed first.
var dropOrder = []string{"tool", "started", "model", "load", "job", "tokens", "subagents", "elapsed", "cond", "kind"}

// boardWidth is the width columns take at the given widths: a leading space, a space after each.
func boardWidth(cols []column, width func(column) int) int {
	used := 1
	for _, c := range cols {
		used += width(c) + 1
	}
	return used
}

// layout returns the columns shown at a width and how wide each is. Content, when given, is the
// widest cell in each column: a column never asks for more than that (or its title), so room goes
// to the columns with text to show. The project column is dropped while one project is in focus;
// columns in dropOrder go only while the rest do not fit at their minimums; then the columns flex
// to fill the width exactly.
func layout(focus string, width int, content map[string]int) []column {
	sized := make([]column, len(columns))
	copy(sized, columns)
	if content != nil {
		for i := range sized {
			c := &sized[i]
			if c.grow > 0 {
				continue
			}
			want := max(content[c.key], runewidth.StringWidth(c.title), 1)
			if want < c.width {
				c.min = min(c.least(), want)
				c.width = want
			}
		}
	}
	gone := map[string]bool{"project": focus != ""}
	shown := func() []column {
		var out []column
		for _, c := range sized {
			if !gone[c.key] {
				out = append(out, c)
			}
		}
		return out
	}
	for _, key := range dropOrder {
		if boardWidth(shown(), column.least) <= width {
			break
		}
		gone[key] = true
	}
	cols := shown()
	room := width - boardWidth(cols, func(c column) int { return c.width })
	switch {
	case room > 0:
		share(cols, room, func(c column) int { return c.grow }, 1)
	case room < 0:
		// The columns that grow are read first, so they give way last.
		short := -room
		for _, growing := range []bool{false, true} {
			before := boardWidth(cols, func(c column) int { return c.width })
			share(cols, short, func(c column) int {
				if (c.grow > 0) != growing {
					return 0
				}
				return c.width - c.least()
			}, -1)
			short -= before - boardWidth(cols, func(c column) int { return c.width })
		}
	}
	return cols
}

// contentWidths is the widest cell in each column over the given rows.
func (v *view) contentWidths(lists ...[]*model.Item) map[string]int {
	widths := map[string]int{}
	for _, items := range lists {
		for _, item := range items {
			for key, parts := range v.cells(item, false) {
				widths[key] = max(widths[key], runewidth.StringWidth(plainText(parts)))
			}
		}
	}
	return widths
}

// share grows (sign 1) or shrinks (sign -1) the columns by n cells in all, each in proportion to
// its weight. A column shrinks by no more than its weight, which is how far it can give.
func share(cols []column, n int, weight func(column) int, sign int) {
	weights := make([]int, len(cols))
	total := 0
	for i, c := range cols {
		weights[i] = max(0, weight(c))
		total += weights[i]
	}
	if total == 0 {
		return
	}
	if sign < 0 {
		n = min(n, total)
	}
	given := make([]int, len(cols))
	left := n
	for i, w := range weights {
		given[i] = n * w / total
		left -= given[i]
	}
	// Rounding leaves a few cells; hand them out one at a time, left to right.
	for left > 0 {
		for i, w := range weights {
			if left > 0 && w > 0 && (sign > 0 || given[i] < w) {
				given[i]++
				left--
			}
		}
	}
	for i := range cols {
		cols[i].width += sign * given[i]
	}
}

// tinted gives a style the row's background, when it has one.
func tinted(style lipgloss.Style, bg lipgloss.Color) lipgloss.Style {
	if bg == "" {
		return style
	}
	return style.Background(bg)
}

// render draws a cell at its column's width on the row's background.
func (c column) render(parts []part, bg lipgloss.Color) string {
	plain := plainText(parts)
	width := runewidth.StringWidth(plain)
	if width > c.width || len(parts) == 1 {
		style := lipgloss.NewStyle()
		if len(parts) > 0 {
			style = parts[0].style
		}
		return tinted(style, bg).Render(fit(plain, c.width, c.right))
	}
	var out strings.Builder
	for _, p := range parts {
		out.WriteString(tinted(p.style, bg).Render(p.text))
	}
	out.WriteString(tinted(lipgloss.NewStyle(), bg).Render(strings.Repeat(" ", c.width-width)))
	return out.String()
}

func (v *view) headerRow(cols []column, width int) string {
	var out []string
	for _, c := range cols {
		out = append(out, fit(c.title, c.width, c.right))
	}
	return v.s.headBar.Render(fit(" "+strings.Join(out, " "), width, false))
}

// row draws one board row across the full width. The selected row is a bright bar of plain
// text, because per-cell colors would fight it; flagged rows carry a faint red tint.
func (v *view) row(item *model.Item, cols []column, width int, selected, finished bool) string {
	c := v.cells(item, finished)
	if selected {
		var plain []string
		for _, col := range cols {
			plain = append(plain, fit(plainText(c[col.key]), col.width, col.right))
		}
		return v.s.selectd.Render(fit(" "+strings.Join(plain, " "), width, false))
	}
	var bg lipgloss.Color
	switch {
	case item.Attention:
		bg = v.s.attnTint
	case item.Flag != "":
		bg = v.s.flagTint
	}
	gap := tinted(lipgloss.NewStyle(), bg).Render(" ")
	var b strings.Builder
	b.WriteString(gap)
	used := 1
	for i, col := range cols {
		if i > 0 {
			b.WriteString(gap)
			used++
		}
		b.WriteString(col.render(c[col.key], bg))
		used += col.width
	}
	if used < width {
		b.WriteString(tinted(lipgloss.NewStyle(), bg).Render(strings.Repeat(" ", width-used)))
	}
	return b.String()
}

func (v *view) cells(item *model.Item, finished bool) cells {
	s := v.s
	c := cells{
		"mark":    v.mark(item, finished),
		"project": one(item.Project, s.project),
	}
	if item.Run != nil {
		v.runCells(c, item)
	} else {
		v.sessionCells(c, item)
	}
	if other(item) {
		// Other work is dimmed throughout, so Waypoint work stands out.
		for _, parts := range c {
			for i := range parts {
				parts[i].style = s.dim
			}
		}
		return c
	}
	if finished && item.Flag == "" {
		for key, parts := range c {
			if key == "state" || key == "mark" || key == "units" || key == "cond" {
				continue
			}
			for i := range parts {
				parts[i].style = s.dim
			}
		}
	}
	return c
}

// other reports whether an item is other work shown dimmed: a session no event names, unless it
// waits on a person.
func other(item *model.Item) bool {
	return item.Session != nil && item.Session.Other && !item.Attention
}

// mark is the first cell: a pulsing "!" for flagged work, a spinner for work in progress, bars
// for waiting, a dot for quiet sessions and a tick for finished work.
func (v *view) mark(item *model.Item, finished bool) []part {
	s := v.s
	switch {
	case item.Flag != "":
		return one("!", s.pulse(v.phase, item.Attention))
	case item.Session != nil && item.Session.Gone:
		return one("–", s.dim)
	case finished:
		if item.Run != nil && item.Run.Outcome == "DONE" || item.Session != nil {
			return one("✓", s.good)
		}
		return one(" ", s.dim)
	case item.Category == model.Active:
		return one(v.spin(), s.good)
	case item.Category == model.Waiting:
		return one("‖", s.wait)
	}
	return one("·", s.wait)
}

func (v *view) runCells(c cells, item *model.Item) {
	s := v.s
	r := item.Run
	now := v.snap.Now
	models := r.Models()
	modelName := "—"
	if len(models) == 1 {
		modelName = shortModel(models[0])
	} else if len(models) > 1 {
		modelName = "mixed"
	}
	c["kind"] = one("run", s.text)
	c["name"] = one(r.Short(), s.bold)
	c["tool"] = one(r.Tool.Name, s.dim)
	c["model"] = one(modelName, s.dim)
	c["state"] = one(v.runState(item), v.stateStyle(item))
	c["units"] = v.unitBar(r)
	c["subagents"] = v.dots(r.Subagents(), r.State != model.Finished && r.State != model.Died)
	c["job"] = v.jobCell(r)
	c["started"] = one(clock(r.Started), s.dim)
	var cond *model.Condition
	if u := r.Running(); u != nil {
		cond = u.Condition
	}
	c["cond"] = v.conditionCell(cond)
	c["load"] = v.loadCell(r.Load)
	elapsed := span(r.Elapsed(now))
	if r.State == model.Died {
		// The runner's death was not recorded, so the run lasted at least this long.
		elapsed += "+"
	}
	c["elapsed"] = one(elapsed, s.text)
	tokens := "—"
	if n := r.Tokens(); n > 0 {
		tokens = count(n)
	}
	c["tokens"] = one(tokens, s.info)
}

func (v *view) runState(item *model.Item) string {
	r := item.Run
	switch r.State {
	case model.Died:
		return item.Flag
	case model.Finished:
		if item.Flag != "" {
			return item.Flag
		}
		return r.Outcome
	case model.Paused:
		text := "waiting"
		if r.WaitDetail != "" {
			return text + ": " + r.WaitDetail
		}
		if r.WaitReason != "" {
			return text + ": " + r.WaitReason
		}
		return text
	case model.Running:
		u := r.Running()
		text := fmt.Sprintf("running unit %s", u.ID)
		if u.Name != "" {
			text += ": " + u.Name
		}
		if a := u.Activity; a != nil && a.Turn == transcript.Working && a.Tool != "" {
			text = fmt.Sprintf("unit %s%s", u.ID, doing(a, " · "))
			if u.Name != "" {
				text += " · " + u.Name
			}
		}
		if item.Flag != "" {
			text = item.Flag + " · " + text
		}
		return text
	default:
		return "starting"
	}
}

func (v *view) stateStyle(item *model.Item) lipgloss.Style {
	if item.Attention {
		return v.s.attn
	}
	if item.Session != nil && item.Session.Gone {
		return v.s.dim
	}
	if item.Flag != "" {
		return v.s.flag
	}
	switch item.Category {
	case model.Active:
		return v.s.good
	case model.Waiting:
		return v.s.wait
	}
	if item.Run != nil && item.Run.Outcome == "DONE" {
		return v.s.good
	}
	if item.Session != nil && item.Session.Open() {
		return v.s.wait
	}
	return v.s.dim
}

// statusWords says how each unit status reads.
var statusWords = map[string]string{
	"DONE":        "done",
	model.Working: "running",
	"WAITING":     "waiting",
	"FAILED":      "failed",
	"BLOCKED":     "blocked",
	model.CutOff:  "cut off",
	"NO_STATUS":   "no status",
	model.Pending: "pending",
}

// statusGlyphs draw one unit each in a unit bar; each status has its own shape so the bar reads
// without color.
var statusGlyphs = map[string]string{
	"DONE":        "■",
	model.Working: "◆",
	"WAITING":     "◇",
	"FAILED":      "✕",
	"BLOCKED":     "✕",
	model.CutOff:  "✕",
	"NO_STATUS":   "?",
	model.Pending: "□",
}

func (v *view) statusStyle(status string) lipgloss.Style {
	switch status {
	case "DONE":
		return v.s.good
	case model.Working:
		return v.s.info
	case "WAITING", "NO_STATUS":
		return v.s.wait
	case "FAILED", "BLOCKED", model.CutOff:
		return v.s.bad
	case model.Pending:
		return v.s.empty
	}
	return v.s.text
}

// barCells is the most units a bar draws one by one; longer runs are drawn to scale.
const barCells = 10

// unitBar draws a run's units in runbook order, one glyph per unit, then done of total.
func (v *view) unitBar(r *model.Run) []part {
	if len(r.Units) == 0 {
		return one("—", v.s.dim)
	}
	var statuses []string
	if len(r.Units) <= barCells {
		for _, u := range r.Units {
			statuses = append(statuses, u.Status)
		}
	} else {
		for _, cnt := range r.UnitCounts() {
			n := max(1, cnt.N*barCells/len(r.Units))
			for i := 0; i < n; i++ {
				statuses = append(statuses, cnt.Status)
			}
		}
	}
	var parts []part
	for _, status := range statuses {
		glyph := statusGlyphs[status]
		if glyph == "" {
			glyph = "?"
		}
		parts = append(parts, part{glyph, v.statusStyle(status)})
	}
	done := 0
	for _, u := range r.Units {
		if u.Status == "DONE" {
			done++
		}
	}
	parts = append(parts, part{fmt.Sprintf(" %d/%d done", done, len(r.Units)), v.s.dim})
	return parts
}

// dots draws subagents as filled dots for live ones and hollow dots for ended ones.
func (v *view) dots(list []*model.Session, open bool) []part {
	if len(list) == 0 {
		return one("—", v.s.dim)
	}
	live := 0
	for _, a := range list {
		if open && a.Open() {
			live++
		}
	}
	shown := min(len(list), 5)
	filled := min(live, shown)
	parts := []part{
		{strings.Repeat("●", filled), v.s.sub},
		{strings.Repeat("○", shown-filled), v.s.dim},
	}
	label := fmt.Sprintf(" %d", len(list))
	if open {
		label = fmt.Sprintf(" %d/%d", live, len(list))
	}
	return append(parts, part{label, v.s.dim})
}

func (v *view) jobCell(r *model.Run) []part {
	j := r.Job()
	if j == nil {
		return one("—", v.s.dim)
	}
	switch {
	case j.Alive:
		return []part{{v.spin() + " ", v.s.good}, {"running " + span(v.snap.Now.Sub(j.Started)), v.s.good}}
	case j.Ended:
		code := "?"
		if j.ExitCode != nil {
			code = fmt.Sprint(*j.ExitCode)
		}
		style := v.s.dim
		if code != "0" {
			style = v.s.wait
		}
		return one(fmt.Sprintf("exit %s · %s", code, span(j.Took)), style)
	default:
		return one("pid gone, no end", v.s.wait)
	}
}

func (v *view) sessionCells(c cells, item *model.Item) {
	s := v.s
	sess := item.Session
	now := v.snap.Now
	kind := "session"
	switch {
	case sess.Other:
		kind = "other"
	case sess.Kind != events.Interactive:
		kind = sess.Kind
	}
	c["kind"] = one(kind, s.text)
	c["name"] = one(shortID(sess.ID), s.bold)
	c["tool"] = one(sess.Tool.Name, s.dim)
	modelName := sess.Model
	if modelName == "" && sess.Activity != nil {
		modelName = sess.Activity.Model
	}
	c["model"] = one(shortModel(modelName), s.dim)
	c["state"] = one(sessionState(sess, now, v.quiet), v.stateStyle(item))
	c["units"] = v.spawnedBar(sess)
	c["subagents"] = v.dots(sess.Subagents, sess.Open())
	c["job"] = one("—", s.dim)
	c["started"] = one(clock(sess.Started), s.dim)
	c["cond"] = v.conditionCell(sess.Condition)
	c["load"] = v.loadCell(sess.Load)
	elapsed := now.Sub(sess.Started)
	if !sess.Open() {
		elapsed = sess.Ended.Sub(sess.Started)
	}
	c["elapsed"] = one(span(elapsed), s.text)
	tokens := "—"
	switch {
	case sess.Activity != nil && sess.Activity.Tokens.Total() > 0:
		tokens = count(sess.Activity.Tokens.Total())
	case sess.Tokens != nil:
		tokens = count(sess.Tokens.Total())
	}
	c["tokens"] = one(tokens, s.info)
}

// sessionState says what a session is doing. With a transcript reader that is its turn: working
// (on which tool and file) or waiting for input; without one, only when its last event came.
func sessionState(s *model.Session, now time.Time, quiet time.Duration) string {
	if !s.Open() {
		return "ended " + ago(s.Ended, now)
	}
	a := s.Activity
	if s.Gone {
		return "gone, no end recorded · last activity " + ago(s.Last, now)
	}
	if p := s.Process; p != nil {
		switch p.Status {
		case claude.Waiting:
			what := p.WaitingFor
			if what == "" {
				what = "the person"
			}
			return "waiting on " + what + " · " + span(now.Sub(p.Since))
		case claude.Busy:
			since := p.Since
			if a != nil && a.Turn == transcript.Working && a.TurnSince.Before(since) {
				since = a.TurnSince
			}
			text := "working"
			if a != nil && a.Turn == transcript.Working {
				text += doing(a, " · ")
			}
			return text + " · " + span(now.Sub(since))
		case claude.Idle:
			since := p.Since
			if a != nil && a.Turn == transcript.Idle && a.TurnSince.Before(since) {
				since = a.TurnSince
			}
			return "waiting for input · " + span(now.Sub(since))
		}
	}
	switch s.Waiting {
	case events.WaitPermission:
		return "waiting on permission prompt · " + span(now.Sub(s.WaitingSince))
	case events.WaitInput:
		return "waiting for input · " + span(now.Sub(s.WaitingSince))
	}
	if a == nil {
		return "open · last event " + ago(s.Last, now)
	}
	switch {
	case a.Turn == transcript.Idle:
		return "waiting for input · " + span(now.Sub(a.TurnSince))
	case a.Turn == transcript.Working && now.Sub(a.Last) > quiet:
		return "silent " + span(now.Sub(a.Last)) + doing(a, " · last ")
	case a.Turn == transcript.Working:
		if a.Tool == "" {
			return "working · " + span(now.Sub(a.TurnSince))
		}
		return "working" + doing(a, " · ") + " · " + span(now.Sub(a.TurnSince))
	}
	return "open · last activity " + ago(a.Last, now)
}

// doing names the tool a session is running and the file it works on, after a prefix.
func doing(a *transcript.Activity, prefix string) string {
	if a.Tool == "" {
		return ""
	}
	text := prefix + a.Tool
	if a.Target != "" {
		text += " " + a.Target
	}
	return text
}

// spawnedBar draws the units an interactive session started: running ones, then ended ones.
func (v *view) spawnedBar(s *model.Session) []part {
	if len(s.Units) == 0 {
		return one("—", v.s.dim)
	}
	running := 0
	for _, u := range s.Units {
		if u.Open() {
			running++
		}
	}
	ended := len(s.Units) - running
	scale := func(n int) int {
		if len(s.Units) <= barCells {
			return n
		}
		if n == 0 {
			return 0
		}
		return max(1, n*barCells/len(s.Units))
	}
	return []part{
		{strings.Repeat(statusGlyphs[model.Working], scale(running)), v.s.info},
		{strings.Repeat(statusGlyphs["DONE"], scale(ended)), v.s.good},
		{fmt.Sprintf(" %d spawned", len(s.Units)), v.s.dim},
	}
}
