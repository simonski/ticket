package tui

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

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
	modeProjectEdit
	modeIdeas // new: list of top-level non-epic tickets (m.toplevel)
	modeSdlcs // sdlc list with expandable stages
	modeBoard // kanban board: tickets by stage columns
)

// tabModes are the top-level panels cycled by tab: Home > Projects > Ideas > Epics > Sdlcs > Config.
var tabModes = []viewMode{modeSummary, modeProjects, modeIdeas, modeList, modeBoard, modeSdlcs, modeSettings}
var tabNames = []string{"Home", "Projects", "Ideas", "Tickets", "Board", "Sdlcs", "Config"}

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
type sdlcLoadedMsg []store.SdlcWithStages
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
	expanded map[string]bool
	items    []listItem
	cursor   int
	offset   int

	// detail / edit
	selected    *store.Ticket
	form        editForm
	newForm     *newTicketForm
	projectForm *projectEditForm

	// settings / theme picker
	settingsCursor int

	// project picker
	projects      []store.Project
	projectCursor int

	// ideas panel
	ideas       []store.Ticket
	ideasCursor int
	ideasOffset int

	// sdlcs panel
	sdlcs         []store.SdlcWithStages
	wfCursor      int
	wfExpanded    map[int64]bool // expanded sdlc IDs
	wfAddingStage bool
	wfStageInput  textinput.Model

	// board (kanban)
	boardCol      int           // active column index
	boardRow      int           // cursor row within the active column
	boardOffset   int           // scroll offset within the active column
	boardCols     []boardColumn // columns built from sdlc stages
	boardInHeader bool          // true when focus is on the stage header row
	boardInTabBar bool          // true when focus is on the panel tab bar

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
	lastEsc   time.Time

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
		svc:        svc,
		cfg:        cfg,
		theme:      th,
		mode:       modeIntro,
		intro:      newIntroState(),
		lastTick:   time.Now(),
		expanded:   map[string]bool{},
		wfExpanded: map[int64]bool{},
		cmdInput:   ci,
		ecg:        ecgState{params: th.ECGStyle},
	}
}

// ─── tea.Model interface ─────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadTickets(m.svc, m.cfg),
		loadSdlcs(m.svc),
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
				var extraCmd tea.Cmd
				m, extraCmd = m.restoreSession()
				m.tickCount++
				if m.tickCount%20 == 0 {
					m.moonPhase = (m.moonPhase + 1) % len(moonPhases)
				}
				return m, tea.Batch(tickCmd(), extraCmd)
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
		m.ideas = filterRequirements(msg.toplevel)
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
		m.buildBoardColumns()
		if m.statusMsg == "reloading..." {
			m.statusMsg = ""
		}

	case ticketCreatedMsg:
		m.statusMsg = "created: " + store.Ticket(msg).ID
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
			if fmt.Sprintf("%d", p.ID) == m.cfg.ProjectID {
				m.projectCursor = i
				break
			}
		}

	case sdlcLoadedMsg:
		m.sdlcs = []store.SdlcWithStages(msg)
		m.buildBoardColumns()

	case projectSavedMsg:
		m.projectForm = nil
		m.mode = modeProjects
		m.statusMsg = "project saved"
		return m, loadProjects(m.svc)

	case projectSwitchedMsg:
		m.project = msg.project
		m.cfg.ProjectID = fmt.Sprintf("%d", msg.project.ID)
		m.nodes = msg.tickets.nodes
		m.toplevel = msg.tickets.toplevel
		m.ideas = filterRequirements(msg.tickets.toplevel)
		m.expanded = map[string]bool{} // reset expand state for new project
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

	// Intro: any key skips to restored panel
	if m.mode == modeIntro {
		var cmd tea.Cmd
		m, cmd = m.restoreSession()
		return m, cmd
	}

	// Command input active: route there
	if m.showCmd {
		return m.handleKeyCmd(msg)
	}

	// Track double-shift for search dialog
	if strings.HasPrefix(key, "shift+") {
		if time.Since(m.lastShift) < 350*time.Millisecond {
			m.shiftCount++
			if m.shiftCount >= 2 {
				m.shiftCount = 0
				m.showCmd = true
				m.cmdInput.SetValue("")
				m.cmdInput.Focus()
				return m, nil
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
			// In board mode, tab cycles tickets (handled by board key handler)
			// unless focus is on the header, where tab enters the body.
			if m.mode != modeBoard {
				return m.nextPanel()
			}
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
			return m, m.panelEntryCmd()
		case "/":
			m.showCmd = true
			m.cmdInput.SetValue("")
			m.cmdInput.Focus()
			return m, nil
		case "esc":
			// Board mode handles esc internally (body → header → tab bar → exit).
			if m.mode == modeBoard {
				break
			}
			if time.Since(m.lastEsc) < 500*time.Millisecond {
				return m, tea.Quit
			}
			m.lastEsc = time.Now()
			m.statusMsg = "press esc again to quit"
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
	case modeProjectEdit:
		return m.handleKeyProjectEdit(msg)
	case modeIdeas:
		return m.handleKeyIdeas(key)
	case modeSdlcs:
		return m.handleKeySdlcs(msg)
	case modeBoard:
		return m.handleKeyBoard(key)
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
		return m, m.saveExpandedState()
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
		return m, m.saveExpandedState()
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
			m.theme = Themes[ThemeOrder[m.settingsCursor]]
			m.ecg.params = m.theme.ECGStyle
			m.cfg.TUITheme = string(m.theme.ID)
		}
	case "down", "j", "s":
		if m.settingsCursor < len(ThemeOrder)-1 {
			m.settingsCursor++
			m.theme = Themes[ThemeOrder[m.settingsCursor]]
			m.ecg.params = m.theme.ECGStyle
			m.cfg.TUITheme = string(m.theme.ID)
		}
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
	case " ":
		if m.projectCursor < len(m.projects) {
			chosen := m.projects[m.projectCursor]
			m.mode = modeList
			return m, m.switchProject(chosen)
		}
	case "enter", "e":
		if m.projectCursor < len(m.projects) {
			chosen := m.projects[m.projectCursor]
			m.projectForm = newProjectEditForm(chosen)
			m.projectForm.applyFocus(m.width - 2)
			m.mode = modeProjectEdit
		}
	case "left", "a":
		return m.prevPanel()
	case "right", "d":
		return m.nextPanel()
	}
	return m, nil
}

func (m Model) handleKeyProjectEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.projectForm == nil {
		m.mode = modeProjects
		return m, nil
	}
	f := m.projectForm
	key := msg.String()

	switch key {
	case "esc":
		m.mode = modeProjects
		m.projectForm = nil
	case "ctrl+s", "ctrl+d":
		return m, m.saveProject()
	case "tab":
		f.nextField()
		f.applyFocus(m.width - 2)
	case "shift+tab":
		f.prevField()
		f.applyFocus(m.width - 2)
	case "enter", " ":
		// Space must reach text input fields, not trigger save action
		if key == " " && f.focus != pfSave {
			cmd := f.update(msg)
			return m, cmd
		}
		if f.focus == pfSave {
			return m, m.saveProject()
		}
	default:
		cmd := f.update(msg)
		return m, cmd
	}
	return m, nil
}

type projectSavedMsg struct{}

func (m Model) saveProject() tea.Cmd {
	if m.projectForm == nil {
		return nil
	}
	f := m.projectForm
	id := f.project.ID
	req := libticket.ProjectUpdateRequest{
		Title:              f.title.Value(),
		Description:        f.desc.Value(),
		AcceptanceCriteria: f.dor.Value(),
		Notes:              f.dod.Value(),
	}
	svc := m.svc
	return func() tea.Msg {
		_, err := svc.UpdateProject(context.Background(), id, req)
		if err != nil {
			return errMsg{err}
		}
		return projectSavedMsg{}
	}
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
		if m.ideasCursor < len(m.ideas)-1 {
			m.ideasCursor++
			vis := m.visibleRows()
			if m.ideasCursor >= m.ideasOffset+vis {
				m.ideasOffset = m.ideasCursor - vis + 1
			}
		}
	case "enter", " ":
		if m.ideasCursor < len(m.ideas) {
			t := m.ideas[m.ideasCursor]
			m.selected = &t
			m.mode = modeDetail
		}
	case "e":
		if m.ideasCursor < len(m.ideas) {
			t := m.ideas[m.ideasCursor]
			m.selected = &t
			m.form = newEditForm(t)
			m.mode = modeEdit
		}
	case "n":
		m.selected = nil
		m.newForm = makeNewTicketForm()
		m.newForm.ticketType = "requirement"
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
// nextPanel cycles to the next tab in the tabModes ring.
func (m Model) nextPanel() (tea.Model, tea.Cmd) {
	for i, tm := range tabModes {
		if m.mode == tm {
			m.mode = tabModes[(i+1)%len(tabModes)]
			return m, m.panelEntryCmd()
		}
	}
	m.mode = tabModes[0]
	return m, m.panelEntryCmd()
}

// prevPanel cycles to the previous tab in the tabModes ring.
func (m Model) prevPanel() (tea.Model, tea.Cmd) {
	for i, tm := range tabModes {
		if m.mode == tm {
			m.mode = tabModes[(i-1+len(tabModes))%len(tabModes)]
			return m, m.panelEntryCmd()
		}
	}
	m.mode = tabModes[len(tabModes)-1]
	return m, m.panelEntryCmd()
}

// goBack implements the universal "back" action (ESC / double-shift / left).
// Moves one level up in the navigation hierarchy, ending at the intro animation.
func (m Model) goBack() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeDetail, modeEdit, modeNew:
		m.mode = modeList
		m.selected = nil
	case modeProjectEdit:
		m.mode = modeProjects
		m.projectForm = nil
	case modeProjectPicker, modeSettings:
		m.mode = modeList
	case modeBoard:
		m.mode = modeSummary
	case modeProjects, modeIdeas, modeSdlcs:
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
	cfg.ProjectID = newID
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
		projects, err := svc.ListProjects(context.Background())
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
	case modeProjectEdit:
		content = m.viewProjectEdit()
	case modeIdeas:
		content = m.viewIdeas()
	case modeSdlcs:
		content = m.viewSdlcs()
	case modeBoard:
		content = m.viewBoard()
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
			fmt.Sprintf("  %s %s", typeIcon(t.Type), t.ID),
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
			modeEdit:          "tab next · enter pick/save · ctrl+s save · esc cancel",
			modeNew:           "tab next · ctrl+s create · esc cancel",
			modeSettings:      "↑↓ nav · enter apply theme · esc close",
			modeProjectPicker: "↑↓ nav · enter switch · esc cancel",
			modeProjects:      "↑↓ nav · space switch · enter/e edit · esc back",
			modeProjectEdit:   "tab next · enter save · ctrl+s save · esc cancel",
			modeIdeas:         "↑↓/wasd · enter · e edit · n new · esc back",
			modeSdlcs:         "↑↓ nav · enter expand · n add stage · x delete · K/J reorder · esc back",
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
		projectName = m.cfg.ProjectID
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
		si := stateIcon(t.State, !t.Complete)
		title := truncate(t.Title, inner-len(prefix)-12)
		keyStr := lipgloss.NewStyle().Foreground(th.Muted).Render(t.ID)
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

	title := fmt.Sprintf(" %s%s  %s", typeIcon(t.Type), stateIcon(t.State, !t.Complete), t.Title)
	lines = append(lines, headerStyle.Render(padRight(title, inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	add("key", t.ID)
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
		projectName = m.cfg.ProjectID
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
		if !t.Complete {
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
		si := stateIcon(e.State, !e.Complete)
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
			si := stateIcon(t.State, !t.Complete)
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
		{"tab", "cycle panels (Home/Tickets/Config)"},
		{"↑↓/ws", "navigate"},
		{"→/d  ←/a", "expand / collapse epics in tickets"},
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
		if fmt.Sprintf("%d", p.ID) == m.cfg.ProjectID {
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
			if fmt.Sprintf("%d", p.ID) == m.cfg.ProjectID {
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

// ─── project edit view ────────────────────────────────────────────────────────

func (m Model) viewProjectEdit() []string {
	if m.projectForm == nil {
		return []string{"no form"}
	}
	f := m.projectForm
	th := m.theme
	inner := m.width - 2

	f.applyFocus(inner)

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	activeLabel := lipgloss.NewStyle().Foreground(th.Accent).Background(th.SelBg).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)

	var lines []string
	lines = append(lines, headerStyle.Render(padRight(" edit project  "+f.project.Prefix, inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))
	lines = append(lines, "")

	// Title
	titleLbl := fmt.Sprintf("  %-14s", "title:")
	if f.focus == pfTitle {
		lines = append(lines, activeLabel.Render(titleLbl)+f.title.View())
	} else {
		lines = append(lines, labelStyle.Render(titleLbl)+valStyle.Render(" "+f.title.Value()))
	}
	lines = append(lines, "")

	// Description
	descLbl := fmt.Sprintf("  %-14s", "description:")
	if f.focus == pfDesc {
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

	// Definition of Ready
	dorLbl := fmt.Sprintf("  %-14s", "def of ready:")
	if f.focus == pfDoR {
		lines = append(lines, activeLabel.Render(dorLbl))
		for _, tl := range strings.Split(f.dor.View(), "\n") {
			lines = append(lines, "  "+tl)
		}
	} else {
		lines = append(lines, labelStyle.Render(dorLbl))
		dorVal := f.dor.Value()
		if dorVal == "" {
			dorVal = "(empty)"
		}
		for _, dl := range wordWrap(dorVal, inner-4) {
			lines = append(lines, valStyle.Render("  "+dl))
		}
	}
	lines = append(lines, "")

	// Definition of Done
	dodLbl := fmt.Sprintf("  %-14s", "def of done:")
	if f.focus == pfDoD {
		lines = append(lines, activeLabel.Render(dodLbl))
		for _, tl := range strings.Split(f.dod.View(), "\n") {
			lines = append(lines, "  "+tl)
		}
	} else {
		lines = append(lines, labelStyle.Render(dodLbl))
		dodVal := f.dod.Value()
		if dodVal == "" {
			dodVal = "(empty)"
		}
		for _, dl := range wordWrap(dodVal, inner-4) {
			lines = append(lines, valStyle.Render("  "+dl))
		}
	}
	lines = append(lines, "")

	// Save button
	saveStr := "  [ Save ]"
	if f.focus == pfSave {
		lines = append(lines, lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true).Render(padRight(saveStr, inner)))
	} else {
		lines = append(lines, valStyle.Render(padRight(saveStr, inner)))
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

	if len(m.ideas) == 0 {
		lines = append(lines, mutedStyle.Render(padRight("  (no ideas — press n to add one)", inner)))
	} else {
		// Clamp cursor
		cursor := m.ideasCursor
		if cursor >= len(m.ideas) {
			cursor = len(m.ideas) - 1
		}
		vis := m.visibleRows() - 1 // extra row used by tab bar
		for i := m.ideasOffset; i < len(m.ideas) && len(lines)-3 < vis; i++ {
			t := m.ideas[i]
			si := stateIcon(t.State, !t.Complete)
			icon := typeIcon(t.Type)
			title := truncate(t.Title, inner-12)
			keyStr := lipgloss.NewStyle().Foreground(th.Muted).Render(t.ID)
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
	project, err := svc.GetProject(context.Background(), cfg.ProjectID)
	if err != nil {
		return treeLoadedMsg{}
	}
	all, err := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, false)
	if err != nil {
		return treeLoadedMsg{}
	}

	epicMap := map[string][]store.Ticket{}
	var epics []store.Ticket
	var toplevel []store.Ticket
	for _, t := range all {
		if t.Complete {
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

func filterRequirements(tickets []store.Ticket) []store.Ticket {
	var out []store.Ticket
	for _, t := range tickets {
		if t.Type == "requirement" {
			out = append(out, t)
		}
	}
	return out
}

func flattenTree(nodes []treeNode, toplevel []store.Ticket, expanded map[string]bool) []listItem {
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
		status, err := svc.Status(context.Background())
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

// ─── session persistence ──────────────────────────────────────────────────────

func modeToString(m viewMode) string {
	switch m {
	case modeProjects:
		return "projects"
	case modeIdeas:
		return "ideas"
	case modeList:
		return "list"
	case modeBoard:
		return "board"
	case modeSettings:
		return "settings"
	default:
		return "summary"
	}
}

func modeFromString(s string) viewMode {
	switch s {
	case "projects":
		return modeProjects
	case "ideas":
		return modeIdeas
	case "list":
		return modeList
	case "board":
		return modeBoard
	case "settings":
		return modeSettings
	default:
		return modeSummary
	}
}

// restoreSession sets the mode from persisted config and returns any
// cmd needed to populate that panel (e.g. load projects).
func (m Model) restoreSession() (Model, tea.Cmd) {
	m.mode = modeFromString(m.cfg.TUIMode)
	var cmd tea.Cmd
	if m.mode == modeProjects && len(m.projects) == 0 {
		cmd = loadProjects(m.svc)
	}
	return m, cmd
}

// panelEntryCmd saves panel state and loads data required for the new panel.
func (m Model) panelEntryCmd() tea.Cmd {
	var cmds []tea.Cmd
	if !m.cfg.TUIDisablePersist {
		cfg := m.cfg
		cfg.TUIMode = modeToString(m.mode)
		m.cfg = cfg
		cmds = append(cmds, func() tea.Msg {
			if err := config.Save(cfg); err != nil {
				log.Printf("tui: save mode: %v", err)
			}
			return nil
		})
	}
	if m.mode == modeProjects && len(m.projects) == 0 {
		cmds = append(cmds, loadProjects(m.svc))
	}
	return tea.Batch(cmds...)
}

// saveExpandedState persists the current expanded epic set to config.
func (m Model) saveExpandedState() tea.Cmd {
	if m.cfg.TUIDisablePersist {
		return nil
	}
	var ids []string
	for id, exp := range m.expanded {
		if exp {
			ids = append(ids, id)
		}
	}
	cfg := m.cfg
	cfg.TUIExpandedEpics = ids
	return func() tea.Msg {
		if err := config.Save(cfg); err != nil {
			log.Printf("tui: save expanded state: %v", err)
		}
		return nil
	}
}
