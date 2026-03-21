package tui

import (
	"math"
	"math/rand/v2"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// introCards are the credit cards that fly past, Superman-style.
var introCards = []string{
	"A TICKET SYSTEM",
	"tk",
	"EPICS",
	"STORIES  ·  BUGS  ·  TASKS",
	"REQUIREMENTS",
	"DECISIONS",
	"YOUR JOURNEY STARTS NOW",
}

const (
	numStars           = 150
	starfieldLeadIn    = 0.4  // seconds of pure star field before first card
	cardDuration       = 1.5  // seconds per card
	introBg            = "#00000e" // deep space blue-black
)

// star3D is a point in 3D space moving toward the viewer.
type star3D struct {
	x, y float64 // world coords (normalised: ±1 maps to screen edge at z=1)
	z    float64 // depth: larger = farther; decreases over time
	br   float64 // base brightness [0,1]
}

func newStarField() []star3D {
	stars := make([]star3D, numStars)
	for i := range stars {
		stars[i] = randomStar()
		// Scatter initial z so stars don't all arrive at once
		stars[i].z = rand.Float64()*4.5 + 0.3
	}
	return stars
}

func randomStar() star3D {
	return star3D{
		x:  (rand.Float64()*2 - 1),
		y:  (rand.Float64()*2 - 1),
		z:  4.0 + rand.Float64()*1.5,
		br: rand.Float64()*0.5 + 0.5,
	}
}

// advance moves a star toward the viewer and resets it when it passes.
func (s *star3D) advance(dt float64) {
	s.z -= dt * 1.4
	if s.z <= 0.05 {
		*s = randomStar()
	}
}

// project returns screen (x, y) and brightness for a w×h terminal.
func (s star3D) project(w, h int) (sx, sy int, brightness float64) {
	if s.z < 0.01 {
		return -1, -1, 0
	}
	cx, cy := float64(w)*0.5, float64(h)*0.5
	// Perspective: star at (x,y,z=1) maps to edge of screen
	px := cx + s.x*cx/s.z
	py := cy + s.y*cy/s.z
	brightness = math.Min(1.0, s.br*0.6/s.z)
	return int(px), int(py), brightness
}

// starRune picks a character by brightness.
func starRune(brightness float64) rune {
	switch {
	case brightness > 0.85:
		return '✦'
	case brightness > 0.6:
		return '*'
	case brightness > 0.35:
		return '+'
	default:
		return '·'
	}
}

// starColor returns a lipgloss colour by brightness.
func starColor(brightness float64) lipgloss.Color {
	b := int(brightness * 255)
	if b > 255 {
		b = 255
	}
	// Cool blue-white gradient
	r := b * 6 / 10
	g := b * 7 / 10
	return lipgloss.Color(rgbHex(r, g, b))
}

// ─── intro state ─────────────────────────────────────────────────────────────

type introState struct {
	stars   []star3D
	elapsed float64 // total seconds since intro started
}

func newIntroState() introState {
	return introState{stars: newStarField()}
}

// done returns true when all cards have played.
func (s *introState) done() bool {
	totalDuration := starfieldLeadIn + float64(len(introCards))*cardDuration + 0.3
	return s.elapsed >= totalDuration
}

// advance moves the intro forward by dt seconds.
func (s *introState) advance(dt float64) {
	s.elapsed += dt
	for i := range s.stars {
		s.stars[i].advance(dt)
	}
}

// currentCard returns which card is active and its local t [0,1].
// cardIdx == -1 means we're in the star-field lead-in.
func (s *introState) currentCard() (cardIdx int, localT float64) {
	if s.elapsed < starfieldLeadIn {
		return -1, 0
	}
	offset := s.elapsed - starfieldLeadIn
	idx := int(offset / cardDuration)
	if idx >= len(introCards) {
		return len(introCards), 1 // past all cards
	}
	lt := math.Mod(offset, cardDuration) / cardDuration
	return idx, lt
}

// ─── rendering ───────────────────────────────────────────────────────────────

type introCell struct {
	ch    rune
	color lipgloss.Color
	bold  bool
}

// renderIntro builds the full w×h intro frame as a []string.
func renderIntro(s *introState, w, h int) []string {
	// Build cell grid
	grid := make([][]introCell, h)
	bg := lipgloss.Color(introBg)
	for y := range grid {
		grid[y] = make([]introCell, w)
		for x := range grid[y] {
			grid[y][x] = introCell{' ', bg, false}
		}
	}

	// Draw stars
	for _, star := range s.stars {
		sx, sy, brightness := star.project(w, h)
		if sx >= 0 && sx < w && sy >= 0 && sy < h && brightness > 0 {
			ch := starRune(brightness)
			color := starColor(brightness)
			bold := brightness > 0.85
			// Only overwrite if brighter
			if grid[sy][sx].ch == ' ' || brightness > 0.5 {
				grid[sy][sx] = introCell{ch, color, bold}
			}
		}
	}

	// Draw active text card
	cardIdx, localT := s.currentCard()
	if cardIdx >= 0 && cardIdx < len(introCards) {
		text := introCards[cardIdx]
		// cardZ: approaches from 2.0 → 0 as localT goes 0 → 0.56
		//        then goes negative (past) as localT → 1
		cardZ := 2.0 - localT*3.6
		if cardZ > -0.5 {
			drawCard(grid, text, cardZ, w, h)
		}
	}

	// Convert grid to strings
	bgStyle := lipgloss.NewStyle().Background(bg)
	lines := make([]string, h)
	for y, row := range grid {
		var sb strings.Builder
		sb.WriteString(bgStyle.Render("")) // set bg
		for _, c := range row {
			style := lipgloss.NewStyle().
				Foreground(c.color).
				Background(bg)
			if c.bold {
				style = style.Bold(true)
			}
			sb.WriteString(style.Render(string(c.ch)))
		}
		lines[y] = sb.String()
	}
	return lines
}

// drawCard places the zooming text onto the grid using perspective spread.
func drawCard(grid [][]introCell, text string, cardZ float64, w, h int) {
	chars := []rune(strings.ToUpper(text))
	if len(chars) == 0 {
		return
	}

	// Spread: how many blank columns between characters
	var spread int
	if cardZ > 0.05 {
		spread = int(0.9 / cardZ)
	} else {
		spread = int(0.9 / 0.05) // cap
	}
	if spread > w/2 {
		spread = w / 2
	}

	totalWidth := len(chars) + (len(chars)-1)*spread
	cx := w / 2
	cy := h / 2

	// Vertical offset: slight upward drift as text approaches
	vy := int(float64(h) * 0.04 * (1.0 - math.Abs(cardZ)/2.0))

	startX := cx - totalWidth/2
	textY := cy - vy

	// Colour based on cardZ (closer = brighter, whiter)
	color, bold := cardColor(cardZ)

	for i, ch := range chars {
		x := startX + i*(spread+1)
		if x < 0 || x >= w {
			continue
		}
		if textY < 0 || textY >= h {
			continue
		}
		grid[textY][x] = introCell{ch, color, bold}
	}
}

// cardColor returns (color, bold) based on card depth.
func cardColor(cardZ float64) (lipgloss.Color, bool) {
	dist := math.Abs(cardZ)
	switch {
	case dist > 1.5:
		return lipgloss.Color("#1a3366"), false // very far, barely visible
	case dist > 1.0:
		return lipgloss.Color("#2255aa"), false
	case dist > 0.6:
		return lipgloss.Color("#4488dd"), false
	case dist > 0.3:
		return lipgloss.Color("#88bbff"), true
	case dist > 0.1:
		return lipgloss.Color("#ccddff"), true
	default:
		return lipgloss.Color("#ffffff"), true // peak brightness
	}
}

// rgbHex formats an RGB colour as a lipgloss-compatible hex string.
func rgbHex(r, g, b int) string {
	hexChars := "0123456789abcdef"
	return string([]byte{
		'#',
		hexChars[r>>4], hexChars[r&0xf],
		hexChars[g>>4], hexChars[g&0xf],
		hexChars[b>>4], hexChars[b&0xf],
	})
}
