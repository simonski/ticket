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
	projectWorkflowID := int64(11)
	parentID := "PRJ-1"
	selected := store.Ticket{
		ID:                 "PRJ-2",
		ParentID:           &parentID,
		Type:               "task",
		Title:              "Implement lifecycle UI",
		Description:        "Show richer lifecycle details in the TUI.",
		AcceptanceCriteria: "Display draft and Workflow context.",
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
		ID:         1,
		Prefix:     "PRJ",
		Title:      "Project Alpha",
		WorkflowID: &projectWorkflowID,
		DODMap:     store.GuidanceMap{store.DefaultGuidanceStageKey: "Project DoD"},
		ACMap:      store.GuidanceMap{store.DefaultGuidanceStageKey: "Project AC"},
		Status:     "open",
	}
	m.selected = &selected
	m.roles = []store.Role{{
		ID:    roleID,
		Title: "Engineer",
	}}
	m.workflows = []store.WorkflowWithStages{{
		Workflow: store.Workflow{ID: projectWorkflowID, Name: "Default Flow"},
	}}
	m.items = []listItem{{
		ticket: store.Ticket{ID: parentID},
	}}

	out := strings.Join(m.viewDetail(), "\n")
	for _, needle := range []string{
		"flags",
		"draft",
		"effective workflow",
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
	projectWorkflowID := int64(3)
	ticketWorkflowID := int64(5)

	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.width = 100
	m.height = 30
	m.workflows = []store.WorkflowWithStages{
		{Workflow: store.Workflow{ID: projectWorkflowID, Name: "Project Flow"}},
		{Workflow: store.Workflow{ID: ticketWorkflowID, Name: "Ticket Flow"}},
	}
	m.projectForm = newProjectEditForm(store.Project{
		Prefix:        "PRJ",
		Visibility:    store.ProjectVisibilityPublic,
		DefaultDraft:  true,
		WorkflowID:    &projectWorkflowID,
		GitRepository: "github.com/example/project",
	})
	m.newForm = makeNewTicketForm()
	m.newForm.draft = true
	m.newForm.workflowID = &ticketWorkflowID

	projectOut := strings.Join(m.viewProjectEdit(), "\n")
	for _, needle := range []string{"visibility:", "default draft:", "default workflow:", "git repo:"} {
		if !strings.Contains(projectOut, needle) {
			t.Fatalf("project edit view missing %q:\n%s", needle, projectOut)
		}
	}

	newTicketOut := strings.Join(m.viewNewTicket(), "\n")
	for _, needle := range []string{"draft:", "workflow:", "Ticket Flow"} {
		if !strings.Contains(newTicketOut, needle) {
			t.Fatalf("new ticket view missing %q:\n%s", needle, newTicketOut)
		}
	}
}

func TestBuildBoardColumnsUsesWorkflowStageOrder(t *testing.T) {
	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.workflows = []store.WorkflowWithStages{{
		Workflow: store.Workflow{ID: 1, Name: "Flow"},
		Stages: []store.WorkflowStage{
			{StageName: "backlog"},
			{StageName: "doing"},
			{StageName: "done"},
		},
	}}
	m.toplevel = []store.Ticket{
		{ID: "PRJ-1", Stage: "backlog", Status: "backlog/idle", Type: "task", Title: "Backlog item"},
		{ID: "PRJ-2", Stage: "done", Status: "done/success", Type: "task", Title: "Done item"},
	}

	m.buildBoardColumns()

	if len(m.boardCols) != 3 {
		t.Fatalf("board column count = %d, want 3", len(m.boardCols))
	}
	if m.boardCols[0].stage != "backlog" || m.boardCols[1].stage != "doing" || m.boardCols[2].stage != "done" {
		t.Fatalf("unexpected board stage order: %#v", []string{m.boardCols[0].stage, m.boardCols[1].stage, m.boardCols[2].stage})
	}
	if len(m.boardCols[0].tickets) != 1 || m.boardCols[0].tickets[0].ID != "PRJ-1" {
		t.Fatalf("backlog column tickets = %#v", m.boardCols[0].tickets)
	}
	if len(m.boardCols[2].tickets) != 1 || m.boardCols[2].tickets[0].ID != "PRJ-2" {
		t.Fatalf("done column tickets = %#v", m.boardCols[2].tickets)
	}
}

func TestHandleKeyBoardTransitionsAndSelection(t *testing.T) {
	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.mode = modeBoard
	m.width = 100
	m.height = 30
	m.boardCols = []boardColumn{
		{stage: "design", tickets: []store.Ticket{{ID: "PRJ-1", Stage: "design", Status: "design/active", Type: "task", Title: "First"}}},
		{stage: "develop", tickets: []store.Ticket{{ID: "PRJ-2", Stage: "develop", Status: "develop/idle", Type: "task", Title: "Second"}}},
	}
	m.boardInHeader = true

	updatedAny, _ := m.handleKeyBoard("down")
	updated, ok := updatedAny.(Model)
	if !ok {
		t.Fatalf("handleKeyBoard type = %T, want Model", updatedAny)
	}
	if updated.boardInHeader {
		t.Fatal("board should move from header to body on down")
	}

	updatedAny, _ = updated.handleKeyBoard("right")
	updated, ok = updatedAny.(Model)
	if !ok {
		t.Fatalf("handleKeyBoard type = %T, want Model", updatedAny)
	}
	if updated.boardCol != 1 {
		t.Fatalf("boardCol = %d, want 1 after moving right", updated.boardCol)
	}

	updatedAny, _ = updated.handleKeyBoard("enter")
	updated, ok = updatedAny.(Model)
	if !ok {
		t.Fatalf("handleKeyBoard type = %T, want Model", updatedAny)
	}
	if updated.mode != modeDetail {
		t.Fatalf("mode = %v, want modeDetail", updated.mode)
	}
	if updated.selected == nil || updated.selected.ID != "PRJ-2" {
		t.Fatalf("selected = %#v, want PRJ-2", updated.selected)
	}
}

func TestNewEditFormDescriptionHeightIsLarger(t *testing.T) {
	f := newEditForm(store.Ticket{})
	if f.desc.Height() != 8 {
		t.Fatalf("description height = %d, want 8", f.desc.Height())
	}
}

func TestHandleKeyEditArrowNavigationForTextareas(t *testing.T) {
	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.width = 100
	m.mode = modeEdit
	m.form = newEditForm(store.Ticket{
		Title:       "Example",
		Description: "line1\nline2\nline3",
	})
	m.form.focus = efDesc
	m.form.applyFocus(m.width - 2)

	// Move cursor to first line in description.
	for i := 0; i < 20 && !textareaAtFirstLine(m.form.desc); i++ {
		m.form.desc, _ = m.form.desc.Update(tea.KeyMsg{Type: tea.KeyUp})
	}

	// Down from first line should stay in description (move within field).
	updatedAny, _ := m.handleKeyEdit(tea.KeyMsg{Type: tea.KeyDown})
	updated, ok := updatedAny.(Model)
	if !ok {
		t.Fatalf("handleKeyEdit type = %T, want Model", updatedAny)
	}
	if updated.form.focus != efDesc {
		t.Fatalf("focus after first down inside description = %d, want %d", updated.form.focus, efDesc)
	}

	// Move down until the last line while staying in description.
	for i := 0; i < 20 && !textareaAtLastLine(updated.form.desc); i++ {
		updatedAny, _ = updated.handleKeyEdit(tea.KeyMsg{Type: tea.KeyDown})
		updated, ok = updatedAny.(Model)
		if !ok {
			t.Fatalf("handleKeyEdit type = %T, want Model", updatedAny)
		}
		if updated.form.focus != efDesc {
			t.Fatalf("focus while moving within description = %d, want %d", updated.form.focus, efDesc)
		}
	}
	if !textareaAtLastLine(updated.form.desc) {
		t.Fatalf("description cursor did not reach last line")
	}

	// One more down at last line should move to next field.
	updatedAny, _ = updated.handleKeyEdit(tea.KeyMsg{Type: tea.KeyDown})
	updated, ok = updatedAny.(Model)
	if !ok {
		t.Fatalf("handleKeyEdit type = %T, want Model", updatedAny)
	}
	if updated.form.focus != efAC {
		t.Fatalf("focus after down at last description line = %d, want %d", updated.form.focus, efAC)
	}

	// From the first line, up should jump to the previous field.
	updated.form = newEditForm(store.Ticket{
		Title:       "Example",
		Description: "line1\nline2\nline3",
	})
	updated.form.focus = efDesc
	updated.form.applyFocus(updated.width - 2)
	for i := 0; i < 20 && !textareaAtFirstLine(updated.form.desc); i++ {
		updated.form.desc, _ = updated.form.desc.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	updatedAny, _ = updated.handleKeyEdit(tea.KeyMsg{Type: tea.KeyUp})
	updated, ok = updatedAny.(Model)
	if !ok {
		t.Fatalf("handleKeyEdit type = %T, want Model", updatedAny)
	}
	if updated.form.focus != efTitle {
		t.Fatalf("focus after up at first description line = %d, want %d", updated.form.focus, efTitle)
	}
}

func TestHandleKeyCtrlCQuitsImmediatelyInEditMode(t *testing.T) {
	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.mode = modeEdit

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c in edit mode should return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command result = %T, want tea.QuitMsg", cmd())
	}
}

func TestEditDescriptionBlockHeightIsFixedAcrossFocus(t *testing.T) {
	m := newModel(nil, config.Config{}, Themes[ThemeTheGrey])
	m.width = 100
	m.height = 40
	m.form = newEditForm(store.Ticket{
		Title:       "Example",
		Description: "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9",
	})

	// Unfocused description
	m.form.focus = efTitle
	unfocusedLines := m.viewForm("Edit Ticket")
	unfocusedGap := fieldGapLines(unfocusedLines, "description:", "acceptance:")
	if unfocusedGap != m.form.desc.Height()+1 {
		t.Fatalf("unfocused description span = %d, want %d", unfocusedGap, m.form.desc.Height()+1)
	}

	// Focused description
	m.form.focus = efDesc
	focusedLines := m.viewForm("Edit Ticket")
	focusedGap := fieldGapLines(focusedLines, "description:", "acceptance:")
	if focusedGap != m.form.desc.Height()+1 {
		t.Fatalf("focused description span = %d, want %d", focusedGap, m.form.desc.Height()+1)
	}
}

func fieldGapLines(lines []string, label, nextLabel string) int {
	start := -1
	end := -1
	for i, line := range lines {
		text := stripANSI(line)
		if start == -1 && strings.Contains(text, label) {
			start = i
		}
		if strings.Contains(text, nextLabel) {
			end = i
			break
		}
	}
	if start == -1 || end == -1 || end <= start {
		return 0
	}
	return end - start - 1
}
