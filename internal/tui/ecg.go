package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ecgParams holds per-instance ECG animation tuning.
type ecgParams struct {
	speedFactor float64 // multiplier on base speed (1.0 = perimeter in ~3s)
	trailLen    int     // chars of trailing memory glow behind peak
	peakWidth   int     // chars of bright QRS peak
	pWave       int     // chars of anticipation glow ahead of peak
}

var defaultECGParams = ecgParams{
	speedFactor: 1.0,
	trailLen:    28,
	peakWidth:   3,
	pWave:       4,
}

// ecgState tracks the animated ECG pulse on the screen border.
type ecgState struct {
	pos    float64 // position on perimeter [0, perimeter)
	t      float64 // elapsed time (seconds) for noise
	params ecgParams
}

const ecgDT = 0.05 // seconds per tick

// advance moves the pulse forward using smooth noise for velocity variation.
func (e *ecgState) advance(perimeter int) {
	p := e.params
	if p.speedFactor == 0 {
		p = defaultECGParams
	}
	// Perlin-like smooth noise via summed sinusoids
	noise := 0.5*math.Sin(e.t*0.7+1.3) +
		0.3*math.Sin(e.t*1.7+2.1) +
		0.2*math.Sin(e.t*3.1+0.5)
	// Base speed: traverse full perimeter in ~3s; noise ±40%
	baseVel := float64(perimeter) / 3.0 * p.speedFactor
	vel := baseVel * (1.0 + 0.4*noise)
	e.pos = math.Mod(e.pos+vel*ecgDT, float64(perimeter))
	e.t += ecgDT
}

// intensity returns 0.0–1.0 for a perimeter index based on ECG waveform shape.
func (e *ecgState) intensity(i, perimeter int) float64 {
	p := e.params
	if p.speedFactor == 0 {
		p = defaultECGParams
	}
	// "behind" = how far behind the pulse front index i is (0 = at front)
	behind := int(math.Round(e.pos)) - i
	if behind < 0 {
		behind += perimeter
	}
	ahead := perimeter - behind // distance ahead of pulse

	switch {
	case behind < p.peakWidth:
		// QRS complex: full brightness, sharper at tip
		return 1.0 - float64(behind)*0.08
	case behind < p.peakWidth+4:
		// T-wave: gentler bump just after peak
		d := float64(behind - p.peakWidth)
		return 0.55 - d*0.05
	case behind < p.trailLen:
		// Trailing memory glow: exponential decay
		return 0.45 * math.Exp(-float64(behind-p.peakWidth-4)/float64(p.trailLen/4))
	case ahead > 0 && ahead <= p.pWave:
		// Anticipation glow ahead of pulse
		return 0.15 + float64(p.pWave-ahead)*0.04
	default:
		return 0
	}
}

// perimIndex converts a border (x, y) to a perimeter index.
// Returns -1 if (x,y) is not on the border.
func perimIndex(x, y, w, h int) int {
	switch {
	case y == 0:
		return x
	case x == w-1:
		return w + (y - 1)
	case y == h-1:
		return w + (h - 1) + (w - 1 - x)
	case x == 0:
		return w + (h - 1) + (w - 1) + (h - 1 - y)
	}
	return -1
}

// perimLen returns the total number of border characters for a w×h screen.
func perimLen(w, h int) int {
	return 2*w + 2*h - 4
}

// borderChar returns the box-drawing character for a border position.
func borderChar(x, y, w, h int) string {
	switch {
	case x == 0 && y == 0:
		return "╭"
	case x == w-1 && y == 0:
		return "╮"
	case x == 0 && y == h-1:
		return "╰"
	case x == w-1 && y == h-1:
		return "╯"
	case y == 0 || y == h-1:
		return "─"
	default:
		return "│"
	}
}

// renderBorder returns the W×H screen as a slice of lines (strings), drawing
// the ECG-animated border. content is a slice of (h-2) inner strings, each
// already exactly (w-2) runes wide (padding is caller's responsibility).
func renderBorder(e *ecgState, t Theme, w, h int, content []string) string {
	P := perimLen(w, h)
	lines := make([]string, h)

	// Top border line
	{
		var sb strings.Builder
		for x := 0; x < w; x++ {
			idx := perimIndex(x, 0, w, h)
			intensity := e.intensity(idx, P)
			ch := borderChar(x, 0, w, h)
			color := t.ecgColor(intensity)
			style := lipgloss.NewStyle().Foreground(color)
			if intensity > 0.8 {
				style = style.Bold(true)
			}
			sb.WriteString(style.Render(ch))
		}
		lines[0] = sb.String()
	}

	// Middle lines: left border │ content │ right border
	for row := 1; row < h-1; row++ {
		var sb strings.Builder
		// Left border
		{
			idx := perimIndex(0, row, w, h)
			intensity := e.intensity(idx, P)
			color := t.ecgColor(intensity)
			style := lipgloss.NewStyle().Foreground(color)
			if intensity > 0.8 {
				style = style.Bold(true)
			}
			sb.WriteString(style.Render("│"))
		}
		// Content
		if row-1 < len(content) {
			sb.WriteString(content[row-1])
		} else {
			sb.WriteString(strings.Repeat(" ", w-2))
		}
		// Right border
		{
			idx := perimIndex(w-1, row, w, h)
			intensity := e.intensity(idx, P)
			color := t.ecgColor(intensity)
			style := lipgloss.NewStyle().Foreground(color)
			if intensity > 0.8 {
				style = style.Bold(true)
			}
			sb.WriteString(style.Render("│"))
		}
		lines[row] = sb.String()
	}

	// Bottom border line
	{
		var sb strings.Builder
		for x := 0; x < w; x++ {
			idx := perimIndex(x, h-1, w, h)
			intensity := e.intensity(idx, P)
			ch := borderChar(x, h-1, w, h)
			color := t.ecgColor(intensity)
			style := lipgloss.NewStyle().Foreground(color)
			if intensity > 0.8 {
				style = style.Bold(true)
			}
			sb.WriteString(style.Render(ch))
		}
		lines[h-1] = sb.String()
	}

	return strings.Join(lines, "\n")
}

// renderStaticBorder draws a plain (non-animated) border.
func renderStaticBorder(t Theme, w, h int, content []string) string {
	e := &ecgState{pos: -1e9} // all intensities → 0
	return renderBorder(e, t, w, h, content)
}
