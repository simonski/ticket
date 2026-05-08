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
	numStars        = 150
	starfieldLeadIn = 0.5  // seconds of pure star field before first card
	cardDuration    = 5.0  // seconds per card
	cardCrossT      = 0.55 // fraction of cardDuration when card reaches the viewer
	ghostFadeTime   = 1.0  // seconds for the chroma ghost to fade
	maxApproachZ    = 2.5  // Z value where the card starts (far away)
	flyOutFactor    = 0.5  // how fast spread grows in the fly-past phase
	introBg         = "#00000e"
)

// ─── perlin-like noise ────────────────────────────────────────────────────────

// perlin1D returns smooth noise in [-1,1] using 4-octave sinusoids with
// golden-ratio frequency spacing to avoid harmonic repetition.
func perlin1D(x float64) float64 {
	const phi = 1.6180339887498949
	v := 0.500*math.Sin(x*1.000+0.37) +
		0.250*math.Sin(x*phi+1.23) +
		0.125*math.Sin(x*phi*phi+2.07) +
		0.063*math.Sin(x*phi*phi*phi+0.94)
	return v / (0.500 + 0.250 + 0.125 + 0.063)
}

func perlin2D(x, y float64) float64 {
	return perlin1D(x)*0.6 + perlin1D(y*1.3+7.77)*0.4
}

// ─── star field ───────────────────────────────────────────────────────────────

type star3D struct {
	x, y float64
	z    float64
	br   float64
	t    float64 // per-star noise phase
}

func newStarField() []star3D {
	stars := make([]star3D, numStars)
	for i := range stars {
		stars[i] = randomStar()
		stars[i].z = rand.Float64()*4.5 + 0.3 // #nosec G404 -- math/rand is sufficient for cosmetic animation
		stars[i].t = rand.Float64() * 20.0    // #nosec G404
	}
	return stars
}

func randomStar() star3D {
	return star3D{
		x:  rand.Float64()*2 - 1,     // #nosec G404 -- math/rand is sufficient for cosmetic animation
		y:  rand.Float64()*2 - 1,     // #nosec G404
		z:  4.0 + rand.Float64()*1.5, // #nosec G404
		br: rand.Float64()*0.5 + 0.5, // #nosec G404
		t:  rand.Float64() * 20.0,    // #nosec G404
	}
}

func (s *star3D) advance(dt float64) {
	noise := perlin1D(s.t * 0.7)
	s.z -= dt * 0.75 * (1.0 + 0.25*noise) // gentle warp
	s.t += dt
	if s.z <= 0.05 {
		*s = randomStar()
	}
}

func (s star3D) project(w, h int) (sx, sy int, brightness float64) {
	if s.z < 0.01 {
		return -1, -1, 0
	}
	cx, cy := float64(w)*0.5, float64(h)*0.5
	px := cx + s.x*cx/s.z
	py := cy + s.y*cy/s.z
	brightness = math.Min(1.0, s.br*0.6/s.z)
	return int(px), int(py), brightness
}

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

func starColor(brightness float64) lipgloss.Color {
	b := clamp(int(brightness*255), 0, 255)
	return lipgloss.Color(rgbHex(b*6/10, b*7/10, b))
}

// ─── chroma ghost ─────────────────────────────────────────────────────────────

// ghostChar holds a single character's baked screen position at the moment
// of impact (cardZ ≈ 0).
type ghostChar struct {
	x, y int
	ch   rune
}

// chromaGhost is the fading chroma-aberration afterimage of a card.
type chromaGhost struct {
	chars []ghostChar
	age   float64 // 0=fresh → 1=fully faded
}

// ─── intro state ──────────────────────────────────────────────────────────────

type introState struct {
	stars     []star3D
	elapsed   float64
	ghost     *chromaGhost
	ghostCard int // which card index the ghost belongs to
}

func newIntroState() introState {
	return introState{stars: newStarField(), ghostCard: -1}
}

func (s *introState) done() bool {
	total := starfieldLeadIn + float64(len(introCards))*cardDuration + 0.5
	return s.elapsed >= total
}

func (s *introState) advance(dt float64) {
	s.elapsed += dt
	for i := range s.stars {
		s.stars[i].advance(dt)
	}
	if s.ghost != nil {
		s.ghost.age += dt / ghostFadeTime
		if s.ghost.age >= 1.0 {
			s.ghost = nil
		}
	}
}

func (s *introState) currentCard() (cardIdx int, localT float64) {
	if s.elapsed < starfieldLeadIn {
		return -1, 0
	}
	offset := s.elapsed - starfieldLeadIn
	idx := int(offset / cardDuration)
	if idx >= len(introCards) {
		return len(introCards), 1
	}
	return idx, math.Mod(offset, cardDuration) / cardDuration
}

// stableDrift returns a card's lateral drift, computed from a fixed time so it
// doesn't wobble during the card's lifetime.
func stableDrift(cardIdx, w int) float64 {
	t := starfieldLeadIn + float64(cardIdx)*cardDuration
	return perlin2D(t*0.32, float64(cardIdx)*3.1) * float64(w) * 0.011
}

// ─── rendering ────────────────────────────────────────────────────────────────

type introCell struct {
	ch    rune
	color lipgloss.Color
	bold  bool
}

func renderIntro(s *introState, w, h int) []string {
	grid := make([][]introCell, h)
	bg := lipgloss.Color(introBg)
	for y := range grid {
		grid[y] = make([]introCell, w)
		for x := range grid[y] {
			grid[y][x] = introCell{' ', bg, false}
		}
	}

	// Layer 1: stars
	for _, star := range s.stars {
		sx, sy, br := star.project(w, h)
		if sx >= 0 && sx < w && sy >= 0 && sy < h && br > 0 {
			if grid[sy][sx].ch == ' ' || br > 0.5 {
				grid[sy][sx] = introCell{starRune(br), starColor(br), br > 0.85}
			}
		}
	}

	// Layer 2: chroma ghost (over stars)
	if s.ghost != nil {
		drawChromaGhost(grid, s.ghost, w, h)
	}

	// Layer 3: live card — only during approach phase (localT < cardCrossT).
	// Once the card reaches the viewer it stops being drawn; the ghost takes over.
	cardIdx, localT := s.currentCard()
	if cardIdx >= 0 && cardIdx < len(introCards) {
		text := introCards[cardIdx]
		drift := stableDrift(cardIdx, w)

		// Compute Z continuously at constant velocity: approach then fly-past.
		// Approach:  localT 0→cardCrossT, Z: maxApproachZ→0
		// Fly-past:  localT cardCrossT→1, Z: 0→-maxApproachZ*(1-crossT)/crossT
		var cardZ float64
		if localT <= cardCrossT {
			t := localT / cardCrossT
			cardZ = maxApproachZ * (1.0 - t)
		} else {
			t := (localT - cardCrossT) / cardCrossT
			cardZ = -maxApproachZ * t
		}

		// Capture ghost the moment the card crosses the viewer
		if localT >= cardCrossT && s.ghostCard != cardIdx {
			s.ghostCard = cardIdx
			s.ghost = captureGhost(text, drift, w, h)
		}

		// Approach chroma fringes (only during approach, before viewer)
		if cardZ > 0 && cardZ < 2.0 {
			drawApproachChroma(grid, text, cardZ, drift, w, h)
		}

		// Draw card for the full duration — approach and fly-past.
		// During fly-past, charSpread grows so characters race off-screen.
		drawCard(grid, text, cardZ, drift, w, h)
	}

	bgStyle := lipgloss.NewStyle().Background(bg)
	lines := make([]string, h)
	for y, row := range grid {
		var sb strings.Builder
		sb.WriteString(bgStyle.Render(""))
		for _, c := range row {
			style := lipgloss.NewStyle().Foreground(c.color).Background(bg)
			if c.bold {
				style = style.Bold(true)
			}
			sb.WriteString(style.Render(string(c.ch)))
		}
		lines[y] = sb.String()
	}
	return lines
}

// ─── card geometry ────────────────────────────────────────────────────────────

// charSpread returns the inter-character gap (in columns) at depth cardZ.
// The word keeps its shape — maximum spread is capped so the text never
// expands beyond ~25 % of the screen width, giving a compact fly-past.
func charSpread(cardZ float64, n, w int) int {
	if n <= 1 {
		return 0
	}
	// Cap spread so total word width ≤ w/4 at peak, also hard-cap at 4 cols.
	nn := n - 1
	if nn < 1 {
		nn = 1
	}
	maxSp := (w/4 - n) / nn
	if maxSp < 1 {
		maxSp = 1
	}
	if maxSp > 4 {
		maxSp = 4
	}
	if cardZ <= 0 {
		// Fly-past: spread grows so characters race off-screen
		extra := int(float64(w) * math.Abs(cardZ) * flyOutFactor)
		return maxSp + extra
	}
	// Approach: spread grows as 1/Z; visible earlier with crossZ=0.80
	const crossZ = 0.80
	sp := int(float64(maxSp) * crossZ / cardZ)
	if sp > maxSp {
		return maxSp
	}
	return sp
}

// tiltAmount returns the total tilt amplitude in rows.
// Zero at Z = 2.2 (far away), grows to 22 % of screen height at the viewer.
func tiltAmount(cardZ float64, h int) float64 {
	if cardZ <= 0 {
		return float64(h) * 0.22
	}
	t := math.Max(0, 1.0-cardZ/2.2)
	ts := t * t * (3 - 2*t) // smoothstep
	return ts * float64(h) * 0.22
}

// cardPositions fills (x, y) for each character in text at the given depth/drift.
// Tilt is computed relative to the word's own centre so it's always visible
// regardless of word length.
func cardPositions(text string, cardZ, drift float64, w, h int) (xs, ys []int) {
	chars := []rune(strings.ToUpper(text))
	n := len(chars)
	if n == 0 {
		return nil, nil
	}

	sp := charSpread(cardZ, n, w)
	totalWidth := n + (n-1)*sp
	cx := w / 2
	cy := h / 2

	absZ := math.Abs(cardZ)
	vy := int(float64(h) * 0.035 * math.Max(0, 1.0-absZ/2.0))
	startX := cx - totalWidth/2 + int(drift)
	baseY := cy - vy
	tilt := tiltAmount(cardZ, h)

	xs = make([]int, n)
	ys = make([]int, n)
	for i := range chars {
		x := startX + i*(sp+1)
		xs[i] = x

		// Word-relative normalised position: -0.5 (left end) to +0.5 (right end)
		var norm float64
		if n > 1 {
			norm = float64(i)/float64(n-1) - 0.5
		}
		ys[i] = baseY + int(norm*tilt)
	}
	return xs, ys
}

// captureGhost bakes the exact rendered positions at the peak frame (cardZ ≈ 0).
func captureGhost(text string, drift float64, w, h int) *chromaGhost {
	chars := []rune(strings.ToUpper(text))
	xs, ys := cardPositions(text, 0.0, drift, w, h)
	if xs == nil {
		return &chromaGhost{}
	}
	var gc []ghostChar
	for i, ch := range chars {
		x, y := xs[i], ys[i]
		if x >= 0 && x < w && y >= 0 && y < h {
			gc = append(gc, ghostChar{x, y, ch})
		}
	}
	return &chromaGhost{chars: gc, age: 0}
}

// drawCard renders text onto the grid at depth cardZ with drift and tilt.
func drawCard(grid [][]introCell, text string, cardZ, drift float64, w, h int) {
	chars := []rune(strings.ToUpper(text))
	if len(chars) == 0 {
		return
	}
	xs, ys := cardPositions(text, cardZ, drift, w, h)
	color, bold := cardColor(cardZ)
	for i, ch := range chars {
		x, y := xs[i], ys[i]
		if x < 0 || x >= w || y < 0 || y >= h {
			continue
		}
		grid[y][x] = introCell{ch, color, bold}
	}
}

// ─── approach chroma ──────────────────────────────────────────────────────────

// drawApproachChroma renders multicolour fringing around the approaching card.
// It starts as a faint halo when the card is far away (Z≈2.0) and intensifies
// to a vivid RGB split as the card nears the viewer (Z→0).
// Render order: drawn before drawCard so the card's own characters always win.
func drawApproachChroma(grid [][]introCell, text string, cardZ, drift float64, w, h int) {
	if cardZ <= 0 || cardZ > 2.0 {
		return
	}
	// intensity: 0 at Z=2.0, 1 at Z=0, quadratic so it builds slowly then surges
	raw := (2.0 - cardZ) / 2.0
	intensity := raw * raw

	maxShift := 1 + int(intensity*3) // 1..4 fringe columns

	chars := []rune(strings.ToUpper(text))
	xs, ys := cardPositions(text, cardZ, drift, w, h)
	if xs == nil {
		return
	}

	for i, ch := range chars {
		x, y := xs[i], ys[i]

		for shift := 1; shift <= maxShift; shift++ {
			alpha := intensity * float64(maxShift-shift+1) / float64(maxShift+1)

			// Red channel — left
			rx := x - shift
			if rx >= 0 && rx < w && y >= 0 && y < h && grid[y][rx].ch == ' ' {
				v := clamp(int(alpha*210), 0, 255)
				if v > 8 {
					grid[y][rx] = introCell{ch, lipgloss.Color(rgbHex(v, v/10, v/14)), false}
				}
			}

			// Cyan channel — right
			bx := x + shift
			if bx >= 0 && bx < w && y >= 0 && y < h && grid[y][bx].ch == ' ' {
				v := clamp(int(alpha*190), 0, 255)
				if v > 8 {
					grid[y][bx] = introCell{ch, lipgloss.Color(rgbHex(v/12, clamp(v*3/4, 0, 255), v)), false}
				}
			}
		}

		// Green channel — one row above, appears when close (intensity > 0.35)
		if intensity > 0.35 && y-1 >= 0 && y-1 < h && grid[y-1][x].ch == ' ' {
			alpha := (intensity - 0.35) / 0.65
			v := clamp(int(alpha*160), 0, 255)
			if v > 8 {
				grid[y-1][x] = introCell{ch, lipgloss.Color(rgbHex(v/8, v, v/4)), false}
			}
		}
	}
}

// ─── chroma ghost rendering ───────────────────────────────────────────────────

// drawChromaGhost renders the afterimage with chromatic aberration:
//   - bright red channel shifted 1-3 cols left
//   - bright cyan/blue channel shifted 1-3 cols right
//   - warm white at original position
//   - dim blue vertical tilt trail
//
// Chroma echoes always overwrite whatever is in the cell (including stars)
// so they are guaranteed to be visible.
func drawChromaGhost(grid [][]introCell, g *chromaGhost, w, h int) {
	if g == nil || len(g.chars) == 0 {
		return
	}
	raw := 1.0 - g.age
	fade := raw * math.Sqrt(raw) // ease-out: stays bright, then drops

	for _, gc := range g.chars {
		x, y, ch := gc.x, gc.y, gc.ch

		// Red channel — columns to the left
		for shift, strength := range map[int]float64{-1: 1.0, -2: 0.72, -3: 0.40} {
			rx := x + shift
			if rx >= 0 && rx < w && y >= 0 && y < h {
				v := clamp(int(fade*strength*255), 0, 255)
				if v > 6 {
					// pure red with tiny green to avoid pure #ff0000 harshness
					grid[y][rx] = introCell{ch, lipgloss.Color(rgbHex(v, v/12, v/18)), false}
				}
			}
		}

		// Cyan/blue channel — columns to the right
		for shift, strength := range map[int]float64{1: 0.95, 2: 0.68, 3: 0.36} {
			bx := x + shift
			if bx >= 0 && bx < w && y >= 0 && y < h {
				v := clamp(int(fade*strength*255), 0, 255)
				if v > 6 {
					// cyan: moderate green + full blue
					grid[y][bx] = introCell{ch, lipgloss.Color(rgbHex(v/14, clamp(v*4/5, 0, 255), v)), false}
				}
			}
		}

		// Main ghost: warm white at original position — always on top
		if x >= 0 && x < w && y >= 0 && y < h {
			v := clamp(int(fade*255), 0, 255)
			mc := lipgloss.Color(rgbHex(v, clamp(v*9/10, 0, 255), clamp(v*8/10, 0, 255)))
			grid[y][x] = introCell{ch, mc, v > 180}
		}

		// Tilt trail: dim blue echo one row in the tilt direction
		cx := w / 2
		if x != cx {
			// Characters right of centre tilt downward, so trail is above them
			ghostY := y - sign(x-cx)
			if ghostY >= 0 && ghostY < h {
				v := clamp(int(fade*0.42*255), 0, 255)
				if v > 8 && grid[ghostY][x].ch == ' ' {
					grid[ghostY][x] = introCell{ch, lipgloss.Color(rgbHex(v/8, v/6, v)), false}
				}
			}
		}
	}
}

// ─── colour helpers ───────────────────────────────────────────────────────────

func cardColor(cardZ float64) (lipgloss.Color, bool) {
	dist := math.Abs(cardZ)
	switch {
	case dist > 1.8:
		return lipgloss.Color("#0d1a33"), false
	case dist > 1.3:
		return lipgloss.Color("#1a3366"), false
	case dist > 0.9:
		return lipgloss.Color("#2255aa"), false
	case dist > 0.55:
		return lipgloss.Color("#4488dd"), false
	case dist > 0.25:
		return lipgloss.Color("#88bbff"), true
	case dist > 0.06:
		return lipgloss.Color("#ccddff"), true
	default:
		return lipgloss.Color("#ffffff"), true
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

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sign(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}
