// Package tui implements the interactive terminal UI for tk (tk -g).
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

// Run starts the full-screen TUI. themeID selects an initial theme; use ""
// for the default (deep-dark-green).
func Run(svc libticket.Service, cfg config.Config, project store.Project, themeID string) error {
	th := Themes[ThemeDeepDarkGreen]
	if t, ok := Themes[ThemeID(themeID)]; ok {
		th = t
	}
	m := newModel(svc, cfg, th)
	m.project = project

	// Find initial theme cursor
	for i, id := range ThemeOrder {
		if id == th.ID {
			m.settingsCursor = i
			break
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
