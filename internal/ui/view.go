package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/w-p/waypoint-ops/internal/model"
)

// view draws one frame from a snapshot. It holds no state between frames: anything that moves
// over time comes in through phase and anim.
type view struct {
	s       styles
	snap    model.Snapshot
	width   int
	height  int
	focus   string
	recent  bool
	cursor  int
	refresh string

	// window is how long finished work stays in view; quiet is how long an open session may be
	// silent before it reads as quiet.
	window time.Duration
	quiet  time.Duration

	// phase is seconds since the app started, for the spinner and the flag's pulse.
	phase float64

	// anim returns the animated value for key as it moves towards target.
	anim func(key string, target float64) float64
}

// rows are the selectable board rows in order: flagged and unfinished work, then finished work
// when the recent section is shown.
func rows(snap model.Snapshot, focus string, recent bool) (items, done []*model.Item) {
	for _, item := range snap.Items {
		if focus == "" || item.Project == focus {
			items = append(items, item)
		}
	}
	if !recent {
		return items, nil
	}
	for _, item := range snap.Recent {
		if focus == "" || item.Project == focus {
			done = append(done, item)
		}
	}
	return items, done
}

// hiddenRecent counts finished items in scope, shown or not.
func (v *view) hiddenRecent() int {
	n := 0
	for _, item := range v.snap.Recent {
		if v.focus == "" || item.Project == v.focus {
			n++
		}
	}
	return n
}

const headerHeight = 7

func (v *view) render() string {
	items, done := rows(v.snap, v.focus, v.recent)
	all := append(append([]*model.Item{}, items...), done...)
	var selected *model.Item
	if v.cursor >= 0 && v.cursor < len(all) {
		selected = all[v.cursor]
	}

	var top []string
	if v.height >= 30 && v.width >= 100 {
		top = v.header(items, done)
	} else {
		top = []string{v.compactHeader(items)}
	}

	bottom := v.problems()
	bottom = append(bottom, v.keys())

	// The board gets the room its rows need, up to three fifths of the screen; the detail panel
	// gets what is left, up to its own length; any slack goes back to the board.
	rest := v.height - len(top) - len(bottom)
	detailBody := v.detail(selected)
	workNeed := len(items) + len(done) + 6
	if len(items) == 0 {
		workNeed++
	}
	detailHeight := min(len(detailBody)+2, max(5, rest-min(workNeed, rest*3/5)))
	workHeight := max(5, rest-detailHeight)

	lines := top
	lines = append(lines, v.work(items, done, workHeight)...)
	lines = append(lines, v.box(v.detailTitle(selected), "", "", v.width, detailHeight, detailBody)...)
	lines = append(lines, bottom...)
	if len(lines) > v.height {
		lines = lines[:v.height]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, v.width, "")
	}
	return strings.Join(lines, "\n")
}

// header is the three panels across the top: status, run progress and tokens.
func (v *view) header(items, done []*model.Item) []string {
	statusWidth := 44
	// The graph shows the last 24 hours as one bar per hour, two cells wide, or one cell wide
	// when the screen is narrower.
	wide := 0
	switch {
	case v.width >= 170:
		wide = 2
	case v.width >= 130:
		wide = 1
	}
	tokensWidth := 0
	if wide > 0 {
		tokensWidth = 24*wide + 4
	}
	runsWidth := v.width - statusWidth - tokensWidth
	panels := [][]string{
		v.statusPanel(items, statusWidth),
		v.runsPanel(items, done, runsWidth),
	}
	if tokensWidth > 0 {
		panels = append(panels, v.tokensPanel(tokensWidth, wide))
	}
	out := make([]string, headerHeight)
	for i := range out {
		for _, p := range panels {
			out[i] += p[i]
		}
	}
	return out
}

func (v *view) compactHeader(items []*model.Item) string {
	n := map[model.Category]int{}
	for _, item := range items {
		n[item.Category]++
	}
	scope := "all projects"
	if v.focus != "" {
		scope = v.focus
	}
	return v.s.boxTitle.Render(" waypoint-ops ") + v.s.dim.Render(scope+" · ") +
		v.count(n[model.Flagged], "flagged", v.s.flag) + v.s.dim.Render(" · ") +
		v.count(n[model.Active], "active", v.s.good) + v.s.dim.Render(" · ") +
		v.count(n[model.Waiting], "waiting", v.s.wait) + v.s.dim.Render(" · ") +
		v.count(n[model.Quiet], "quiet", v.s.text)
}

func (v *view) count(n int, word string, style lipgloss.Style) string {
	if n == 0 {
		style = v.s.dim
	}
	return style.Render(fmt.Sprintf("%d %s", n, word))
}

// statusPanel holds the verdict counts on the left and the machine's on the right.
func (v *view) statusPanel(items []*model.Item, width int) []string {
	s := v.s
	n := map[model.Category]int{}
	for _, item := range items {
		n[item.Category]++
	}
	m := v.snap.Machine
	num := func(value int, style lipgloss.Style) string {
		if value == 0 {
			style = s.dim
		}
		return style.Render(fmt.Sprintf("%3d", value))
	}
	left := []string{
		s.dim.Render(" flagged ") + num(n[model.Flagged], s.flag),
		s.dim.Render(" active  ") + num(n[model.Active], s.good),
		s.dim.Render(" waiting ") + num(n[model.Waiting], s.wait),
		s.dim.Render(" quiet   ") + num(n[model.Quiet], s.text),
	}
	right := []string{
		s.dim.Render("sessions  ") + num(m.SessionsOpen, s.text) + s.dim.Render(" open"),
		s.dim.Render("subagents ") + num(m.SubagentsLive, s.sub) + s.dim.Render(" live"),
		s.dim.Render("jobs      ") + num(m.JobsRunning, s.good) + s.dim.Render(" running"),
		s.dim.Render("runners   ") + num(m.RunnersAlive, s.text) + s.dim.Render(fmt.Sprintf(" of %d alive", m.RunsUnfinished)),
	}
	var body []string
	for i := range left {
		body = append(body, pad(left[i], 17)+right[i])
	}
	projects := plural(len(v.snap.Projects), "project")
	body = append(body, s.dim.Render(fmt.Sprintf(" %s · %d finished in %s", projects, v.hiddenRecent(), window(v.window))))
	scope := "all projects"
	if v.focus != "" {
		scope = "focus " + v.focus
	}
	return v.box("waypoint-ops", scope, "", width, headerHeight, body)
}

// runsPanel shows each run in view as a progress gauge through its units.
func (v *view) runsPanel(items, done []*model.Item, width int) []string {
	s := v.s
	var runs []*model.Item
	for _, item := range append(append([]*model.Item{}, items...), done...) {
		if item.Run != nil {
			runs = append(runs, item)
		}
	}
	inner := width - 2
	nameWidth := 16
	projectWidth := 0
	if v.focus == "" {
		projectWidth = 11
	}
	gaugeWidth := inner - 1 - projectWidth - nameWidth - 1 - 1 - 6 - 1 - 9
	var body []string
	for i, item := range runs {
		if i == headerHeight-2 {
			break
		}
		r := item.Run
		finished := 0
		for _, u := range r.Units {
			if u.Status == "DONE" {
				finished++
			}
		}
		frac := 0.0
		if len(r.Units) > 0 {
			frac = float64(finished) / float64(len(r.Units))
		}
		style, word := v.runWord(item)
		line := " "
		if projectWidth > 0 {
			line += s.project.Render(fit(r.Project, projectWidth-1, false)) + " "
		}
		line += s.bold.Render(fit(r.Short(), nameWidth, false)) + " "
		line += v.gauge(v.anim("gauge/"+item.Key(), frac), max(4, gaugeWidth), style) + " "
		line += s.dim.Render(fit(fmt.Sprintf("%d/%d", finished, len(r.Units)), 6, true)) + " "
		line += style.Render(fit(word, 9, false))
		body = append(body, line)
	}
	if len(runs) == 0 {
		body = append(body, s.dim.Render(fmt.Sprintf(" no runs in the last %s", window(v.window))))
	}
	note := ""
	if len(runs) > headerHeight-2 {
		note = fmt.Sprintf("%d more below", len(runs)-(headerHeight-2))
	}
	return v.box("runs", note, "", width, headerHeight, body)
}

// runWord is a run's state in one word, with its color.
func (v *view) runWord(item *model.Item) (lipgloss.Style, string) {
	r := item.Run
	switch r.State {
	case model.Died:
		return v.s.bad, "died"
	case model.Finished:
		if item.Flag != "" {
			return v.s.bad, r.Outcome
		}
		return v.s.good, r.Outcome
	case model.Paused:
		return v.s.wait, "waiting"
	case model.Running:
		if item.Flag != "" {
			return v.s.bad, "running"
		}
		return v.s.info, "running"
	}
	return v.s.info, "starting"
}

// tokensPanel graphs tokens reported over the last 24 hours, one bar per hour.
func (v *view) tokensPanel(width, wide int) []string {
	s := v.s
	m := v.snap.Machine
	const buckets = 24
	graphWidth := buckets * wide
	size := time.Hour
	// Align buckets to the clock so the graph steps rather than jitters as time passes.
	end := v.snap.Now.Truncate(size).Add(size)
	start := end.Add(-time.Duration(buckets) * size)
	values := make([]float64, buckets)
	var total int64
	for _, sp := range m.Spend {
		i := int(sp.At.Sub(start) / size)
		if i >= 0 && i < buckets {
			values[i] += float64(sp.Tokens)
		}
		total += sp.Tokens
	}
	top := 0.0
	for _, x := range values {
		top = max(top, x)
	}
	boot := v.anim("graph/boot", 1)
	for i := range values {
		values[i] *= boot
	}
	var body []string
	for _, line := range v.bars(values, top, wide, 3) {
		body = append(body, " "+line)
	}
	axis := make([]byte, graphWidth)
	for i := range axis {
		axis[i] = ' '
	}
	label := func(at int, text string) {
		for i := 0; i < len(text) && at+i < graphWidth; i++ {
			if at+i >= 0 {
				axis[at+i] = text[i]
			}
		}
	}
	label(0, "-24h")
	label(graphWidth/2-2, "-12h")
	label(graphWidth-3, "now")
	body = append(body, " "+s.dim.Render(string(axis)))

	var tools []string
	for name := range m.TokensToday {
		tools = append(tools, name)
	}
	sort.Strings(tools)
	today := s.dim.Render(" today ")
	if len(tools) == 0 {
		today += s.dim.Render("—")
	}
	for i, name := range tools {
		if i > 0 {
			today += s.dim.Render(" · ")
		}
		today += s.info.Render(count(m.TokensToday[name])) + s.dim.Render(" "+name)
	}
	body = append(body, today)
	note := ""
	if top > 0 {
		note = fmt.Sprintf("peak %s in an hour", count(int64(top)))
	}
	if total == 0 {
		note = "none reported"
	}
	return v.box("tokens · 24h", note, "", width, headerHeight, body)
}

// work is the main panel: the board, then finished work below a divider.
func (v *view) work(items, done []*model.Item, height int) []string {
	inner := v.width - 2
	cols := layout(v.focus, inner)
	room := height - 3 // borders and the column header

	type line struct {
		text string
		row  int // index into the selectable rows, or -1
	}
	var lines []line
	for i, item := range items {
		lines = append(lines, line{v.row(item, cols, inner, i == v.cursor, false), i})
	}
	if len(items) == 0 {
		lines = append(lines, line{v.s.dim.Render("   nothing running"), -1})
	}
	lines = append(lines, line{"", -1})
	if v.recent {
		label := fmt.Sprintf("recent · last %s · %s", window(v.window), plural(len(done), "finished item"))
		lines = append(lines, line{v.divider(label, inner), -1})
	} else {
		label := fmt.Sprintf("recent · %d finished, hidden", v.hiddenRecent())
		lines = append(lines, line{v.divider(label, inner), -1})
	}
	for i, item := range done {
		n := len(items) + i
		lines = append(lines, line{v.row(item, cols, inner, n == v.cursor, true), n})
	}

	start := 0
	if room < 1 {
		room = 1
	}
	if len(lines) > room {
		at := 0
		for i, l := range lines {
			if l.row == v.cursor {
				at = i
			}
		}
		start = min(max(0, at-room/2), len(lines)-room)
	}
	body := []string{v.headerRow(cols, inner)}
	for _, l := range lines[start:min(len(lines), start+room)] {
		body = append(body, l.text)
	}

	title := "work · all projects"
	if v.focus != "" {
		title = "work · " + v.focus
	}
	note := fmt.Sprintf("%d live · %d recent", len(items), v.hiddenRecent())
	footer := ""
	if len(lines) > room {
		footer = fmt.Sprintf("%d–%d of %d", start+1, min(len(lines), start+room), len(lines))
	}
	return v.box(title, note, footer, v.width, height, body)
}

// divider is a labelled rule across the board.
func (v *view) divider(label string, width int) string {
	text := " " + label + " "
	return v.s.dim.Render(" ─"+text) + v.s.border.Render(strings.Repeat("─", max(0, width-ansi.StringWidth(text)-3)))
}

func (v *view) problems() []string {
	var all []string
	for _, p := range v.snap.Projects {
		all = append(all, p.Problems...)
	}
	if len(all) == 0 {
		return nil
	}
	text := fmt.Sprintf(" ▲ %s: %s", plural(len(all), "problem"), all[0])
	if len(all) > 1 {
		text += fmt.Sprintf(" (and %d more)", len(all)-1)
	}
	return []string{v.s.wait.Render(text)}
}

// keys is the bar along the bottom, in the manner of htop: each key, then what it does.
func (v *view) keys() string {
	type key struct{ key, label string }
	list := []key{{"q", "Quit"}, {"↑↓", "Move"}, {"f", "Focus"}}
	if v.focus != "" {
		list = append(list, key{"esc", "All"})
	}
	recent := "Hide done"
	if !v.recent {
		recent = "Show done"
	}
	list = append(list, key{"r", recent})
	var b strings.Builder
	for _, k := range list {
		b.WriteString(v.s.keyKey.Render(" " + k.key + " "))
		b.WriteString(v.s.keyLabel.Render(fit(" "+k.label, max(8, len(k.label)+2), false)))
	}
	left := b.String()
	right := v.s.dim.Render("every "+v.refresh+" · ") + v.s.text.Render(v.snap.Now.Local().Format("Mon 15:04:05")) + " "
	gap := v.width - ansi.StringWidth(left) - ansi.StringWidth(right)
	return left + strings.Repeat(" ", max(1, gap)) + right
}

// window formats the recent window in whole hours or days: "48h", "7d".
func window(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 72 && hours%24 == 0 {
		return fmt.Sprintf("%dd", hours/24)
	}
	return fmt.Sprintf("%dh", hours)
}
