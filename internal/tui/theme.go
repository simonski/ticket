package tui

import "github.com/charmbracelet/lipgloss"

// ThemeID identifies a named theme.
type ThemeID string

const (
	ThemeTheGrey       ThemeID = "the-grey"
	ThemeDeepDarkGreen ThemeID = "deep-dark-green"
	ThemeMaudlinMaroon ThemeID = "maudlin-maroon"
	ThemeNeonNights    ThemeID = "neon-nights"
	ThemeBrightBreezy  ThemeID = "bright-n-breezy"
	ThemeContrastDisco ThemeID = "contrast-my-disco"
	ThemeLoFi          ThemeID = "lo-fi"
	ThemeBrainFuzz     ThemeID = "brain-fuzz"
	ThemeKPop          ThemeID = "k-pop"
)

// Theme holds all colour values for a visual theme.
type Theme struct {
	ID       ThemeID
	Name     string
	Bg       lipgloss.Color
	Fg       lipgloss.Color
	Accent   lipgloss.Color
	Muted    lipgloss.Color
	Border   lipgloss.Color
	SelBg    lipgloss.Color
	SelFg    lipgloss.Color
	Header   lipgloss.Color
	StatusBg lipgloss.Color
	StatusFg lipgloss.Color
	// ECG pulse colours (gradient from dim to bright)
	PulseGrad []lipgloss.Color
	HasPulse  bool
	// ECGStyle overrides animation defaults; zero values use defaultECGParams.
	ECGStyle ecgParams
}

// ThemeOrder controls the cycle order when the user presses 't'.
var ThemeOrder = []ThemeID{
	ThemeTheGrey,
	ThemeDeepDarkGreen,
	ThemeMaudlinMaroon,
	ThemeNeonNights,
	ThemeBrightBreezy,
	ThemeContrastDisco,
	ThemeLoFi,
	ThemeBrainFuzz,
	ThemeKPop,
}

var Themes = map[ThemeID]Theme{
	ThemeTheGrey: {
		ID:       ThemeTheGrey,
		Name:     "The Grey",
		Bg:       "#111213",
		Fg:       "#9a9ea3",
		Accent:   "#c0c4c8",
		Muted:    "#454a50",
		Border:   "#2a2d30",
		SelBg:    "#1e2124",
		SelFg:    "#dde0e3",
		Header:   "#b0b4b8",
		StatusBg: "#1a1d1f",
		StatusFg: "#7a7e82",
		PulseGrad: []lipgloss.Color{
			"#1a1d1f", "#222528", "#2a2d30", "#363a3e",
			"#474b50", "#585d62", "#6a6f74", "#8a8f94",
		},
		HasPulse: false,
	},
	ThemeDeepDarkGreen: {
		ID:       ThemeDeepDarkGreen,
		Name:     "Deep Dark Green",
		Bg:       "#030d03",
		Fg:       "#7fcc7f",
		Accent:   "#00ff41",
		Muted:    "#2d5a27",
		Border:   "#0a2a0a",
		SelBg:    "#0a2e0a",
		SelFg:    "#00ff41",
		Header:   "#00cc33",
		StatusBg: "#0a2e0a",
		StatusFg: "#00ff41",
		PulseGrad: []lipgloss.Color{
			"#061a06", "#0a2a0a", "#0f3d0f", "#155215",
			"#1a6b1a", "#20851a", "#1ec920", "#00ff41",
		},
		HasPulse: true,
		// Very slow: ~12s per lap; long memory trail + wide anticipation glow
		ECGStyle: ecgParams{
			speedFactor: 0.25,
			trailLen:    80,
			peakWidth:   2,
			pWave:       22,
		},
	},
	ThemeMaudlinMaroon: {
		ID:       ThemeMaudlinMaroon,
		Name:     "Maudlin Maroon",
		Bg:       "#0d0305",
		Fg:       "#c08080",
		Accent:   "#cc2244",
		Muted:    "#4d1a22",
		Border:   "#3d0f1a",
		SelBg:    "#2d0d14",
		SelFg:    "#ff4466",
		Header:   "#aa2233",
		StatusBg: "#2d0d14",
		StatusFg: "#ff6680",
		PulseGrad: []lipgloss.Color{
			"#3d0f1a", "#5d1a26", "#7a2233", "#992244",
			"#bb2244", "#dd3355", "#ff4466", "#ff6680",
		},
		HasPulse: false,
	},
	ThemeNeonNights: {
		ID:       ThemeNeonNights,
		Name:     "Neon Nights",
		Bg:       "#030309",
		Fg:       "#b0b0d0",
		Accent:   "#ff00ff",
		Muted:    "#2a1a4d",
		Border:   "#1a0d3d",
		SelBg:    "#130a2e",
		SelFg:    "#ff00ff",
		Header:   "#cc00cc",
		StatusBg: "#130a2e",
		StatusFg: "#dd88ff",
		PulseGrad: []lipgloss.Color{
			"#1a0d3d", "#2a1a5d", "#3a2080", "#5500aa",
			"#7700cc", "#aa00dd", "#cc00ee", "#ff00ff",
		},
		HasPulse: false,
	},
	ThemeBrightBreezy: {
		ID:       ThemeBrightBreezy,
		Name:     "Bright n Breezy",
		Bg:       "#f0f4f8",
		Fg:       "#2d3a4a",
		Accent:   "#0077cc",
		Muted:    "#8a99aa",
		Border:   "#c0ccd8",
		SelBg:    "#d0e4f4",
		SelFg:    "#0055aa",
		Header:   "#005599",
		StatusBg: "#d0e4f4",
		StatusFg: "#003377",
		PulseGrad: []lipgloss.Color{
			"#c0ccd8", "#aabcd0", "#88aac8", "#5599bb",
			"#3388cc", "#1177dd", "#0066cc", "#0055ff",
		},
		HasPulse: false,
	},
	ThemeContrastDisco: {
		ID:       ThemeContrastDisco,
		Name:     "Contrast My Disco",
		Bg:       "#000000",
		Fg:       "#ffffff",
		Accent:   "#ffff00",
		Muted:    "#888888",
		Border:   "#444444",
		SelBg:    "#222222",
		SelFg:    "#ffff00",
		Header:   "#ffdd00",
		StatusBg: "#111111",
		StatusFg: "#ffff00",
		PulseGrad: []lipgloss.Color{
			"#222222", "#444400", "#666600", "#888800",
			"#aaaa00", "#cccc00", "#eeee00", "#ffff00",
		},
		HasPulse: false,
	},
	ThemeLoFi: {
		ID:       ThemeLoFi,
		Name:     "Lo-Fi",
		Bg:       "#1a1510",
		Fg:       "#c8b89a",
		Accent:   "#d4956a",
		Muted:    "#6b5a48",
		Border:   "#3d2e22",
		SelBg:    "#2a1e15",
		SelFg:    "#e8c89a",
		Header:   "#c07a50",
		StatusBg: "#2a1e15",
		StatusFg: "#d4956a",
		PulseGrad: []lipgloss.Color{
			"#3d2e22", "#5a3a28", "#7a4a30", "#9a5a38",
			"#b46a40", "#cc7a48", "#e08850", "#f49660",
		},
		HasPulse: false,
	},
	ThemeBrainFuzz: {
		ID:       ThemeBrainFuzz,
		Name:     "Brain Fuzz",
		Bg:       "#220066",
		Fg:       "#ff6600",
		Accent:   "#00ffcc",
		Muted:    "#ff0066",
		Border:   "#ffff00",
		SelBg:    "#ff00ff",
		SelFg:    "#00ff00",
		Header:   "#ff3300",
		StatusBg: "#ff0099",
		StatusFg: "#00ffff",
		PulseGrad: []lipgloss.Color{
			"#440088", "#660099", "#8800bb", "#aa00cc",
			"#cc00dd", "#ee00ee", "#ff00ff", "#ff88ff",
		},
		HasPulse: false,
	},
	ThemeKPop: {
		ID:       ThemeKPop,
		Name:     "K-Pop",
		Bg:       "#1a0a1f",
		Fg:       "#f0b8d8",
		Accent:   "#ff69b4",
		Muted:    "#8a5a78",
		Border:   "#5a2d4a",
		SelBg:    "#3d1a33",
		SelFg:    "#ffaad8",
		Header:   "#ff69b4",
		StatusBg: "#3d1a33",
		StatusFg: "#ffaad8",
		PulseGrad: []lipgloss.Color{
			"#3d1a33", "#5a2244", "#772d55", "#993366",
			"#bb3d77", "#dd4888", "#ff55aa", "#ff88cc",
		},
		HasPulse: false,
	},
}

// NextTheme returns the theme after the given one in ThemeOrder.
func NextTheme(current ThemeID) ThemeID {
	for i, id := range ThemeOrder {
		if id == current {
			return ThemeOrder[(i+1)%len(ThemeOrder)]
		}
	}
	return ThemeOrder[0]
}

// ecgColor returns a colour from the pulse gradient at intensity 0.0–1.0.
func (t Theme) ecgColor(intensity float64) lipgloss.Color {
	g := t.PulseGrad
	if len(g) == 0 {
		return t.Border
	}
	if intensity <= 0 {
		return g[0]
	}
	if intensity >= 1 {
		return g[len(g)-1]
	}
	idx := int(intensity * float64(len(g)-1))
	if idx >= len(g)-1 {
		return g[len(g)-1]
	}
	return g[idx]
}
