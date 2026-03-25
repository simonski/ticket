// Package tui implements the interactive terminal UI for tk (tk -g).
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

// Run starts the full-screen TUI. themeID selects an initial theme; use ""
// to use the persisted or default theme.
func Run(svc libticket.Service, cfg config.Config, project store.Project, themeID string) error {
	// Determine theme: CLI flag > persisted > default (The Grey)
	th := Themes[ThemeTheGrey]
	if t, ok := Themes[ThemeID(themeID)]; ok {
		th = t
	} else if cfg.TUITheme != "" {
		if t, ok := Themes[ThemeID(cfg.TUITheme)]; ok {
			th = t
		}
	}

	m := newModel(svc, cfg, th)
	m.project = project

	// Restore persisted state when TUIPersistState is true (default: on).
	// Config zero-value has TUIPersistState=false, so we treat it as enabled
	// unless explicitly disabled via future config toggle.
	persistEnabled := !cfg.TUIDisablePersist
	if persistEnabled && cfg.TUIMode != "" {
		switch cfg.TUIMode {
		case "summary":
			m.mode = modeSummary
		case "list":
			m.mode = modeList
		case "settings":
			m.mode = modeSettings
		case "projects":
			m.mode = modeProjects
		case "ideas":
			m.mode = modeIdeas
		}
		if cfg.TUICursor > 0 {
			m.cursor = cfg.TUICursor
		}
	}

	// Find initial settings cursor for the active theme
	for i, id := range ThemeOrder {
		if id == th.ID {
			m.settingsCursor = i
			break
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()

	// Persist state on exit when enabled.
	if persistEnabled {
		if fm, ok := final.(Model); ok {
			// Use fm.cfg so in-session changes (project switch, theme) are included.
			saveTUIState(fm.cfg, fm)
		}
	}

	return err
}

// RunEdit opens the TUI directly in edit mode for the given ticket.
func RunEdit(svc libticket.Service, cfg config.Config, project store.Project, ticket store.Ticket) error {
	th := Themes[ThemeTheGrey]
	if cfg.TUITheme != "" {
		if t, ok := Themes[ThemeID(cfg.TUITheme)]; ok {
			th = t
		}
	}

	m := newModel(svc, cfg, th)
	m.project = project
	m.selected = &ticket
	m.form = newEditForm(ticket)
	m.mode = modeEdit

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func saveTUIState(cfg config.Config, m Model) {
	cfg.TUITheme = string(m.theme.ID)
	switch m.mode {
	case modeSummary:
		cfg.TUIMode = "summary"
	case modeList:
		cfg.TUIMode = "list"
	case modeSettings:
		cfg.TUIMode = "settings"
	case modeProjects:
		cfg.TUIMode = "projects"
	case modeIdeas:
		cfg.TUIMode = "ideas"
	default:
		cfg.TUIMode = "summary"
	}
	cfg.TUICursor = m.cursor

	// Persist expanded epic IDs
	var expanded []string
	for id, isOpen := range m.expanded {
		if isOpen {
			expanded = append(expanded, id)
		}
	}
	cfg.TUIExpandedEpics = expanded

	_ = config.Save(cfg)
}
