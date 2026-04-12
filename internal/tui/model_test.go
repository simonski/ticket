package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonski/ticket/internal/config"
)

func TestNewModelInitializesCoreState(t *testing.T) {
	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])

	if m.mode != modeIntro {
		t.Fatalf("mode = %v, want %v", m.mode, modeIntro)
	}
	if m.expanded == nil {
		t.Fatal("expanded = nil, want initialized map")
	}
	if m.wfExpanded == nil {
		t.Fatal("wfExpanded = nil, want initialized map")
	}
	if m.cmdInput.Placeholder != "enter command..." {
		t.Fatalf("cmdInput.Placeholder = %q", m.cmdInput.Placeholder)
	}
}

func TestModelUpdateHandlesWindowSizeAndTick(t *testing.T) {
	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.mode = modeSummary
	start := m.lastTick

	updatedAny, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Fatalf("window size cmd = %v, want nil", cmd)
	}
	updated, ok := updatedAny.(Model)
	if !ok {
		t.Fatalf("Update(window size) type = %T, want Model", updatedAny)
	}
	if updated.width != 80 || updated.height != 24 {
		t.Fatalf("window size = %dx%d, want 80x24", updated.width, updated.height)
	}

	updatedAny, cmd = updated.Update(tickMsg(start.Add(time.Second)))
	if cmd == nil {
		t.Fatal("tick cmd = nil, want follow-up tick command")
	}
	updated, ok = updatedAny.(Model)
	if !ok {
		t.Fatalf("Update(tick) type = %T, want Model", updatedAny)
	}
	if updated.tickCount == 0 {
		t.Fatal("tickCount = 0, want incremented")
	}
	if !updated.lastTick.After(start) {
		t.Fatalf("lastTick = %v, want after %v", updated.lastTick, start)
	}
}
