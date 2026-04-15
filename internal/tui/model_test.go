package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
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

func TestViewDetailShowsExtendedTicketContext(t *testing.T) {
	roleID := int64(7)
	projectSdlcID := int64(11)
	parentID := "PRJ-1"
	selected := store.Ticket{
		ID:                 "PRJ-2",
		ParentID:           &parentID,
		Type:               "task",
		Title:              "Implement lifecycle UI",
		Description:        "Show richer lifecycle details in the TUI.",
		AcceptanceCriteria: "Display draft and SDLC context.",
		RoleID:             &roleID,
		Stage:              store.StageDevelop,
		State:              store.StateActive,
		Status:             "develop/active",
		Draft:              true,
		DORMap:             store.GuidanceMap{store.DefaultGuidanceStageKey: "Ticket DoR"},
	}

	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.width = 100
	m.height = 30
	m.project = store.Project{
		ID:     1,
		Prefix: "PRJ",
		Title:  "Project Alpha",
		SdlcID: &projectSdlcID,
		DODMap: store.GuidanceMap{store.DefaultGuidanceStageKey: "Project DoD"},
		ACMap:  store.GuidanceMap{store.DefaultGuidanceStageKey: "Project AC"},
		Status: "open",
	}
	m.selected = &selected
	m.roles = []store.Role{{
		ID:    roleID,
		Title: "Engineer",
	}}
	m.sdlcs = []store.SdlcWithStages{{
		Sdlc: store.Sdlc{ID: projectSdlcID, Name: "Default Flow"},
	}}
	m.items = []listItem{{
		ticket: store.Ticket{ID: parentID},
	}}

	out := strings.Join(m.viewDetail(), "\n")
	for _, needle := range []string{
		"flags",
		"draft",
		"effective sdlc",
		"Default Flow (project default)",
		"role",
		"Engineer",
		"project guidance",
		"ticket guidance",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("detail view missing %q:\n%s", needle, out)
		}
	}
}

func TestProjectEditAndNewTicketViewsShowLifecycleFields(t *testing.T) {
	projectSdlcID := int64(3)
	ticketSdlcID := int64(5)

	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.width = 100
	m.height = 30
	m.sdlcs = []store.SdlcWithStages{
		{Sdlc: store.Sdlc{ID: projectSdlcID, Name: "Project Flow"}},
		{Sdlc: store.Sdlc{ID: ticketSdlcID, Name: "Ticket Flow"}},
	}
	m.projectForm = newProjectEditForm(store.Project{
		Prefix:        "PRJ",
		Visibility:    store.ProjectVisibilityPublic,
		DefaultDraft:  true,
		SdlcID:        &projectSdlcID,
		GitRepository: "github.com/example/project",
	})
	m.newForm = makeNewTicketForm()
	m.newForm.draft = true
	m.newForm.sdlcID = &ticketSdlcID

	projectOut := strings.Join(m.viewProjectEdit(), "\n")
	for _, needle := range []string{"visibility:", "default draft:", "default sdlc:", "git repo:"} {
		if !strings.Contains(projectOut, needle) {
			t.Fatalf("project edit view missing %q:\n%s", needle, projectOut)
		}
	}

	newTicketOut := strings.Join(m.viewNewTicket(), "\n")
	for _, needle := range []string{"draft:", "sdlc:", "Ticket Flow"} {
		if !strings.Contains(newTicketOut, needle) {
			t.Fatalf("new ticket view missing %q:\n%s", needle, newTicketOut)
		}
	}
}
