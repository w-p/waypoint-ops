package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// fit pads or cuts text to exactly width cells.
func fit(text string, width int, right bool) string {
	if width <= 0 {
		return ""
	}
	text = runewidth.Truncate(text, width, "…")
	if right {
		return runewidth.FillLeft(text, width)
	}
	return runewidth.FillRight(text, width)
}

// span formats a duration at the precision a person needs: seconds for short spans, days for
// long ones.
func span(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < 10*time.Minute:
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %02dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// count formats a token count: 530k, 5.2M, 123M.
func count(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%dk", n/1000)
	case n < 100_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	default:
		return fmt.Sprintf("%dM", n/1_000_000)
	}
}

// clock formats a time as month-day hour:minute in local time.
func clock(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("01-02 15:04")
}

// ago says how long before now a time was.
func ago(t, now time.Time) string {
	return span(now.Sub(t)) + " ago"
}

var modelVersion = regexp.MustCompile(`(\d+)-(\d+)$`)

// shortModel turns "claude-opus-5-5" into "opus-5.5".
func shortModel(name string) string {
	if name == "" {
		return "—"
	}
	name = strings.TrimPrefix(name, "claude-")
	return modelVersion.ReplaceAllString(name, "$1.$2")
}

// shortID is the first block of a session id.
func shortID(id string) string {
	if i := strings.IndexAny(id, "-"); i > 0 && !strings.HasPrefix(id, "agent-") {
		return id[:i]
	}
	return id
}

// plural writes "1 unit" or "3 units".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// bytes formats a memory size: 340M, 1.2G, 15G.
func bytes(n uint64) string {
	const mb, gb = 1 << 20, 1 << 30
	switch {
	case n < 10*mb:
		return fmt.Sprintf("%.1fM", float64(n)/mb)
	case n < gb:
		return fmt.Sprintf("%dM", n/mb)
	case n < 10*gb:
		return fmt.Sprintf("%.1fG", float64(n)/gb)
	default:
		return fmt.Sprintf("%dG", n/gb)
	}
}

// percent formats a CPU figure: 0%, 34%, 143%.
func percent(p float64) string {
	return fmt.Sprintf("%.0f%%", p)
}
