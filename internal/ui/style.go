package ui

import (
	"math"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// styles holds every style the screen uses. Color is decided here and nowhere else, so turning
// it off turns all of it off. Meaning never rests on color alone: flags carry a "!" mark, unit
// bars use a different glyph per status, and states are spelled out.
type styles struct {
	// color is false when color is off; rich is true when the terminal shows 256 colors or more,
	// which the row tints and gradients need.
	color bool
	rich  bool

	text    lipgloss.Style
	dim     lipgloss.Style
	bold    lipgloss.Style
	flag    lipgloss.Style
	bad     lipgloss.Style
	wait    lipgloss.Style
	good    lipgloss.Style
	info    lipgloss.Style
	sub     lipgloss.Style
	project lipgloss.Style

	border      lipgloss.Style
	boxTitle    lipgloss.Style
	headBar     lipgloss.Style
	selectd     lipgloss.Style
	keyKey      lipgloss.Style
	keyLabel    lipgloss.Style
	empty       lipgloss.Style
	flagTint    lipgloss.Color
	attnTint    lipgloss.Color
	attn        lipgloss.Style
	gradient    []rgb
	newStyleFor func() lipgloss.Style
}

// Color modes, as in the settings file.
const (
	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"
)

// The palette: a dark slate with soft accents.
const (
	cText   = "#c0caf5"
	cDim    = "#737aa2"
	cBorder = "#4a5274"
	cTitle  = "#7aa2f7"
	cGood   = "#9ece6a"
	cWait   = "#e0af68"
	cBad    = "#f7768e"
	cInfo   = "#7dcfff"
	cSub    = "#bb9af7"
	cProj   = "#2ac3de"
	cBar    = "#292e42"
	cSelect = "#7dcfff"
	cInk    = "#1a1b26"
	cTint   = "#3a2130"
	cAttn   = "#3a3220"
)

// newStyles builds the styles for a color mode. With color off every style is empty and renders
// text unchanged, except the selected row, which uses reverse video.
func newStyles(mode string) styles {
	r := lipgloss.DefaultRenderer()
	if mode == ColorNever {
		r.SetColorProfile(termenv.Ascii)
		plain := r.NewStyle
		return styles{
			text: plain(), dim: plain(), bold: plain(), flag: plain(), bad: plain(), wait: plain(),
			good: plain(), info: plain(), sub: plain(), project: plain(), border: plain(), attn: plain(),
			boxTitle: plain(), headBar: plain(), empty: plain(),
			selectd:     plain().Reverse(true),
			keyKey:      plain(),
			keyLabel:    plain().Reverse(true),
			newStyleFor: plain,
		}
	}
	if mode == ColorAlways {
		r.SetColorProfile(termenv.TrueColor)
	}
	rich := r.ColorProfile() == termenv.TrueColor || r.ColorProfile() == termenv.ANSI256
	style := r.NewStyle
	fg := func(c string) lipgloss.Style { return style().Foreground(lipgloss.Color(c)) }
	s := styles{
		color:       true,
		rich:        rich,
		text:        fg(cText),
		dim:         fg(cDim),
		bold:        fg(cText).Bold(true),
		flag:        fg(cBad).Bold(true),
		bad:         fg(cBad),
		wait:        fg(cWait),
		good:        fg(cGood),
		info:        fg(cInfo),
		sub:         fg(cSub),
		project:     fg(cProj),
		border:      fg(cBorder),
		boxTitle:    fg(cTitle).Bold(true),
		headBar:     fg(cText).Background(lipgloss.Color(cBar)).Bold(true),
		selectd:     style().Foreground(lipgloss.Color(cInk)).Background(lipgloss.Color(cSelect)).Bold(true),
		keyKey:      fg(cText).Bold(true),
		keyLabel:    style().Foreground(lipgloss.Color(cInk)).Background(lipgloss.Color(cSelect)),
		empty:       fg(cBorder),
		flagTint:    lipgloss.Color(cTint),
		attnTint:    lipgloss.Color(cAttn),
		attn:        fg(cWait).Bold(true),
		gradient:    []rgb{hex(cProj), hex(cTitle), hex(cSub)},
		newStyleFor: style,
	}
	if !rich {
		// Sixteen colors cannot tint a row without drowning it, and bars read better without
		// a background.
		s.headBar = style().Bold(true).Underline(true)
		s.flagTint = ""
		s.attnTint = ""
	}
	return s
}

// rgb is a color for blending.
type rgb struct{ r, g, b float64 }

func hex(s string) rgb {
	var c [3]float64
	for i := 0; i < 3; i++ {
		var v int
		for _, ch := range s[1+2*i : 3+2*i] {
			v *= 16
			switch {
			case ch >= '0' && ch <= '9':
				v += int(ch - '0')
			case ch >= 'a' && ch <= 'f':
				v += int(ch-'a') + 10
			}
		}
		c[i] = float64(v)
	}
	return rgb{c[0], c[1], c[2]}
}

func (c rgb) String() string {
	const digits = "0123456789abcdef"
	out := []byte{'#'}
	for _, v := range []float64{c.r, c.g, c.b} {
		n := int(math.Round(math.Max(0, math.Min(255, v))))
		out = append(out, digits[n/16], digits[n%16])
	}
	return string(out)
}

// blend mixes two colors: t=0 gives a, t=1 gives b.
func blend(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// along picks a color along the gradient at t in [0, 1].
func (s styles) along(t float64) lipgloss.Style {
	if !s.color || len(s.gradient) == 0 {
		return s.text
	}
	t = math.Max(0, math.Min(1, t))
	n := float64(len(s.gradient) - 1)
	i := int(math.Min(t*n, n-1))
	c := blend(s.gradient[i], s.gradient[i+1], t*n-float64(i))
	return s.newStyleFor().Foreground(lipgloss.Color(c.String()))
}

// pulse is the flag color breathing between full and half brightness; phase is in seconds.
// Attention flags breathe in amber, problems in red.
func (s styles) pulse(phase float64, attention bool) lipgloss.Style {
	if !s.rich {
		if attention {
			return s.attn
		}
		return s.flag
	}
	t := 0.5 + 0.5*math.Cos(phase*2*math.Pi/1.6)
	c := blend(hex("#6b2a3a"), hex(cBad), t)
	if attention {
		c = blend(hex("#6b5328"), hex(cWait), t)
	}
	return s.newStyleFor().Foreground(lipgloss.Color(c.String())).Bold(true)
}
