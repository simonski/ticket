package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

// moonPhases cycles the status-bar icon slowly.
var moonPhases = []string{"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"}

// ─── view modes ─────────────────────────────────────────────────────────────

type viewMode int

const (
	modeIntro viewMode = iota
	modeSummary
	modeList
	modeDetail
	modeEdit
	modeNew
	modeSettings
	modeProjectPicker
	modeProjects // new: inline projects panel (replaces the modal modeProjectPicker)
	modeIdeas    // new: list of top-level non-epic tickets (m.toplevel)
)

// tabModes are the top-level panels cycled by tab: Home > Projects > Ideas > Epics > Config.
var tabModes = []viewMode{modeSummary, modeProjects, modeIdeas, modeList, modeSettings}
var tabNames = []string{"Home", "Projects", "Ideas", "Epics", "Config"}

// ─── messages ────────────────────────────────────────────────────────────────

type tickMsg time.Time
type treeLoadedMsg struct {
	nodes    []treeNode
	toplevel []store.Ticket
}
type ticketCreatedMsg store.Ticket
type updateAvailableMsg string
type errMsg struct{ err error }
type projectsLoadedMsg []store.Project
type projectSwitchedMsg struct {
	project store.Project
	tickets treeLoadedMsg
}

// ─── tree node ───────────────────────────────────────────────────────────────

type treeNode struct {
	epic     store.Ticket
	children []store.Ticket
}

// ─── list item ───────────────────────────────────────────────────────────────

type listItem struct {
	ticket      store.Ticket
	depth       int
	hasChildren bool
	expanded    bool
}

// ─── edit form ───────────────────────────────────────────────────────────────

var editFieldNames = []string{"title", "description", "type", "status"}
var newFieldNames = []string{"title", "type"}

type editForm struct {
	inputs []textinput.Model
	focus  int
	names  []string
}

func newEditForm(t store.Ticket) editForm {
	return buildForm(editFieldNames, []string{t.Title, t.Description, t.Type, t.Status})
}

func newCreateForm() editForm {
	return buildForm(newFieldNames, []string{"", "task"})
}

func buildForm(names, vals []string) editForm {
	inputs := make([]textinput.Model, len(names))
	for i := range names {
		ti := textinput.New()
		if i < len(vals) {
			ti.SetValue(vals[i])
		}
		ti.CharLimit = 200
		if i == 0 {
			ti.Focus()
		}
		inputs[i] = ti
	}
	return editForm{inputs: inputs, focus: 0, names: names}
}

func (f *editForm) update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i := range f.inputs {
		var cmd tea.Cmd
		f.inputs[i], cmd = f.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (f *editForm) nextField() {
	f.inputs[f.focus].Blur()
	f.focus = (f.focus + 1) % len(f.inputs)
	f.inputs[f.focus].Focus()
}

func (f *editForm) prevField() {
	f.inputs[f.focus].Blur()
	f.focus = (f.focus - 1 + len(f.inputs)) % len(f.inputs)
	f.inputs[f.focus].Focus()
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

var ticketTypes  = []string{"task", "epic", "story", "bug", "requirement", "decision", "question", "note"}
var ticketStates = []string{"open", "active", "success", "fail"}
var ticketStages = []string{"", "planning", "development", "review", "done"}

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
		state:      "open",
		stage:      "",
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

// ─── main model ──────────────────────────────────────────────────────────────

type Model struct {
	svc     libticket.Service
	cfg     config.Config
	project store.Project

	mode   viewMode
	width  int
	height int
	theme  Theme

	// intro animation
	intro    introState
	lastTick time.Time

	// tree / list
	nodes    []treeNode
	toplevel []store.Ticket
	expanded map[int64]bool
	items    []listItem
	cursor   int
	offset   int

	// detail / edit
	selected *store.Ticket
	form     editForm
	newForm  *newTicketForm

	// settings / theme picker
	settingsCursor int

	// project picker
	projects      []store.Project
	projectCursor int

	// ideas panel
	ideasCursor int
	ideasOffset int

	// animation
	ecg       ecgState
	moonPhase int
	tickCount int

	// command input (/)
	cmdInput textinput.Model
	showCmd  bool

	// context popup (double-shift)
	showPopup  bool
	lastShift  time.Time
	shiftCount int

	// quit tracking
	lastQ     time.Time
	lastCtrlC time.Time

	// status
	statusMsg string
	updateMsg string
	err       error
}

func newModel(svc libticket.Service, cfg config.Config, th Theme) Model {
	ci := textinput.New()
	ci.Placeholder = "enter command..."
	ci.CharLimit = 200
	return Model{
		svc:      svc,
		cfg:      cfg,
		theme:    th,
		mode:     modeIntro,
		intro:    newIntroState(),
		lastTick: time.Now(),
		expanded: map[int64]bool{},
		cmdInput: ci,
		ecg:      ecgState{params: th.ECGStyle},
	}
}

// ─── tea.Model interface ─────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadTickets(m.svc, m.cfg),
		checkUpdate(m.svc),
		tickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		now := time.Time(msg)
		dt := now.Sub(m.lastTick).Seconds()
		m.lastTick = now
		m.ecg.advance(perimLen(m.width, m.height))
		if m.mode == modeIntro {
			m.intro.advance(dt)
			if m.intro.done() {
				m.mode = modeSummary
			}
		}
		// Advance moon ~every second (20 ticks @ 50ms)
		m.tickCount++
		if m.tickCount%20 == 0 {
			m.moonPhase = (m.moonPhase + 1) % len(moonPhases)
		}
		return m, tickCmd()

	case treeLoadedMsg:
		m.nodes = msg.nodes
		m.toplevel = msg.toplevel
		for _, n := range m.nodes {
			if _, set := m.expanded[n.epic.ID]; !set {
				m.expanded[n.epic.ID] = false // default: collapsed
			}
		}
		// restore persisted expand state from config
		for _, id := range m.cfg.TUIExpandedEpics {
			m.expanded[id] = true
		}
		m.items = flattenTree(m.nodes, m.toplevel, m.expanded)
		if m.cursor >= len(m.items) && len(m.items) > 0 {
			m.cursor = len(m.items) - 1
		}
		if m.statusMsg == "reloading..." {
			m.statusMsg = ""
		}

	case ticketCreatedMsg:
		m.statusMsg = "created: " + store.Ticket(msg).Key
		m.mode = modeList
		m.newForm = nil
		return m, loadTickets(m.svc, m.cfg)

	case updateAvailableMsg:
		m.updateMsg = string(msg)

	case errMsg:
		m.err = msg.err
		m.statusMsg = "error: " + msg.err.Error()

	case projectsLoadedMsg:
		m.projects = []store.Project(msg)
		// pre-select current project
		for i, p := range m.projects {
			if fmt.Sprintf("%d", p.ID) == m.cfg.CurrentProject {
				m.projectCursor = i
				break
			}
		}

	case projectSwitchedMsg:
		m.project = msg.project
		m.cfg.CurrentProject = fmt.Sprintf("%d", msg.project.ID)
		m.nodes = msg.tickets.nodes
		m.toplevel = msg.tickets.toplevel
		m.expanded = map[int64]bool{} // reset expand state for new project
		m.items = flattenTree(m.nodes, m.toplevel, m.expanded)
		m.cursor = 0
		m.offset = 0
		m.statusMsg = "switched to: " + msg.project.Title

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Intro: any key skips to list
	if m.mode == modeIntro {
		m.mode = modeList
		return m, nil
	}

	// Command input active: route there
	if m.showCmd {
		return m.handleKeyCmd(msg)
	}

	// Track double-shift for popup
	if strings.HasPrefix(key, "shift+") {
		if time.Since(m.lastShift) < 350*time.Millisecond {
			m.shiftCount++
			if m.shiftCount >= 2 {
				// Double-shift = back button
				m.shiftCount = 0
				return m.goBack()
			}
		} else {
			m.shiftCount = 1
		}
		m.lastShift = time.Now()
	} else if key != "shift" {
		m.shiftCount = 0
	}

	// Popup absorbs all keys except esc/q
	if m.showPopup {
		if key == "esc" || key == "q" {
			m.showPopup = false
		}
		return m, nil
	}

	// Double ctrl+c to quit
	if key == "ctrl+c" {
		if time.Since(m.lastCtrlC) < 500*time.Millisecond {
			return m, tea.Quit
		}
		m.lastCtrlC = time.Now()
		m.statusMsg = "ctrl+c again to quit"
		return m, nil
	}

	// Global shortcuts (not in edit/new)
	if m.mode != modeEdit && m.mode != modeNew {
		switch key {
		case "tab":
			return m.nextPanel()
		case "t":
			m.theme = Themes[NextTheme(m.theme.ID)]
			m.ecg.params = m.theme.ECGStyle
			return m, nil
		case "T":
			m.mode = modeSettings
			return m, nil
		case "?":
			if m.mode == modeSettings {
				m.mode = modeList
			} else {
				m.mode = modeSettings
			}
			return m, nil
		case "p":
			m.mode = modeProjects
			if len(m.projects) == 0 {
				return m, loadProjects(m.svc)
			}
			return m, nil
		case "/":
			m.showCmd = true
			m.cmdInput.SetValue("")
			m.cmdInput.Focus()
			return m, nil
		case "esc":
			return m.goBack()
		}
	}

	switch m.mode {
	case modeSummary:
		return m.handleKeySummary(key)
	case modeList:
		return m.handleKeyList(key)
	case modeDetail:
		return m.handleKeyDetail(key)
	case modeEdit:
		return m.handleKeyEdit(msg)
	case modeNew:
		return m.handleKeyNew(msg)
	case modeSettings:
		return m.handleKeySettings(key)
	case modeProjectPicker:
		return m.handleKeyProjectPicker(key)
	case modeProjects:
		return m.handleKeyProjects(key)
	case modeIdeas:
		return m.handleKeyIdeas(key)
	}

	return m, nil
}

func (m Model) handleKeyCmd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.showCmd = false
		m.cmdInput.Blur()
		return m, nil
	case "enter":
		input := strings.TrimSpace(m.cmdInput.Value())
		m.showCmd = false
		m.cmdInput.Blur()
		if input != "" {
			m.statusMsg = "cmd: " + input
			// Future: execute parsed commands
		}
		return m, nil
	}
	var c tea.Cmd
	m.cmdInput, c = m.cmdInput.Update(msg)
	return m, c
}

func (m Model) handleKeyList(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		if time.Since(m.lastQ) < 500*time.Millisecond {
			return m, tea.Quit
		}
		m.lastQ = time.Now()
		m.statusMsg = "press q again to quit  (or ctrl+c twice)"
	case "esc":
		m.statusMsg = ""
	case "up", "k", "w":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "down", "j", "s":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			vis := m.visibleRows()
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
	case "left", "a":
		consumed := false
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.ticket.Type == "epic" && item.hasChildren && item.expanded {
				m.expanded[item.ticket.ID] = false
				m.items = flattenTree(m.nodes, m.toplevel, m.expanded)
				consumed = true
			}
		}
		if !consumed {
			return m.prevPanel()
		}
	case "right", "d":
		consumed := false
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.ticket.Type == "epic" && item.hasChildren && !item.expanded {
				m.expanded[item.ticket.ID] = true
				m.items = flattenTree(m.nodes, m.toplevel, m.expanded)
				consumed = true
			}
		}
		if !consumed {
			return m.nextPanel()
		}
	case "home", "g":
		m.cursor = 0
		m.offset = 0
	case "end", "G":
		m.cursor = len(m.items) - 1
		vis := m.visibleRows()
		if m.cursor >= vis {
			m.offset = m.cursor - vis + 1
		}
	case "enter", " ":
		if m.cursor < len(m.items) {
			t := m.items[m.cursor].ticket
			m.selected = &t
			m.mode = modeDetail
		}
	case "e":
		if m.cursor < len(m.items) {
			t := m.items[m.cursor].ticket
			m.selected = &t
			m.form = newEditForm(t)
			m.mode = modeEdit
		}
	case "n":
		m.selected = nil
		m.newForm = makeNewTicketForm()
		m.newForm.applyFocus(m.width - 2)
		m.mode = modeNew
	case "r":
		m.statusMsg = "reloading..."
		return m, loadTickets(m.svc, m.cfg)
	}
	return m, nil
}

func (m Model) handleKeyDetail(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc", "left", "a":
		return m.goBack()
	case "e":
		if m.selected != nil {
			m.form = newEditForm(*m.selected)
			m.mode = modeEdit
		}
	case "up", "k", "w":
		if m.cursor > 0 {
			m.cursor--
			t := m.items[m.cursor].ticket
			m.selected = &t
		}
	case "down", "j", "s":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			t := m.items[m.cursor].ticket
			m.selected = &t
		}
	}
	return m, nil
}

func (m Model) handleKeyEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		return m.goBack()
	case "ctrl+s", "ctrl+d":
		return m, m.saveTicket()
	case "tab", "down":
		m.form.nextField()
	case "shift+tab", "up":
		m.form.prevField()
	default:
		cmd := m.form.update(msg)
		return m, cmd
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

func (m Model) handleKeySummary(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		if time.Since(m.lastQ) < 500*time.Millisecond {
			return m, tea.Quit
		}
		m.lastQ = time.Now()
		m.statusMsg = "press q again to quit"
	case "enter", " ", "e":
		m.mode = modeList
	case "right", "d":
		return m.nextPanel()
	case "left", "a":
		return m.prevPanel()
	case "n":
		m.selected = nil
		m.newForm = makeNewTicketForm()
		m.newForm.applyFocus(m.width - 2)
		m.mode = modeNew
	case "r":
		m.statusMsg = "reloading..."
		return m, loadTickets(m.svc, m.cfg)
	}
	return m, nil
}

func (m Model) handleKeySettings(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc":
		return m.goBack()
	case "up", "k", "w":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j", "s":
		if m.settingsCursor < len(ThemeOrder)-1 {
			m.settingsCursor++
		}
	case "enter", " ", "t":
		// Apply the highlighted theme without changing the panel
		m.theme = Themes[ThemeOrder[m.settingsCursor]]
		m.ecg.params = m.theme.ECGStyle
	case "P":
		m.cfg.TUIDisablePersist = !m.cfg.TUIDisablePersist
		if err := config.Save(m.cfg); err != nil {
			m.statusMsg = "error saving config"
		}
	case "right", "d":
		return m.nextPanel()
	case "left", "a":
		return m.prevPanel()
	}
	return m, nil
}

func (m Model) handleKeyProjectPicker(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc":
		m.mode = modeList
	case "up", "k", "w":
		if m.projectCursor > 0 {
			m.projectCursor--
		}
	case "down", "j", "s":
		if m.projectCursor < len(m.projects)-1 {
			m.projectCursor++
		}
	case "enter", " ":
		if m.projectCursor < len(m.projects) {
			chosen := m.projects[m.projectCursor]
			m.mode = modeList
			return m, m.switchProject(chosen)
		}
	}
	return m, nil
}

func (m Model) handleKeyProjects(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc":
		return m.goBack()
	case "up", "k", "w":
		if m.projectCursor > 0 {
			m.projectCursor--
		}
	case "down", "j", "s":
		if m.projectCursor < len(m.projects)-1 {
			m.projectCursor++
		}
	case "enter", " ":
		if m.projectCursor < len(m.projects) {
			chosen := m.projects[m.projectCursor]
			m.mode = modeList
			return m, m.switchProject(chosen)
		}
	case "left", "a":
		return m.prevPanel()
	case "right", "d":
		return m.nextPanel()
	}
	return m, nil
}

func (m Model) handleKeyIdeas(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc":
		return m.goBack()
	case "up", "k", "w":
		if m.ideasCursor > 0 {
			m.ideasCursor--
			if m.ideasCursor < m.ideasOffset {
				m.ideasOffset = m.ideasCursor
			}
		}
	case "down", "j", "s":
		if m.ideasCursor < len(m.toplevel)-1 {
			m.ideasCursor++
			vis := m.visibleRows()
			if m.ideasCursor >= m.ideasOffset+vis {
				m.ideasOffset = m.ideasCursor - vis + 1
			}
		}
	case "enter", " ":
		if m.ideasCursor < len(m.toplevel) {
			t := m.toplevel[m.ideasCursor]
			m.selected = &t
			m.mode = modeDetail
		}
	case "e":
		if m.ideasCursor < len(m.toplevel) {
			t := m.toplevel[m.ideasCursor]
			m.selected = &t
			m.form = newEditForm(t)
			m.mode = modeEdit
		}
	case "n":
		m.selected = nil
		m.newForm = makeNewTicketForm()
		m.newForm.applyFocus(m.width - 2)
		m.mode = modeNew
	case "left", "a":
		return m.prevPanel()
	case "right", "d":
		return m.nextPanel()
	}
	return m, nil
}

// visibleRows returns the number of ticket rows that fit in the list view.
func (m Model) visibleRows() int {
	// content height = m.height - 2 (borders)
	// header: 3 rows  status bar: 1 row  → ticket rows = m.height - 6
	v := m.height - 6
	if v < 1 {
		v = 1
	}
	return v
}

// saveTicket updates the ticket and reloads.
func (m Model) saveTicket() tea.Cmd {
	if m.selected == nil {
		return nil
	}
	id := m.selected.ID
	req := libticket.TicketUpdateRequest{
		Title:       m.form.inputs[0].Value(),
		Description: m.form.inputs[1].Value(),
		Status:      m.form.inputs[3].Value(),
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

// nextPanel cycles to the next tab in the tabModes ring.
func (m Model) nextPanel() (tea.Model, tea.Cmd) {
	for i, tm := range tabModes {
		if m.mode == tm {
			m.mode = tabModes[(i+1)%len(tabModes)]
			return m, nil
		}
	}
	m.mode = tabModes[0]
	return m, nil
}

// prevPanel cycles to the previous tab in the tabModes ring.
func (m Model) prevPanel() (tea.Model, tea.Cmd) {
	for i, tm := range tabModes {
		if m.mode == tm {
			m.mode = tabModes[(i-1+len(tabModes))%len(tabModes)]
			return m, nil
		}
	}
	m.mode = tabModes[len(tabModes)-1]
	return m, nil
}

// goBack implements the universal "back" action (ESC / double-shift / left).
// Moves one level up in the navigation hierarchy, ending at the intro animation.
func (m Model) goBack() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeDetail, modeEdit, modeNew:
		m.mode = modeList
		m.selected = nil
	case modeProjectPicker, modeSettings:
		m.mode = modeList
	case modeProjects, modeIdeas:
		m.mode = modeSummary
	case modeList:
		m.mode = modeSummary
	case modeSummary:
		m.intro = newIntroState()
		m.mode = modeIntro
	}
	return m, nil
}

// switchProject saves new current project to config and reloads tickets.
func (m Model) switchProject(p store.Project) tea.Cmd {
	newID := fmt.Sprintf("%d", p.ID)
	svc := m.svc
	cfg := m.cfg
	cfg.CurrentProject = newID
	return func() tea.Msg {
		if err := config.Save(cfg); err != nil {
			return errMsg{err}
		}
		tickets := loadTicketsSync(svc, cfg)
		return projectSwitchedMsg{project: p, tickets: tickets}
	}
}

// loadProjects returns a Cmd that fetches all projects.
func loadProjects(svc libticket.Service) tea.Cmd {
	return func() tea.Msg {
		projects, err := svc.ListProjects()
		if err != nil {
			return errMsg{err}
		}
		return projectsLoadedMsg(projects)
	}
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width < 20 || m.height < 8 {
		return "terminal too small"
	}

	// Intro: full-screen, no border wrapper
	if m.mode == modeIntro {
		lines := renderIntro(&m.intro, m.width, m.height)
		return strings.Join(lines, "\n")
	}

	var content []string
	switch m.mode {
	case modeSummary:
		content = m.viewSummary()
	case modeList:
		content = m.viewList()
	case modeDetail:
		content = m.viewDetail()
	case modeEdit:
		content = m.viewEdit()
	case modeNew:
		content = m.viewNew()
	case modeSettings:
		content = m.viewSettings()
	case modeProjectPicker:
		content = m.viewProjectPicker()
	case modeProjects:
		content = m.viewProjects()
	case modeIdeas:
		content = m.viewIdeas()
	}

	if m.showPopup {
		content = m.overlayPopup(content)
	}

	inner := m.width - 2
	padded := padLines(content, inner, m.height-2, m.theme)

	if m.theme.HasPulse {
		return renderBorder(&m.ecg, m.theme, m.width, m.height, padded)
	}
	return renderStaticBorder(m.theme, m.width, m.height, padded)
}

// overlayPopup draws a context action box centered over the content lines.
func (m Model) overlayPopup(lines []string) []string {
	th := m.theme
	inner := m.width - 2
	result := make([]string, len(lines))
	copy(result, lines)

	var actions []string
	if m.cursor < len(m.items) {
		t := m.items[m.cursor].ticket
		actions = []string{
			fmt.Sprintf("  %s %s", typeIcon(t.Type), t.Key),
			"  ──────────────────",
			"  enter  detail view",
			"  e      edit ticket",
			"  n      new ticket",
			"  r      reload",
			"  t      cycle theme",
			"  T      theme picker",
			"  /      command",
			"  ?      help",
			"  esc    close",
		}
	} else {
		actions = []string{
			"  actions",
			"  ──────────────────",
			"  n      new ticket",
			"  r      reload",
			"  esc    close",
		}
	}

	popW := 26
	popH := len(actions) + 2
	startRow := (len(lines) - popH) / 2
	startCol := (inner - popW) / 2
	textStyle := lipgloss.NewStyle().Foreground(th.Fg).Background(th.SelBg)
	bgStyle := lipgloss.NewStyle().Background(th.Bg)

	for i, action := range actions {
		row := startRow + i + 1
		if row < 0 || row >= len(result) {
			continue
		}
		plain := padRight(stripANSI(result[row]), inner)
		runes := []rune(plain)
		popLine := padRight(action, popW)
		if startCol >= 0 && startCol+popW <= inner {
			prefix := string(runes[:startCol])
			suffix := ""
			if startCol+popW < len(runes) {
				suffix = string(runes[startCol+popW:])
			}
			result[row] = bgStyle.Render(prefix) + textStyle.Render(popLine) + bgStyle.Render(suffix)
		}
	}
	return result
}

// padLines pads/truncates content to exactly (w, maxH) with bg fill.
func padLines(lines []string, w, maxH int, t Theme) []string {
	bgStyle := lipgloss.NewStyle().Background(t.Bg)
	out := make([]string, maxH)
	for i := 0; i < maxH; i++ {
		var s string
		if i < len(lines) {
			s = lines[i]
		}
		vis := utf8.RuneCountInString(stripANSI(s))
		if vis < w {
			s += strings.Repeat(" ", w-vis)
		} else if vis > w {
			runes := []rune(s)
			if len(runes) > w {
				s = string(runes[:w])
			}
		}
		out[i] = bgStyle.Render(s)
	}
	return out
}

// stripANSI removes ANSI escape sequences for width measurement.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// ─── status bar ──────────────────────────────────────────────────────────────

func (m Model) statusBar(w int) string {
	th := m.theme
	moon := moonPhases[m.moonPhase]

	var text string
	if m.showCmd {
		text = " " + moon + "  / " + m.cmdInput.View()
	} else if m.statusMsg != "" {
		text = " " + moon + "  " + m.statusMsg
	} else if m.err != nil {
		text = " " + moon + "  ✗ " + m.err.Error()
	} else if m.updateMsg != "" {
		text = " " + moon + "  ↑ " + m.updateMsg
	} else {
		hints := map[viewMode]string{
			modeSummary:       "tab cycle · e edit · n new · p project · t theme · ? settings · qq quit",
			modeList:          "↑↓/wasd · enter · e edit · n new · p project · / cmd · t theme · ? settings · qq quit",
			modeDetail:        "↑↓/ws nav · e edit · esc back",
			modeEdit:          "tab next · ctrl+s save · esc cancel",
			modeNew:           "tab next · ctrl+s create · esc cancel",
			modeSettings:      "↑↓ nav · enter apply theme · esc close",
			modeProjectPicker: "↑↓ nav · enter switch · esc cancel",
			modeProjects:      "↑↓ nav · enter switch project · esc back",
			modeIdeas:         "↑↓/wasd · enter · e edit · n new · esc back",
		}
		hint := hints[m.mode]
		text = " " + moon + "  " + hint
	}

	return lipgloss.NewStyle().
		Foreground(th.StatusFg).Background(th.StatusBg).
		Render(padRight(truncate(text, w-1), w))
}

// ─── list view ────────────────────────────────────────────────────────────────

func (m Model) viewList() []string {
	th := m.theme
	inner := m.width - 2

	projectName := m.project.Title
	if projectName == "" {
		projectName = m.cfg.CurrentProject
	}
	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)

	var lines []string
	lines = append(lines, m.tabBar(inner))
	lines = append(lines, headerStyle.Render(padRight(" "+projectName, inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))

	vis := m.visibleRows() - 1 // extra row used by tab bar
	for i := m.offset; i < len(m.items) && len(lines)-3 < vis; i++ {
		item := m.items[i]
		t := item.ticket

		var prefix string
		switch {
		case item.depth > 0:
			prefix = "   └─ "
		case item.hasChildren && item.expanded:
			prefix = " - "
		case item.hasChildren:
			prefix = " + "
		default:
			prefix = "   "
		}

		icon := typeIcon(t.Type)
		si := stateIcon(t.State, t.Open)
		title := truncate(t.Title, inner-len(prefix)-12)
		keyStr := lipgloss.NewStyle().Foreground(th.Muted).Render(t.Key)
		row := fmt.Sprintf("%s%s%s %s  %s", prefix, si, icon, title, keyStr)

		style := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
		if i == m.cursor {
			style = lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true)
		}
		lines = append(lines, style.Render(padRight(row, inner)))
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

// ─── detail view ─────────────────────────────────────────────────────────────

func (m Model) viewDetail() []string {
	th := m.theme
	inner := m.width - 2
	t := m.selected
	if t == nil {
		return []string{"no ticket selected"}
	}

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	valStyle := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)

	var lines []string
	add := func(label, val string) {
		lines = append(lines,
			labelStyle.Render(padRight(fmt.Sprintf(" %-14s", label), 16))+
				valStyle.Render(truncate(val, inner-17)))
	}

	title := fmt.Sprintf(" %s%s  %s", typeIcon(t.Type), stateIcon(t.State, t.Open), t.Title)
	lines = append(lines, headerStyle.Render(padRight(title, inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	add("key", t.Key)
	add("type", t.Type)
	add("status", t.Status)
	add("state", t.State)
	add("stage", t.Stage)
	if t.Assignee != "" {
		add("assignee", t.Assignee)
	}
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))

	if t.Description != "" {
		lines = append(lines, labelStyle.Render(padRight(" description", inner)))
		for _, dl := range wordWrap(t.Description, inner-2) {
			lines = append(lines, valStyle.Render("  "+padRight(dl, inner-2)))
		}
		lines = append(lines, "")
	}
	if t.AcceptanceCriteria != "" {
		lines = append(lines, labelStyle.Render(padRight(" acceptance criteria", inner)))
		for _, al := range wordWrap(t.AcceptanceCriteria, inner-2) {
			lines = append(lines, valStyle.Render("  "+padRight(al, inner-2)))
		}
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

// ─── edit / new view ─────────────────────────────────────────────────────────

func (m Model) viewEdit() []string {
	title := " edit ticket"
	if m.selected != nil {
		title = " edit  " + m.selected.Key
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
	th := m.theme
	inner := m.width - 2

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	activeStyle := lipgloss.NewStyle().Foreground(th.Accent).Background(th.SelBg)

	var lines []string
	lines = append(lines, headerStyle.Render(padRight(title, inner)))
	lines = append(lines, lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg).
		Render(strings.Repeat("─", inner)))
	lines = append(lines, "")

	for i, name := range m.form.names {
		inp := m.form.inputs[i]
		label := padRight(fmt.Sprintf("  %-12s", name+":"), 14)
		if i == m.form.focus {
			lines = append(lines, activeStyle.Render(label)+inp.View())
		} else {
			lines = append(lines, labelStyle.Render(label)+
				lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg).Render(inp.Value()))
		}
		lines = append(lines, "")
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

// ─── tab bar ──────────────────────────────────────────────────────────────────

// tabBar returns a styled one-line tab bar showing [Home] [Epics] [Config].
func (m Model) tabBar(w int) string {
	th := m.theme
	var parts []string
	for i, tm := range tabModes {
		name := " " + tabNames[i] + " "
		if m.mode == tm {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(th.SelFg).Background(th.SelBg).Bold(true).
				Render(name))
		} else {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(th.Muted).Background(th.Bg).
				Render(name))
		}
		if i < len(tabModes)-1 {
			parts = append(parts, lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg).Render("│"))
		}
	}
	bar := strings.Join(parts, "")
	// Pad remainder
	vis := utf8.RuneCountInString(stripANSI(bar))
	if vis < w {
		bar += lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", w-vis))
	}
	return bar
}

// ─── summary (home) view ──────────────────────────────────────────────────────

func (m Model) viewSummary() []string {
	th := m.theme
	inner := m.width - 2

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	mutedStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	accentStyle := lipgloss.NewStyle().Foreground(th.Accent).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)

	projectName := m.project.Title
	if projectName == "" {
		projectName = m.cfg.CurrentProject
	}

	var lines []string
	lines = append(lines, m.tabBar(inner))
	lines = append(lines, headerStyle.Render(padRight(" "+projectName, inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))

	// Counts
	epicCount, activeCount, openCount := 0, 0, 0
	for _, item := range m.items {
		t := item.ticket
		if t.Type == "epic" {
			epicCount++
		}
		if t.Open {
			openCount++
		}
		if t.State == "active" {
			activeCount++
		}
	}
	lines = append(lines, mutedStyle.Render(padRight(
		fmt.Sprintf("  %d open · %d active · %d epics", openCount, activeCount, epicCount), inner)))
	lines = append(lines, "")

	// Epic summary: show epics with child counts
	lines = append(lines, accentStyle.Render(padRight(" epics", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	for _, n := range m.nodes {
		e := n.epic
		si := stateIcon(e.State, e.Open)
		childInfo := fmt.Sprintf("(%d)", len(n.children))
		line := fmt.Sprintf("  %s◈ %-40s %s", si, truncate(e.Title, 38), childInfo)
		lines = append(lines, mutedStyle.Render(padRight(line, inner)))
	}
	if len(m.nodes) == 0 {
		lines = append(lines, mutedStyle.Render(padRight("  no epics", inner)))
	}
	lines = append(lines, "")

	// Top-level non-epic summary
	if len(m.toplevel) > 0 {
		lines = append(lines, accentStyle.Render(padRight(" other open tickets", inner)))
		lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
		for _, t := range m.toplevel {
			si := stateIcon(t.State, t.Open)
			line := fmt.Sprintf("  %s%s %s", si, typeIcon(t.Type), truncate(t.Title, inner-8))
			lines = append(lines, mutedStyle.Render(padRight(line, inner)))
		}
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

// ─── settings (config) view ───────────────────────────────────────────────────

func (m Model) viewSettings() []string {
	th := m.theme
	inner := m.width - 2

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(th.Accent).Background(th.Bg)
	valStyle := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)

	var lines []string
	lines = append(lines, m.tabBar(inner))
	lines = append(lines, headerStyle.Render(padRight(" config", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	lines = append(lines, "")

	// Theme section
	lines = append(lines, labelStyle.Render(padRight(" themes", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	for i, id := range ThemeOrder {
		t := Themes[id]
		marker := "  "
		if id == m.theme.ID {
			marker = "● "
		}
		style := lipgloss.NewStyle().Foreground(t.Accent).Background(th.Bg)
		if i == m.settingsCursor {
			style = style.Background(th.SelBg).Bold(true)
		}
		lines = append(lines, style.Render(padRight("  "+marker+t.Name, inner)))
	}
	lines = append(lines, "")

	// Preferences section
	lines = append(lines, labelStyle.Render(padRight(" preferences", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	persistMark := "●"
	if m.cfg.TUIDisablePersist {
		persistMark = "○"
	}
	lines = append(lines, valStyle.Render(fmt.Sprintf("  %s  persist session state  (P to toggle)", persistMark)))
	lines = append(lines, "")

	// Key shortcuts section
	lines = append(lines, labelStyle.Render(padRight(" shortcuts", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	type krow struct{ key, desc string }
	shortcuts := []krow{
		{"tab", "cycle panels (Home/Epics/Config)"},
		{"↑↓/ws", "navigate"},
		{"→/d  ←/a", "expand / collapse epics"},
		{"enter", "open detail"},
		{"e", "edit  · n  new  · r  reload"},
		{"p", "project picker"},
		{"t", "cycle theme  · T  this panel"},
		{"/", "command input"},
		{"shift×2", "context popup"},
		{"qq  ctrl+c×2", "quit"},
		{"esc", "back / home"},
	}
	for _, r := range shortcuts {
		lines = append(lines,
			valStyle.Render(fmt.Sprintf("  %-18s", r.key))+
				lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg).Render("  "+r.desc))
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

// ─── project picker view ──────────────────────────────────────────────────────

func (m Model) viewProjectPicker() []string {
	th := m.theme
	inner := m.width - 2

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)

	var lines []string
	lines = append(lines, headerStyle.Render(padRight(" choose project", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	lines = append(lines, "")

	for i, p := range m.projects {
		marker := "  "
		if fmt.Sprintf("%d", p.ID) == m.cfg.CurrentProject {
			marker = "● "
		}
		style := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
		if i == m.projectCursor {
			style = lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true)
		}
		desc := ""
		if p.Description != "" {
			desc = "  " + truncate(p.Description, inner-30)
		}
		line := fmt.Sprintf("  %s%-20s%s", marker, truncate(p.Title, 20), desc)
		lines = append(lines, style.Render(padRight(line, inner)))
	}
	if len(m.projects) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg).
			Render(padRight("  no projects found", inner)))
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

// ─── projects panel view ──────────────────────────────────────────────────────

func (m Model) viewProjects() []string {
	th := m.theme
	inner := m.width - 2

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)
	mutedStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)

	var lines []string
	lines = append(lines, m.tabBar(inner))
	lines = append(lines, headerStyle.Render(padRight("  projects", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	lines = append(lines, "")

	if len(m.projects) == 0 {
		lines = append(lines, mutedStyle.Render(padRight("  loading projects...", inner)))
	} else {
		for i, p := range m.projects {
			marker := "  "
			if fmt.Sprintf("%d", p.ID) == m.cfg.CurrentProject {
				marker = "● "
			}
			style := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
			if i == m.projectCursor {
				style = lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true)
			}
			desc := ""
			if p.Description != "" {
				desc = "  " + truncate(p.Description, inner-30)
			}
			line := fmt.Sprintf("  %s%-20s%s", marker, truncate(p.Title, 20), desc)
			lines = append(lines, style.Render(padRight(line, inner)))
		}
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

// ─── ideas panel view ─────────────────────────────────────────────────────────

func (m Model) viewIdeas() []string {
	th := m.theme
	inner := m.width - 2

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)
	mutedStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)

	var lines []string
	lines = append(lines, m.tabBar(inner))
	lines = append(lines, headerStyle.Render(padRight("  ideas", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))

	if len(m.toplevel) == 0 {
		lines = append(lines, mutedStyle.Render(padRight("  (no ideas — press n to add one)", inner)))
	} else {
		// Clamp cursor
		cursor := m.ideasCursor
		if cursor >= len(m.toplevel) {
			cursor = len(m.toplevel) - 1
		}
		vis := m.visibleRows() - 1 // extra row used by tab bar
		for i := m.ideasOffset; i < len(m.toplevel) && len(lines)-3 < vis; i++ {
			t := m.toplevel[i]
			si := stateIcon(t.State, t.Open)
			icon := typeIcon(t.Type)
			title := truncate(t.Title, inner-12)
			keyStr := lipgloss.NewStyle().Foreground(th.Muted).Render(t.Key)
			row := fmt.Sprintf("   %s%s %s  %s", si, icon, title, keyStr)

			style := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
			if i == cursor {
				style = lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true)
			}
			lines = append(lines, style.Render(padRight(row, inner)))
		}
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}

// ─── commands ─────────────────────────────────────────────────────────────────

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func loadTickets(svc libticket.Service, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		return loadTicketsSync(svc, cfg)
	}
}

func loadTicketsSync(svc libticket.Service, cfg config.Config) treeLoadedMsg {
	project, err := svc.GetProject(cfg.CurrentProject)
	if err != nil {
		return treeLoadedMsg{}
	}
	all, err := svc.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0, false)
	if err != nil {
		return treeLoadedMsg{}
	}

	epicMap := map[int64][]store.Ticket{}
	var epics []store.Ticket
	var toplevel []store.Ticket
	for _, t := range all {
		if !t.Open {
			continue
		}
		if t.ParentID == nil {
			if t.Type == "epic" {
				epics = append(epics, t)
			} else {
				toplevel = append(toplevel, t)
			}
		} else {
			epicMap[*t.ParentID] = append(epicMap[*t.ParentID], t)
		}
	}

	nodes := make([]treeNode, 0, len(epics))
	for _, e := range epics {
		nodes = append(nodes, treeNode{epic: e, children: epicMap[e.ID]})
	}
	return treeLoadedMsg{nodes: nodes, toplevel: toplevel}
}

func flattenTree(nodes []treeNode, toplevel []store.Ticket, expanded map[int64]bool) []listItem {
	var items []listItem
	for _, n := range nodes {
		hasKids := len(n.children) > 0
		exp := expanded[n.epic.ID]
		items = append(items, listItem{
			ticket:      n.epic,
			depth:       0,
			hasChildren: hasKids,
			expanded:    exp,
		})
		if exp {
			for _, child := range n.children {
				items = append(items, listItem{ticket: child, depth: 1})
			}
		}
	}
	for _, t := range toplevel {
		items = append(items, listItem{ticket: t, depth: 0})
	}
	return items
}

func checkUpdate(svc libticket.Service) tea.Cmd {
	return func() tea.Msg {
		status, err := svc.Status()
		if err != nil {
			return nil
		}
		if status.ServerVersion != "" {
			return updateAvailableMsg("server: " + status.ServerVersion)
		}
		return nil
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func typeIcon(t string) string {
	switch t {
	case "epic":
		return "◈ "
	case "story":
		return "◆ "
	case "task":
		return "◉ "
	case "bug":
		return "⚑ "
	case "requirement":
		return "◇ "
	case "decision":
		return "◁ "
	case "question":
		return "? "
	case "note":
		return "✎ "
	}
	return "· "
}

func stateIcon(state string, open bool) string {
	if !open {
		return "✓"
	}
	switch state {
	case "active":
		return "●"
	case "success":
		return "✓"
	case "fail":
		return "✗"
	}
	return "○"
}

func padRight(s string, width int) string {
	vis := utf8.RuneCountInString(s)
	if vis >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func wordWrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	words := strings.Fields(s)
	var line strings.Builder
	for _, w := range words {
		if line.Len() > 0 && line.Len()+1+len(w) > width {
			lines = append(lines, line.String())
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(w)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}
