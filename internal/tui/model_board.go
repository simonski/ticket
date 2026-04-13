package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonski/ticket/internal/store"
	"strings"
	"time"
)

// ─── board (kanban) ──────────────────────────────────────────────────────────

type boardColumn struct {
	stage   string
	tickets []store.Ticket
}

// buildBoardColumns groups open tickets by stage into columns.
func (m *Model) buildBoardColumns() {
	stages := []string{store.StageDesign, store.StageDevelop, store.StageTest, store.StageDone}
	if len(m.sdlcs) > 0 && len(m.sdlcs[0].Stages) > 0 {
		stages = stages[:0]
		for _, ws := range m.sdlcs[0].Stages {
			stages = append(stages, ws.StageName)
		}
	}

	cols := make([]boardColumn, len(stages))
	for i, s := range stages {
		cols[i].stage = s
	}
	// Collect all open non-epic tickets (children + toplevel).
	for _, n := range m.nodes {
		for _, t := range n.children {
			for i, c := range cols {
				if c.stage == t.Stage {
					cols[i].tickets = append(cols[i].tickets, t)
				}
			}
		}
	}
	for _, t := range m.toplevel {
		for i, c := range cols {
			if c.stage == t.Stage {
				cols[i].tickets = append(cols[i].tickets, t)
			}
		}
	}
	m.boardCols = cols
	if m.boardCol >= len(cols) {
		m.boardCol = 0
	}
}

func (m Model) handleKeyBoard(key string) (tea.Model, tea.Cmd) {
	if len(m.boardCols) == 0 {
		return m, nil
	}

	// ── Tab-bar focused: left/right cycle panels, down enters stage headers ──
	if m.boardInTabBar {
		switch key {
		case "left", "h", "a":
			return m.prevPanel()
		case "right", "l", "d", "tab":
			return m.nextPanel()
		case "down", "j", "s", "enter", " ":
			m.boardInTabBar = false
			m.boardInHeader = true
			return m, nil
		case "esc":
			return m.goBack()
		case "q":
			if time.Since(m.lastQ) < 500*time.Millisecond {
				return m, tea.Quit
			}
			m.lastQ = time.Now()
			m.statusMsg = "press q again to quit"
		}
		return m, nil
	}

	// ── Stage-header focused: left/right cycle columns, up goes to tab bar ──
	if m.boardInHeader {
		switch key {
		case "left", "h", "a":
			if m.boardCol > 0 {
				m.boardCol--
			}
		case "right", "l", "d":
			if m.boardCol < len(m.boardCols)-1 {
				m.boardCol++
			}
		case "up", "k", "w", "esc":
			m.boardInHeader = false
			m.boardInTabBar = true
		case "down", "j", "s", "enter", " ", "tab":
			m.boardInHeader = false
			m.boardRow = 0
			m.boardOffset = 0
		case "q":
			if time.Since(m.lastQ) < 500*time.Millisecond {
				return m, tea.Quit
			}
			m.lastQ = time.Now()
			m.statusMsg = "press q again to quit"
		}
		return m, nil
	}

	// ── Board body: navigate tickets ─────────────────────────────────────────
	switch key {
	case "q":
		if time.Since(m.lastQ) < 500*time.Millisecond {
			return m, tea.Quit
		}
		m.lastQ = time.Now()
		m.statusMsg = "press q again to quit"

	case "esc":
		m.boardInHeader = true

	case "left", "h", "a":
		if m.boardCol > 0 {
			m.boardCol--
			m.boardRow = 0
			m.boardOffset = 0
		}

	case "right", "l", "d":
		if m.boardCol < len(m.boardCols)-1 {
			m.boardCol++
			m.boardRow = 0
			m.boardOffset = 0
		}

	case "up", "k", "w":
		if m.boardRow > 0 {
			m.boardRow--
			if m.boardRow < m.boardOffset {
				m.boardOffset = m.boardRow
			}
		} else {
			m.boardInHeader = true
		}

	case "down", "j", "s":
		col := m.boardCols[m.boardCol]
		if m.boardRow < len(col.tickets)-1 {
			m.boardRow++
		} else {
			m.boardAdvanceColumn()
		}

	case "tab":
		col := m.boardCols[m.boardCol]
		if m.boardRow < len(col.tickets)-1 {
			m.boardRow++
		} else {
			m.boardAdvanceColumn()
		}

	case "enter", " ":
		col := m.boardCols[m.boardCol]
		if m.boardRow < len(col.tickets) {
			t := col.tickets[m.boardRow]
			m.selected = &t
			m.mode = modeDetail
		}

	case "e":
		col := m.boardCols[m.boardCol]
		if m.boardRow < len(col.tickets) {
			t := col.tickets[m.boardRow]
			m.selected = &t
			m.form = newEditForm(t)
			m.mode = modeEdit
		}

	case "r":
		return m, loadTickets(m.svc, m.cfg)

	case "n":
		m.newForm = makeNewTicketForm()
		m.newForm.applyFocus(m.width - 2)
		m.mode = modeNew
	}
	return m, nil
}

// boardAdvanceColumn moves to the first ticket in the next column (wrapping).
func (m *Model) boardAdvanceColumn() {
	for i := 1; i <= len(m.boardCols); i++ {
		next := (m.boardCol + i) % len(m.boardCols)
		if len(m.boardCols[next].tickets) > 0 {
			m.boardCol = next
			m.boardRow = 0
			m.boardOffset = 0
			return
		}
	}
}

func (m Model) viewBoard() []string {
	if len(m.boardCols) == 0 {
		return []string{"no board data — press r to reload"}
	}

	th := m.theme
	inner := m.width - 2
	numCols := len(m.boardCols)
	colWidth := inner / numCols
	if colWidth < 10 {
		colWidth = 10
	}

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)
	accent := lipgloss.NewStyle().Foreground(th.Accent)
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	selStyle := lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true)
	normal := lipgloss.NewStyle().Foreground(th.Fg)

	projectName := m.project.Title
	if projectName == "" {
		projectName = m.cfg.ProjectID
	}

	var lines []string
	lines = append(lines, m.tabBar(inner))
	lines = append(lines, headerStyle.Render(padRight(" "+projectName, inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))

	// bodyHeight accounts for: tab bar, project header, project sep,
	// stage header, stage sep, gutter sep, gutter (3 lines), status bar, borders.
	bodyHeight := m.height - 12

	// Stage column header row
	var header strings.Builder
	for i, col := range m.boardCols {
		name := strings.ToUpper(col.stage)
		count := fmt.Sprintf(" (%d)", len(col.tickets))
		label := name + count
		if len(label) > colWidth-1 {
			label = label[:colWidth-1]
		}
		if i == m.boardCol && m.boardInHeader {
			header.WriteString(selStyle.Render(padRight(label, colWidth)))
		} else if i == m.boardCol {
			header.WriteString(accent.Render(padRight(label, colWidth)))
		} else {
			header.WriteString(muted.Render(padRight(label, colWidth)))
		}
	}

	// Separator
	sep := muted.Render(strings.Repeat("─", inner))

	// Body: ticket IDs per column
	bodyLines := make([]string, bodyHeight)
	for row := 0; row < bodyHeight; row++ {
		var line strings.Builder
		for ci, col := range m.boardCols {
			idx := row
			if ci == m.boardCol {
				idx = row + m.boardOffset
			}
			var cell string
			if idx < len(col.tickets) {
				t := col.tickets[idx]
				label := t.ID
				if len(label) > colWidth-2 {
					label = label[:colWidth-2]
				}
				if ci == m.boardCol && idx == m.boardRow && !m.boardInHeader {
					cell = selStyle.Render(padRight(label, colWidth))
				} else {
					cell = normal.Render(padRight(label, colWidth))
				}
			} else {
				cell = padRight("", colWidth)
			}
			line.WriteString(cell)
		}
		bodyLines[row] = line.String()
	}

	// Gutter: selected ticket details
	var gutterLines []string
	col := m.boardCols[m.boardCol]
	if m.boardRow < len(col.tickets) {
		t := col.tickets[m.boardRow]
		gutterLines = append(gutterLines, accent.Render(t.ID)+" "+normal.Render(t.Title))
		gutterLines = append(gutterLines, muted.Render(fmt.Sprintf("type: %s  status: %s  assignee: %s", t.Type, t.Status, t.Assignee)))
		if t.ParentID != nil {
			gutterLines = append(gutterLines, muted.Render("parent: "+*t.ParentID))
		}
	}

	lines = append(lines, header.String(), sep)
	lines = append(lines, bodyLines...)
	lines = append(lines, muted.Render(strings.Repeat("─", inner)))
	lines = append(lines, gutterLines...)
	lines = append(lines, m.statusBar(inner))
	return lines
}
