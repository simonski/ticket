package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
	"strings"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── sdlc key handler ─────────────────────────────────────────────────────

// wfRowCount returns the total visible rows (sdlcs + expanded stages).
func (m Model) wfRowCount() int {
	n := len(m.sdlcs)
	for _, wf := range m.sdlcs {
		if m.wfExpanded[wf.ID] {
			n += len(wf.Stages)
		}
	}
	return n
}

// wfRowAt returns the sdlc index and stage index at the given flat row.
// stageIdx == -1 means the row is a sdlc header.
func (m Model) wfRowAt(row int) (wfIdx int, stageIdx int) {
	cur := 0
	for i, wf := range m.sdlcs {
		if cur == row {
			return i, -1
		}
		cur++
		if m.wfExpanded[wf.ID] {
			for j := range wf.Stages {
				if cur == row {
					return i, j
				}
				cur++
			}
		}
	}
	return 0, -1
}

func (m Model) handleKeySdlcs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Stage name input mode
	if m.wfAddingStage {
		switch key {
		case "esc":
			m.wfAddingStage = false
		case "enter":
			name := strings.TrimSpace(m.wfStageInput.Value())
			if name != "" {
				wfIdx, _ := m.wfRowAt(m.wfCursor)
				if wfIdx < len(m.sdlcs) {
					wf := m.sdlcs[wfIdx]
					svc := m.svc
					order := len(wf.Stages)
					m.wfAddingStage = false
					return m, func() tea.Msg {
						_, err := svc.AddSdlcStage(wf.ID, libticket.SdlcStageRequest{
							StageName: name,
							SortOrder: order,
						})
						if err != nil {
							return errMsg{err}
						}
						return loadSdlcs(svc)()
					}
				}
			}
			m.wfAddingStage = false
		default:
			var cmd tea.Cmd
			m.wfStageInput, cmd = m.wfStageInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	total := m.wfRowCount()
	switch key {
	case "q", "esc":
		return m.goBack()
	case "up", "k", "w":
		if m.wfCursor > 0 {
			m.wfCursor--
		}
	case "down", "j", "s":
		if m.wfCursor < total-1 {
			m.wfCursor++
		}
	case "enter", " ":
		// Toggle expand on sdlc headers
		wfIdx, stageIdx := m.wfRowAt(m.wfCursor)
		if stageIdx == -1 && wfIdx < len(m.sdlcs) {
			wfID := m.sdlcs[wfIdx].ID
			m.wfExpanded[wfID] = !m.wfExpanded[wfID]
		}
	case "n":
		// Add a stage to the sdlc under cursor
		wfIdx, _ := m.wfRowAt(m.wfCursor)
		if wfIdx < len(m.sdlcs) {
			m.wfExpanded[m.sdlcs[wfIdx].ID] = true
			m.wfAddingStage = true
			ti := textinput.New()
			ti.Placeholder = "stage name..."
			ti.CharLimit = 100
			ti.Focus()
			m.wfStageInput = ti
		}
	case "x":
		// Delete stage under cursor
		wfIdx, stageIdx := m.wfRowAt(m.wfCursor)
		if stageIdx >= 0 && wfIdx < len(m.sdlcs) {
			stage := m.sdlcs[wfIdx].Stages[stageIdx]
			svc := m.svc
			return m, func() tea.Msg {
				if err := svc.RemoveSdlcStage(stage.ID); err != nil {
					return errMsg{err}
				}
				return loadSdlcs(svc)()
			}
		}
	case "shift+up", "K":
		// Move stage up
		wfIdx, stageIdx := m.wfRowAt(m.wfCursor)
		if stageIdx > 0 && wfIdx < len(m.sdlcs) {
			wf := m.sdlcs[wfIdx]
			ids := make([]int64, len(wf.Stages))
			for i, s := range wf.Stages {
				ids[i] = s.ID
			}
			ids[stageIdx], ids[stageIdx-1] = ids[stageIdx-1], ids[stageIdx]
			svc := m.svc
			wfID := wf.ID
			m.wfCursor--
			return m, func() tea.Msg {
				if err := svc.ReorderSdlcStages(wfID, ids); err != nil {
					return errMsg{err}
				}
				return loadSdlcs(svc)()
			}
		}
	case "shift+down", "J":
		// Move stage down
		wfIdx, stageIdx := m.wfRowAt(m.wfCursor)
		if stageIdx >= 0 && wfIdx < len(m.sdlcs) {
			wf := m.sdlcs[wfIdx]
			if stageIdx < len(wf.Stages)-1 {
				ids := make([]int64, len(wf.Stages))
				for i, s := range wf.Stages {
					ids[i] = s.ID
				}
				ids[stageIdx], ids[stageIdx+1] = ids[stageIdx+1], ids[stageIdx]
				svc := m.svc
				wfID := wf.ID
				m.wfCursor++
				return m, func() tea.Msg {
					if err := svc.ReorderSdlcStages(wfID, ids); err != nil {
						return errMsg{err}
					}
					return loadSdlcs(svc)()
				}
			}
		}
	case "left", "a":
		return m.prevPanel()
	case "right", "d":
		return m.nextPanel()
	}
	return m, nil
}


func loadSdlcs(svc libticket.Service) tea.Cmd {
	return func() tea.Msg {
		wfs, err := svc.ListSdlcs()
		if err != nil {
			return errMsg{err}
		}
		var result []store.SdlcWithStages
		for _, wf := range wfs {
			ws, err := svc.GetSdlc(wf.ID)
			if err != nil {
				continue
			}
			result = append(result, ws)
		}
		return sdlcLoadedMsg(result)
	}
}

// ─── sdlcs panel view ─────────────────────────────────────────────────────

func (m Model) viewSdlcs() []string {
	th := m.theme
	inner := m.width - 2

	headerStyle := lipgloss.NewStyle().Foreground(th.Header).Bold(true).Background(th.Bg)
	sepStyle := lipgloss.NewStyle().Foreground(th.Border).Background(th.Bg)
	mutedStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(th.Bg)
	accentStyle := lipgloss.NewStyle().Foreground(th.Accent).Background(th.Bg)

	var lines []string
	lines = append(lines, m.tabBar(inner))
	lines = append(lines, headerStyle.Render(padRight("  sdlcs", inner)))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", inner)))

	if len(m.sdlcs) == 0 {
		lines = append(lines, mutedStyle.Render(padRight("  (no sdlcs)", inner)))
	} else {
		row := 0
		for _, wf := range m.sdlcs {
			expanded := m.wfExpanded[wf.ID]
			arrow := "▸"
			if expanded {
				arrow = "▾"
			}
			stageCount := fmt.Sprintf("(%d stages)", len(wf.Stages))
			line := fmt.Sprintf("  %s %s  %s", arrow, wf.Name, stageCount)

			style := lipgloss.NewStyle().Foreground(th.Fg).Background(th.Bg)
			if row == m.wfCursor {
				style = lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true)
			}
			lines = append(lines, style.Render(padRight(line, inner)))
			row++

			if expanded {
				for j, stage := range wf.Stages {
					prefix := fmt.Sprintf("    %d. ", j+1)
					desc := ""
					if stage.Description != "" {
						desc = "  — " + truncate(stage.Description, inner-30)
					}
					sline := prefix + stage.StageName + desc

					sStyle := mutedStyle
					if row == m.wfCursor {
						sStyle = lipgloss.NewStyle().Foreground(th.SelFg).Background(th.SelBg).Bold(true)
					}
					lines = append(lines, sStyle.Render(padRight(sline, inner)))
					row++
				}
			}
		}
	}

	// Stage input overlay
	if m.wfAddingStage {
		lines = append(lines, "")
		lines = append(lines, accentStyle.Render("  new stage: ")+m.wfStageInput.View())
	}

	for len(lines) < m.height-3 {
		lines = append(lines, lipgloss.NewStyle().Background(th.Bg).Render(strings.Repeat(" ", inner)))
	}
	lines = append(lines, m.statusBar(inner))
	return lines
}


