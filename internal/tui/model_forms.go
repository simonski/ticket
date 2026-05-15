package tui

import (
	"context"
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
	"strconv"
	"strings"
)

// ─── edit form ───────────────────────────────────────────────────────────────

const (
	efTitle    = 0
	efDesc     = 1
	efAC       = 2
	efType     = 3
	efStatus   = 4
	efDraft    = 5
	efWorkflow = 6
	efSave     = 7
	efCount    = 8
)

type editForm struct {
	title      textinput.Model
	desc       textarea.Model
	acceptCrit textarea.Model
	ticketType string
	status     string
	draft      bool
	workflowID *int64
	focus      int
	picker     *pickerPopup
}

func newEditForm(t store.Ticket) editForm {
	ti := textinput.New()
	ti.SetValue(t.Title)
	ti.CharLimit = 200
	ti.Focus()

	desc := textarea.New()
	desc.SetValue(t.Description)
	desc.Placeholder = "describe the ticket..."
	desc.SetHeight(8)
	desc.ShowLineNumbers = false
	desc.CharLimit = 2000

	ac := textarea.New()
	ac.SetValue(t.AcceptanceCriteria)
	ac.Placeholder = "acceptance criteria..."
	ac.SetHeight(3)
	ac.ShowLineNumbers = false
	ac.CharLimit = 2000

	return editForm{
		title:      ti,
		desc:       desc,
		acceptCrit: ac,
		ticketType: t.Type,
		status:     t.Status,
		draft:      t.Draft,
		workflowID: t.WorkflowID,
		focus:      efTitle,
	}
}

func (f *editForm) applyFocus(w int) {
	f.desc.SetWidth(w - 4)
	f.acceptCrit.SetWidth(w - 4)
	f.title.Blur()
	f.desc.Blur()
	f.acceptCrit.Blur()
	switch f.focus {
	case efTitle:
		f.title.Focus()
	case efDesc:
		f.desc.Focus()
	case efAC:
		f.acceptCrit.Focus()
	}
}

func (f *editForm) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch f.focus {
	case efTitle:
		f.title, cmd = f.title.Update(msg)
	case efDesc:
		f.desc, cmd = f.desc.Update(msg)
	case efAC:
		f.acceptCrit, cmd = f.acceptCrit.Update(msg)
	}
	return cmd
}

func (f *editForm) nextField() {
	f.focus = (f.focus + 1) % efCount
}

func (f *editForm) prevField() {
	f.focus = (f.focus - 1 + efCount) % efCount
}

// ─── project edit form ───────────────────────────────────────────────────────

const (
	pfTitle        = 0
	pfVisibility   = 1
	pfDefaultDraft = 2
	pfWorkflow     = 3
	pfRepo         = 4
	pfDesc         = 5
	pfDoR          = 6
	pfDoD          = 7
	pfAC           = 8
	pfSave         = 9
	pfCount        = 10
)

type projectEditForm struct {
	project      store.Project
	title        textinput.Model
	visibility   string
	defaultDraft bool
	workflowID   *int64
	gitRepo      textinput.Model
	desc         textarea.Model
	dor          textarea.Model
	dod          textarea.Model
	ac           textarea.Model
	focus        int
	picker       *pickerPopup
}

func newProjectEditForm(p store.Project) *projectEditForm {
	ti := textinput.New()
	ti.SetValue(p.Title)
	ti.CharLimit = 200
	ti.Focus()

	repo := textinput.New()
	repo.SetValue(p.GitRepository)
	repo.Placeholder = "git repository..."
	repo.CharLimit = 500

	desc := textarea.New()
	desc.SetValue(p.Description)
	desc.Placeholder = "project description..."
	desc.SetHeight(3)
	desc.ShowLineNumbers = false
	desc.CharLimit = 2000

	dor := textarea.New()
	dor.SetValue(guidanceDefaultValue(p.DORMap))
	dor.Placeholder = "definition of ready..."
	dor.SetHeight(3)
	dor.ShowLineNumbers = false
	dor.CharLimit = 2000

	dod := textarea.New()
	dod.SetValue(guidanceDefaultValue(p.DODMap))
	dod.Placeholder = "definition of done..."
	dod.SetHeight(3)
	dod.ShowLineNumbers = false
	dod.CharLimit = 2000

	ac := textarea.New()
	ac.SetValue(guidanceDefaultValue(p.ACMap))
	ac.Placeholder = "acceptance criteria..."
	ac.SetHeight(3)
	ac.ShowLineNumbers = false
	ac.CharLimit = 2000

	return &projectEditForm{
		project:      p,
		title:        ti,
		visibility:   p.Visibility,
		defaultDraft: p.DefaultDraft,
		workflowID:   p.WorkflowID,
		gitRepo:      repo,
		desc:         desc,
		dor:          dor,
		dod:          dod,
		ac:           ac,
		focus:        pfTitle,
	}
}

func (f *projectEditForm) applyFocus(w int) {
	f.desc.SetWidth(w - 4)
	f.dor.SetWidth(w - 4)
	f.dod.SetWidth(w - 4)
	f.ac.SetWidth(w - 4)
	f.title.Blur()
	f.gitRepo.Blur()
	f.desc.Blur()
	f.dor.Blur()
	f.dod.Blur()
	f.ac.Blur()
	switch f.focus {
	case pfTitle:
		f.title.Focus()
	case pfRepo:
		f.gitRepo.Focus()
	case pfDesc:
		f.desc.Focus()
	case pfDoR:
		f.dor.Focus()
	case pfDoD:
		f.dod.Focus()
	case pfAC:
		f.ac.Focus()
	}
}

func (f *projectEditForm) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch f.focus {
	case pfTitle:
		f.title, cmd = f.title.Update(msg)
	case pfRepo:
		f.gitRepo, cmd = f.gitRepo.Update(msg)
	case pfDesc:
		f.desc, cmd = f.desc.Update(msg)
	case pfDoR:
		f.dor, cmd = f.dor.Update(msg)
	case pfDoD:
		f.dod, cmd = f.dod.Update(msg)
	case pfAC:
		f.ac, cmd = f.ac.Update(msg)
	}
	return cmd
}

func (f *projectEditForm) nextField() {
	f.focus = (f.focus + 1) % pfCount
}

func (f *projectEditForm) prevField() {
	f.focus = (f.focus - 1 + pfCount) % pfCount
}

// ─── new ticket form ─────────────────────────────────────────────────────────

const (
	nfTitle    = 0
	nfType     = 1
	nfDesc     = 2
	nfAC       = 3
	nfState    = 4
	nfStage    = 5
	nfDraft    = 6
	nfWorkflow = 7
	nfSave     = 8
	nfCount    = 9
)

var ticketTypes = []string{"task", "epic", "bug", "spike", "chore", "note", "question", "requirement", "decision"}
var ticketStates = []string{store.StateIdle, store.StateActive, store.StateSuccess, store.StateFail}
var ticketStages = []string{"", store.StageDesign, store.StageDevelop, store.StageTest, store.StageDone}
var projectVisibilities = []string{store.ProjectVisibilityPrivate, store.ProjectVisibilityPublic}
var boolPickerItems = []string{"false", "true"}

type pickerPopup struct {
	items    []string
	cursor   int
	forField string
}

type newTicketForm struct {
	title      textinput.Model
	desc       textarea.Model
	acceptCrit textarea.Model
	ticketType string
	state      string
	stage      string
	draft      bool
	workflowID *int64
	focus      int
	picker     *pickerPopup
}

func makeNewTicketForm() *newTicketForm {
	ti := textinput.New()
	ti.Placeholder = "ticket title..."
	ti.CharLimit = 200
	ti.Focus()

	desc := textarea.New()
	desc.Placeholder = "describe the ticket..."
	desc.SetHeight(4)
	desc.ShowLineNumbers = false
	desc.CharLimit = 2000

	ac := textarea.New()
	ac.Placeholder = "acceptance criteria..."
	ac.SetHeight(3)
	ac.ShowLineNumbers = false
	ac.CharLimit = 2000

	return &newTicketForm{
		title:      ti,
		desc:       desc,
		acceptCrit: ac,
		ticketType: "task",
		state:      store.StateIdle,
		stage:      store.StageDesign,
		draft:      false,
		focus:      nfTitle,
	}
}

func (f *newTicketForm) applyFocus(w int) {
	f.desc.SetWidth(w - 4)
	f.acceptCrit.SetWidth(w - 4)
	f.title.Blur()
	f.desc.Blur()
	f.acceptCrit.Blur()
	switch f.focus {
	case nfTitle:
		f.title.Focus()
	case nfDesc:
		f.desc.Focus()
	case nfAC:
		f.acceptCrit.Focus()
	}
}

func indexOf(haystack []string, needle string) int {
	for i, v := range haystack {
		if v == needle {
			return i
		}
	}
	return 0
}

func (m Model) handleKeyEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.form
	key := msg.String()

	// Picker overlay absorbs keys
	if f.picker != nil {
		switch key {
		case "esc":
			f.picker = nil
		case "up", "k":
			if f.picker.cursor > 0 {
				f.picker.cursor--
			}
		case "down", "j":
			if f.picker.cursor < len(f.picker.items)-1 {
				f.picker.cursor++
			}
		case "enter", " ":
			val := f.picker.items[f.picker.cursor]
			switch f.picker.forField {
			case "type":
				f.ticketType = val
			case "status":
				f.status = val
			case "draft":
				f.draft = parseBoolPickerValue(val)
			case "workflow":
				f.workflowID = parseWorkflowPickerValue(val)
			}
			f.picker = nil
		}
		return m, nil
	}

	switch key {
	case "esc":
		return m.goBack()
	case "ctrl+s", "ctrl+d":
		return m, m.saveTicket()
	case "up", "k", "w":
		switch f.focus {
		case efDesc:
			if !textareaAtFirstLine(f.desc) {
				f.desc, _ = f.desc.Update(msg)
				return m, nil
			}
		case efAC:
			if !textareaAtFirstLine(f.acceptCrit) {
				f.acceptCrit, _ = f.acceptCrit.Update(msg)
				return m, nil
			}
		}
		f.prevField()
		f.applyFocus(m.width - 2)
	case "down", "j", "s":
		switch f.focus {
		case efDesc:
			if !textareaAtLastLine(f.desc) {
				f.desc, _ = f.desc.Update(msg)
				return m, nil
			}
		case efAC:
			if !textareaAtLastLine(f.acceptCrit) {
				f.acceptCrit, _ = f.acceptCrit.Update(msg)
				return m, nil
			}
		}
		f.nextField()
		f.applyFocus(m.width - 2)
	case "tab":
		f.nextField()
		f.applyFocus(m.width - 2)
	case "shift+tab":
		f.prevField()
		f.applyFocus(m.width - 2)
	case "enter", " ":
		// Space must reach text input fields, not trigger picker/save actions
		if key == " " && (f.focus == efTitle || f.focus == efDesc || f.focus == efAC) {
			cmd := f.update(msg)
			return m, cmd
		}
		switch f.focus {
		case efType:
			f.picker = &pickerPopup{items: ticketTypes, cursor: indexOf(ticketTypes, f.ticketType), forField: "type"}
		case efStatus:
			statuses := []string{"design/idle", "design/active", "develop/idle", "develop/active", "test/idle", "test/active", "done/success", "done/fail"}
			f.picker = &pickerPopup{items: statuses, cursor: indexOf(statuses, f.status), forField: "status"}
		case efDraft:
			f.picker = &pickerPopup{items: boolPickerItems, cursor: boolPickerCursor(f.draft), forField: "draft"}
		case efWorkflow:
			items := ticketWorkflowPickerItems(m.workflows)
			f.picker = &pickerPopup{items: items, cursor: workflowPickerCursor(items, f.workflowID), forField: "workflow"}
		case efSave:
			return m, m.saveTicket()
		}
	default:
		if f.focus == efTitle || f.focus == efDesc || f.focus == efAC {
			cmd := f.update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func textareaAtFirstLine(t textarea.Model) bool {
	return t.Line() <= 0
}

func textareaAtLastLine(t textarea.Model) bool {
	return t.Line() >= t.LineCount()-1
}

func fixedBlockLines(lines []string, height int) []string {
	if height < 1 {
		height = 1
	}
	if len(lines) > height {
		return lines[:height]
	}
	if len(lines) < height {
		padded := make([]string, len(lines), height)
		copy(padded, lines)
		for len(padded) < height {
			padded = append(padded, "")
		}
		return padded
	}
	return lines
}

func (m Model) handleKeyNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.newForm == nil {
		m.mode = modeList
		return m, nil
	}
	f := m.newForm
	key := msg.String()

	// Picker overlay absorbs keys
	if f.picker != nil {
		switch key {
		case "esc":
			f.picker = nil
		case "up", "k":
			if f.picker.cursor > 0 {
				f.picker.cursor--
			}
		case "down", "j":
			if f.picker.cursor < len(f.picker.items)-1 {
				f.picker.cursor++
			}
		case "enter", " ":
			val := f.picker.items[f.picker.cursor]
			switch f.picker.forField {
			case "type":
				f.ticketType = val
			case "state":
				f.state = val
			case "stage":
				f.stage = val
			case "draft":
				f.draft = parseBoolPickerValue(val)
			case "workflow":
				f.workflowID = parseWorkflowPickerValue(val)
			}
			f.picker = nil
		}
		m.newForm = f
		return m, nil
	}

	switch key {
	case "esc":
		m.mode = modeList
		m.newForm = nil
		return m, nil
	case "ctrl+s":
		return m, m.createTicket()
	case "tab":
		f.focus = (f.focus + 1) % nfCount
		f.applyFocus(m.width - 2)
	case "shift+tab":
		f.focus = (f.focus - 1 + nfCount) % nfCount
		f.applyFocus(m.width - 2)
	case "enter":
		switch f.focus {
		case nfType:
			f.picker = &pickerPopup{items: ticketTypes, cursor: indexOf(ticketTypes, f.ticketType), forField: "type"}
		case nfState:
			f.picker = &pickerPopup{items: ticketStates, cursor: indexOf(ticketStates, f.state), forField: "state"}
		case nfStage:
			f.picker = &pickerPopup{items: ticketStages, cursor: indexOf(ticketStages, f.stage), forField: "stage"}
		case nfDraft:
			f.picker = &pickerPopup{items: boolPickerItems, cursor: boolPickerCursor(f.draft), forField: "draft"}
		case nfWorkflow:
			items := ticketWorkflowPickerItems(m.workflows)
			f.picker = &pickerPopup{items: items, cursor: workflowPickerCursor(items, f.workflowID), forField: "workflow"}
		case nfSave:
			return m, m.createTicket()
		default:
			// pass enter to focused textarea
			var cmd tea.Cmd
			switch f.focus {
			case nfDesc:
				f.desc, cmd = f.desc.Update(msg)
			case nfAC:
				f.acceptCrit, cmd = f.acceptCrit.Update(msg)
			}
			m.newForm = f
			return m, cmd
		}
	default:
		var cmd tea.Cmd
		switch f.focus {
		case nfTitle:
			f.title, cmd = f.title.Update(msg)
		case nfDesc:
			f.desc, cmd = f.desc.Update(msg)
		case nfAC:
			f.acceptCrit, cmd = f.acceptCrit.Update(msg)
		}
		m.newForm = f
		return m, cmd
	}
	m.newForm = f
	return m, nil
}

func (m Model) saveTicket() tea.Cmd {
	if m.selected == nil {
		return nil
	}
	id := m.selected.ID
	req := libticket.TicketUpdateRequest{
		Title:              m.form.title.Value(),
		Description:        m.form.desc.Value(),
		AcceptanceCriteria: m.form.acceptCrit.Value(),
		Type:               m.form.ticketType,
		Status:             m.form.status,
	}
	svc := m.svc
	cfg := m.cfg
	wasDraft := m.selected.Draft
	prevWorkflowID := m.selected.WorkflowID
	nextDraft := m.form.draft
	nextWorkflowID := m.form.workflowID
	return func() tea.Msg {
		_, err := svc.UpdateTicket(context.Background(), id, req)
		if err != nil {
			return errMsg{err}
		}
		if nextDraft != wasDraft {
			if nextDraft {
				_, err = svc.DraftTicket(context.Background(), id, "")
			} else {
				_, err = svc.UndraftTicket(context.Background(), id, "")
			}
			if err != nil {
				return errMsg{err}
			}
		}
		if !equalOptionalInt64(prevWorkflowID, nextWorkflowID) {
			if nextWorkflowID == nil {
				_, err = svc.UnsetTicketWorkflow(context.Background(), id)
			} else {
				_, err = svc.SetTicketWorkflow(context.Background(), id, *nextWorkflowID)
			}
			if err != nil {
				return errMsg{err}
			}
		}
		return loadTicketsSync(svc, cfg)
	}
}

// createTicket creates a new ticket and reloads.
func (m Model) createTicket() tea.Cmd {
	if m.newForm == nil {
		return nil
	}
	f := m.newForm
	title := strings.TrimSpace(f.title.Value())
	if title == "" {
		return nil
	}
	svc := m.svc
	projectID := m.project.ID
	req := libticket.TicketCreateRequest{
		ProjectID:          projectID,
		Type:               f.ticketType,
		Title:              title,
		Description:        f.desc.Value(),
		AcceptanceCriteria: f.acceptCrit.Value(),
		State:              f.state,
		Stage:              f.stage,
	}
	nextDraft := f.draft
	nextWorkflowID := f.workflowID
	return func() tea.Msg {
		t, err := svc.CreateTicket(context.Background(), req)
		if err != nil {
			return errMsg{err}
		}
		if nextDraft {
			t, err = svc.DraftTicket(context.Background(), t.ID, "")
			if err != nil {
				return errMsg{err}
			}
		}
		if nextWorkflowID != nil {
			t, err = svc.SetTicketWorkflow(context.Background(), t.ID, *nextWorkflowID)
			if err != nil {
				return errMsg{err}
			}
		}
		return ticketCreatedMsg(t)
	}
}

// ─── edit / new view ─────────────────────────────────────────────────────────

func (m Model) viewEdit() []string {
	title := " edit ticket"
	if m.selected != nil {
		title = " edit  " + m.selected.ID
	}
	return m.viewForm(title)
}

func (m Model) viewNew() []string {
	if m.newForm == nil {
		return []string{"no form"}
	}
	return m.viewNewTicket()
}

func (m Model) viewNewTicket() []string {
	f := m.newForm
	th := m.theme
	inner := m.width - 2

	f.applyFocus(inner)

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	activeLabel := lipgloss.NewStyle().Foreground(th.Accent).Background(th.SelBg).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
	pickerHint := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)

	field := func(idx int, name, val string) string {
		lbl := fmt.Sprintf("  %-14s", name+":")
		if idx == f.focus {
			return activeLabel.Render(lbl) + valStyle.Render(" "+val) + pickerHint.Render(" ↵ pick")
		}
		return labelStyle.Render(lbl) + valStyle.Render(" "+val)
	}

	var lines []string
	lines = append(lines, headerStyle.Render(padRight(" new ticket", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	lines = append(lines, "")

	// Title
	titleLbl := fmt.Sprintf("  %-14s", "title:")
	if f.focus == nfTitle {
		lines = append(lines, activeLabel.Render(titleLbl)+f.title.View())
	} else {
		lines = append(lines, labelStyle.Render(titleLbl)+valStyle.Render(" "+f.title.Value()))
	}
	lines = append(lines, "")

	// Type
	lines = append(lines, field(nfType, "type", f.ticketType))
	lines = append(lines, "")

	// Description textarea
	descLbl := fmt.Sprintf("  %-14s", "description:")
	if f.focus == nfDesc {
		lines = append(lines, activeLabel.Render(descLbl))
		for _, tl := range strings.Split(f.desc.View(), "\n") {
			lines = append(lines, "  "+tl)
		}
	} else {
		lines = append(lines, labelStyle.Render(descLbl))
		descVal := f.desc.Value()
		if descVal == "" {
			descVal = "(empty)"
		}
		for _, dl := range wordWrap(descVal, inner-4) {
			lines = append(lines, valStyle.Render("  "+dl))
		}
	}
	lines = append(lines, "")

	// Acceptance criteria textarea
	acLbl := fmt.Sprintf("  %-14s", "acceptance:")
	if f.focus == nfAC {
		lines = append(lines, activeLabel.Render(acLbl))
		for _, tl := range strings.Split(f.acceptCrit.View(), "\n") {
			lines = append(lines, "  "+tl)
		}
	} else {
		lines = append(lines, labelStyle.Render(acLbl))
		acVal := f.acceptCrit.Value()
		if acVal == "" {
			acVal = "(empty)"
		}
		for _, dl := range wordWrap(acVal, inner-4) {
			lines = append(lines, valStyle.Render("  "+dl))
		}
	}
	lines = append(lines, "")

	// State / Stage
	lines = append(lines, field(nfState, "state", f.state))
	lines = append(lines, "")
	lines = append(lines, field(nfStage, "stage", f.stage))
	lines = append(lines, "")
	lines = append(lines, field(nfDraft, "draft", formatBoolPickerValue(f.draft)))
	lines = append(lines, "")
	lines = append(lines, field(nfWorkflow, "workflow", formatTicketWorkflowChoice(f.workflowID, m.workflows)))
	lines = append(lines, "")

	// Save button
	saveStr := "  [ Save ]"
	if f.focus == nfSave {
		lines = append(lines, lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true).Render(padRight(saveStr, inner)))
	} else {
		lines = append(lines, valStyle.Render(padRight(saveStr, inner)))
	}

	// Overlay picker popup if open
	if f.picker != nil {
		lines = m.overlayPickerOnLines(lines, f.picker, inner)
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

func (m Model) overlayPickerOnLines(lines []string, p *pickerPopup, inner int) []string {
	th := m.theme
	popW := 24
	popH := len(p.items) + 2
	startRow := 2
	startCol := (inner - popW) / 2
	if startCol < 0 {
		startCol = 0
	}

	result := make([]string, len(lines))
	copy(result, lines)

	borderStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.SelBg)
	itemStyle := lipgloss.NewStyle().Foreground(th.Fg).Background(th.SelBg)
	cursorStyle := lipgloss.NewStyle().Foreground(th.SelFg).Background(th.Accent).Bold(true)

	for i := 0; i < popH && startRow+i < len(result); i++ {
		var cell string
		row := startRow + i
		if i == 0 || i == popH-1 {
			cell = borderStyle.Render(padRight("", popW))
		} else {
			itemIdx := i - 1
			if itemIdx < len(p.items) {
				label := p.items[itemIdx]
				if label == "" {
					label = "(none)"
				}
				line := fmt.Sprintf("  %-20s", label)
				if itemIdx == p.cursor {
					cell = cursorStyle.Render(padRight(line, popW))
				} else {
					cell = itemStyle.Render(padRight(line, popW))
				}
			}
		}
		plain := padRight(stripANSI(result[row]), inner)
		runes := []rune(plain)
		prefix := ""
		if startCol > 0 && startCol <= len(runes) {
			prefix = string(runes[:startCol])
		}
		suffix := ""
		if startCol+popW < len(runes) {
			suffix = string(runes[startCol+popW:])
		}
		result[row] = lipgloss.NewStyle().Background(th.Bg).Render(prefix) + cell + lipgloss.NewStyle().Background(th.Bg).Render(suffix)
	}
	return result
}

func (m Model) viewForm(title string) []string {
	f := &m.form
	th := m.theme
	inner := m.width - 2

	f.applyFocus(inner)

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	activeLabel := lipgloss.NewStyle().Foreground(th.Accent).Background(th.SelBg).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
	pickerHint := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)

	field := func(idx int, name, val string) string {
		lbl := fmt.Sprintf("  %-14s", name+":")
		if idx == f.focus {
			return activeLabel.Render(lbl) + valStyle.Render(" "+val) + pickerHint.Render(" ↵ pick")
		}
		return labelStyle.Render(lbl) + valStyle.Render(" "+val)
	}

	var lines []string
	lines = append(lines, headerStyle.Render(padRight(title, inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	lines = append(lines, "")

	// Title
	titleLbl := fmt.Sprintf("  %-14s", "title:")
	if f.focus == efTitle {
		lines = append(lines, activeLabel.Render(titleLbl)+f.title.View())
	} else {
		lines = append(lines, labelStyle.Render(titleLbl)+valStyle.Render(" "+f.title.Value()))
	}
	lines = append(lines, "")

	// Description textarea
	descLbl := fmt.Sprintf("  %-14s", "description:")
	focusedDesc := f.focus == efDesc
	if focusedDesc {
		lines = append(lines, activeLabel.Render(descLbl))
	} else {
		lines = append(lines, labelStyle.Render(descLbl))
	}
	descLines := strings.Split(f.desc.View(), "\n")
	if !focusedDesc {
		descVal := f.desc.Value()
		if descVal == "" {
			descVal = "(empty)"
		}
		descLines = wordWrap(descVal, inner-4)
	}
	descLines = fixedBlockLines(descLines, f.desc.Height())
	for _, dl := range descLines {
		if focusedDesc {
			lines = append(lines, "  "+dl)
		} else {
			lines = append(lines, valStyle.Render("  "+dl))
		}
	}
	lines = append(lines, "")

	// Acceptance criteria textarea
	acLbl := fmt.Sprintf("  %-14s", "acceptance:")
	if f.focus == efAC {
		lines = append(lines, activeLabel.Render(acLbl))
		for _, tl := range strings.Split(f.acceptCrit.View(), "\n") {
			lines = append(lines, "  "+tl)
		}
	} else {
		lines = append(lines, labelStyle.Render(acLbl))
		acVal := f.acceptCrit.Value()
		if acVal == "" {
			acVal = "(empty)"
		}
		for _, dl := range wordWrap(acVal, inner-4) {
			lines = append(lines, valStyle.Render("  "+dl))
		}
	}
	lines = append(lines, "")

	// Type
	lines = append(lines, field(efType, "type", f.ticketType))
	lines = append(lines, "")

	// Status
	lines = append(lines, field(efStatus, "status", f.status))
	lines = append(lines, "")

	lines = append(lines, field(efDraft, "draft", formatBoolPickerValue(f.draft)))
	lines = append(lines, "")

	lines = append(lines, field(efWorkflow, "workflow", formatTicketWorkflowChoice(f.workflowID, m.workflows)))
	lines = append(lines, "")

	// Save button
	saveStr := "  [ Save ]"
	if f.focus == efSave {
		lines = append(lines, lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true).Render(padRight(saveStr, inner)))
	} else {
		lines = append(lines, valStyle.Render(padRight(saveStr, inner)))
	}

	// Overlay picker popup if open
	if f.picker != nil {
		lines = m.overlayPickerOnLines(lines, f.picker, inner)
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

func formatBoolPickerValue(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func parseBoolPickerValue(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func boolPickerCursor(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ticketWorkflowPickerItems(workflows []store.WorkflowWithStages) []string {
	items := []string{"(inherit project default)"}
	for _, workflow := range workflows {
		items = append(items, fmt.Sprintf("%d %s", workflow.ID, workflow.Name))
	}
	return items
}

func projectWorkflowPickerItems(workflows []store.WorkflowWithStages) []string {
	items := []string{"(none)"}
	for _, workflow := range workflows {
		items = append(items, fmt.Sprintf("%d %s", workflow.ID, workflow.Name))
	}
	return items
}

func workflowPickerCursor(items []string, selected *int64) int {
	if selected == nil {
		return 0
	}
	prefix := strconv.FormatInt(*selected, 10) + " "
	for idx, item := range items {
		if strings.HasPrefix(item, prefix) {
			return idx
		}
	}
	return 0
}

func parseWorkflowPickerValue(value string) *int64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "(") {
		return nil
	}
	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return nil
	}
	id, err := strconv.ParseInt(tokens[0], 10, 64)
	if err != nil {
		return nil
	}
	return &id
}

func formatTicketWorkflowChoice(id *int64, workflows []store.WorkflowWithStages) string {
	if id == nil {
		return "(inherit project default)"
	}
	return formatNamedWorkflowChoice(*id, workflows)
}

func formatProjectWorkflowChoice(id *int64, workflows []store.WorkflowWithStages) string {
	if id == nil {
		return "(none)"
	}
	return formatNamedWorkflowChoice(*id, workflows)
}

func formatNamedWorkflowChoice(id int64, workflows []store.WorkflowWithStages) string {
	for _, workflow := range workflows {
		if workflow.ID == id {
			return fmt.Sprintf("%d %s", workflow.ID, workflow.Name)
		}
	}
	return strconv.FormatInt(id, 10)
}

func guidanceDefaultValue(values store.GuidanceMap) string {
	if values == nil {
		return ""
	}
	return values[store.DefaultGuidanceStageKey]
}

func setDefaultGuidanceValue(values store.GuidanceMap, value string) store.GuidanceMap {
	trimmed := strings.TrimSpace(value)
	next := make(store.GuidanceMap)
	for key, entry := range values {
		if key == store.DefaultGuidanceStageKey {
			continue
		}
		next[key] = entry
	}
	if trimmed != "" {
		next[store.DefaultGuidanceStageKey] = trimmed
	}
	if len(next) == 0 {
		return nil
	}
	return next
}

func equalOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
