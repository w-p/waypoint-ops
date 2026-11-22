package ui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// pad cuts or pads a styled line to exactly width cells.
func pad(line string, width int) string {
	if width <= 0 {
		return ""
	}
	w := ansi.StringWidth(line)
	if w > width {
		return ansi.Truncate(line, width, "")
	}
	return line + strings.Repeat(" ", width-w)
}

// box draws a rounded frame of width by height cells around body, with a title set into the top
// edge at the left, a note at the right, and a footer set into the bottom edge.
func (v *view) box(title, note, footer string, width, height int, body []string) []string {
	s := v.s
	if width < 4 || height < 2 {
		return nil
	}
	inner := width - 2
	edge := func(left, right, label, labelRight string, labelStyle lipgloss.Style) string {
		var b strings.Builder
		b.WriteString(s.border.Render(left + "─"))
		used := 1
		if label != "" {
			b.WriteString(s.border.Render("┤") + labelStyle.Render(" "+label+" ") + s.border.Render("├"))
			used += ansi.StringWidth(label) + 4
		}
		tail := ""
		tailWidth := 0
		if labelRight != "" {
			tail = s.border.Render("┤") + s.dim.Render(" "+labelRight+" ") + s.border.Render("├─")
			tailWidth = ansi.StringWidth(labelRight) + 5
		}
		fill := inner - used - tailWidth
		if fill < 0 {
			tail, fill = "", max(0, inner-used)
		}
		b.WriteString(s.border.Render(strings.Repeat("─", fill)))
		b.WriteString(tail)
		b.WriteString(s.border.Render(right))
		return pad(b.String(), width)
	}
	out := []string{edge("╭", "╮", title, note, s.boxTitle)}
	side := s.border.Render("│")
	for i := 0; i < height-2; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		out = append(out, side+pad(line, inner)+side)
	}
	out = append(out, edge("╰", "╯", "", footer, s.dim))
	return out
}

// eighths are the partial blocks for smooth bars.
var eighths = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// gauge draws a bar width cells long, filled to frac, in eighths of a cell.
func (v *view) gauge(frac float64, width int, fill lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	frac = math.Max(0, math.Min(1, frac))
	steps := int(math.Round(frac * float64(width*8)))
	full, part := steps/8, steps%8
	var b strings.Builder
	b.WriteString(fill.Render(strings.Repeat("█", full) + eighths[part]))
	used := full
	if part > 0 {
		used++
	}
	b.WriteString(v.s.empty.Render(strings.Repeat("·", width-used)))
	return b.String()
}

// levels are the vertical eighths for bar charts.
var levels = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// bars draws values as a vertical bar chart, each bar wide cells across and height rows tall,
// scaled so top fills the chart. Rows are colored along the gradient, low to high. Anything
// above zero shows at least a sliver.
func (v *view) bars(values []float64, top float64, wide, height int) []string {
	if top <= 0 {
		top = 1
	}
	steps := height * 8
	level := make([]int, len(values))
	for i, x := range values {
		if x > 0 {
			level[i] = min(steps, max(1, int(math.Round(x/top*float64(steps)))))
		}
	}
	lines := make([]string, height)
	for row := 0; row < height; row++ {
		base := (height - 1 - row) * 8
		t := float64(height-1-row) / math.Max(1, float64(height-1))
		ink := v.s.along(t)
		var b strings.Builder
		for _, l := range level {
			n := min(8, max(0, l-base))
			switch {
			case n > 0:
				b.WriteString(ink.Render(strings.Repeat(levels[n], wide)))
			case row == height-1:
				b.WriteString(v.s.empty.Render(strings.Repeat("▁", wide)))
			default:
				b.WriteString(strings.Repeat(" ", wide))
			}
		}
		lines[row] = b.String()
	}
	return lines
}

// spinner frames for live rows.
var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (v *view) spin() string {
	return spinner[int(v.phase*10)%len(spinner)]
}
