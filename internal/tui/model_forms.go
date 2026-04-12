package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
	"strings"
)

// ─── edit form ───────────────────────────────────────────────────────────────

const (
	efTitle  = 0
	efDesc   = 1
	efAC     = 2
	efType   = 3
	efStatus = 4
	efSave   = 5
	efCount  = 6
)

type editForm struct {
	title      textinput.Model
	desc       textarea.Model
	acceptCrit textarea.Model
	ticketType string
	status     string
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
	desc.SetHeight(4)
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
	pfTitle = 0
	pfDesc  = 1
	pfDoR   = 2
	pfDoD   = 3
	pfSave  = 4
	pfCount = 5
)

type projectEditForm struct {
	project store.Project
	title   textinput.Model
	desc    textarea.Model
	dor     textarea.Model // Definition of Ready (acceptance_criteria)
	dod     textarea.Model // Definition of Done (notes)
	focus   int
}

func newProjectEditForm(p store.Project) *projectEditForm {
	ti := textinput.New()
	ti.SetValue(p.Title)
	ti.CharLimit = 200
	ti.Focus()

	desc := textarea.New()
	desc.SetValue(p.Description)
	desc.Placeholder = "project description..."
	desc.SetHeight(3)
	desc.ShowLineNumbers = false
	desc.CharLimit = 2000

	dor := textarea.New()
	dor.SetValue(p.AcceptanceCriteria)
	dor.Placeholder = "definition of ready..."
	dor.SetHeight(3)
	dor.ShowLineNumbers = false
	dor.CharLimit = 2000

	dod := textarea.New()
	dod.SetValue(p.Notes)
	dod.Placeholder = "definition of done..."
	dod.SetHeight(3)
	dod.ShowLineNumbers = false
	dod.CharLimit = 2000

	return &projectEditForm{
		project: p,
		title:   ti,
		desc:    desc,
		dor:     dor,
		dod:     dod,
		focus:   pfTitle,
	}
}

func (f *projectEditForm) applyFocus(w int) {
	f.desc.SetWidth(w - 4)
	f.dor.SetWidth(w - 4)
	f.dod.SetWidth(w - 4)
	f.title.Blur()
	f.desc.Blur()
	f.dor.Blur()
	f.dod.Blur()
	switch f.focus {
	case pfTitle:
		f.title.Focus()
	case pfDesc:
		f.desc.Focus()
	case pfDoR:
		f.dor.Focus()
	case pfDoD:
		f.dod.Focus()
	}
}

func (f *projectEditForm) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch f.focus {
	case pfTitle:
		f.title, cmd = f.title.Update(msg)
	case pfDesc:
		f.desc, cmd = f.desc.Update(msg)
	case pfDoR:
		f.dor, cmd = f.dor.Update(msg)
	case pfDoD:
		f.dod, cmd = f.dod.Update(msg)
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
	nfTitle = 0
	nfType  = 1
	nfDesc  = 2
	nfAC    = 3
	nfState = 4
	nfStage = 5
	nfSave  = 6
	nfCount = 7
)

var ticketTypes = []string{"task", "epic", "bug", "spike", "chore", "note", "question", "requirement", "decision"}
var ticketStates = []string{"idle", "active", "success", "fail"}
var ticketStages = []string{"", "design", "develop", "test", "done"}

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
		state:      "idle",
		stage:      "design",
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
		Status:             m.form.status,
	}
	svc := m.svc
	cfg := m.cfg
	return func() tea.Msg {
		_, err := svc.UpdateTicket(id, req)
		if err != nil {
			return errMsg{err}
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
	return func() tea.Msg {
		t, err := svc.CreateTicket(req)
		if err != nil {
			return errMsg{err}
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
	if f.focus == efDesc {
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
