package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestGaugeShares checks that the share under way follows the share done with no blank cell
// between them, at any width, and that the gauge keeps its width.
func TestGaugeShares(t *testing.T) {
	v := &view{s: newStyles(ColorNever)}
	for width := 4; width <= 60; width++ {
		for _, tt := range []struct{ done, next float64 }{{0.5, 0.5}, {1.0 / 3, 1.0 / 3}, {0.2, 0.1}, {0, 0.5}, {0.9, 0.1}} {
			g := ansi.Strip(v.gauge2(tt.done, tt.next, width, v.s.info))
			if w := ansi.StringWidth(g); w != width {
				t.Errorf("width %d, %v: gauge is %d wide", width, tt, w)
			}
			solid := strings.TrimRight(strings.SplitN(g, "▒", 2)[0], "·")
			if strings.ContainsAny(solid, strings.Join(eighths[1:], "")+" ") {
				t.Errorf("width %d, %v: a part-filled cell before the share under way: %q", width, tt, g)
			}
		}
	}
}
