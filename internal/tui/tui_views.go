package tui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/MrBenJ/fleet-commander/internal/monitor"
)

func (d AgentDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	// Handle "Add New Agent" item
	if _, ok := item.(AddNewItem); ok {
		addStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

		title := "+ Add New Agent"
		if index == m.Index() {
			title = selectedItemStyle.Render("> " + title)
		} else {
			title = itemStyle.Render("  " + addStyle.Render(title))
		}
		desc := statusStyle.Render("    Create a new agent workspace")
		fmt.Fprint(w, title+"\n"+desc+"\n")
		return
	}

	i, ok := item.(AgentItem)
	if !ok || i.Agent == nil {
		return
	}

	fmt.Fprint(w, renderAgentItem(i.Agent, i.State, i.LastLine, "", index, m.Index()))
}

// View renders the TUI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 {
		return "Loading..."
	}

	// Show add agent form
	if m.mode == modeAddName || m.mode == modeAddBranch {
		var s string
		s += titleStyle.Render("⚓ Add New Agent") + "\n\n"

		nameLabel := "  Agent name:  "
		branchLabel := "  Branch name: "

		if m.mode == modeAddName {
			nameLabel = selectedItemStyle.Render("> Agent name:  ")
		}
		if m.mode == modeAddBranch {
			branchLabel = selectedItemStyle.Render("> Branch name: ")
		}

		s += nameLabel + m.nameInput.View() + "\n"
		s += branchLabel + m.branchInput.View() + "\n"

		if m.statusMsg != "" {
			s += "\n" + stoppedStyle.Render("  ❌ " + m.statusMsg)
		}

		s += "\n" + helpStyle.Render("  enter: next/confirm • esc: cancel")
		return s
	}

	// Count states
	var waiting, working, stopped int
	for _, item := range m.list.Items() {
		if ai, ok := item.(AgentItem); ok {
			switch ai.State {
			case monitor.StateWaiting:
				waiting++
			case monitor.StateWorking, monitor.StateStarting:
				working++
			default:
				stopped++
			}
		}
	}

	summary := statusStyle.Render(fmt.Sprintf(
		"Repo: %s  │  %s  %s  %s",
		m.fleet.RepoPath,
		waitingStyle.Render(fmt.Sprintf("⏳ %d waiting", waiting)),
		runningStyle.Render(fmt.Sprintf("● %d working", working)),
		stoppedStyle.Render(fmt.Sprintf("○ %d stopped", stopped)),
	))

	help := helpStyle.Render("enter: attach • s: start • k: kill • r: refresh • q: quit")

	view := fmt.Sprintf(
		"%s\n%s\n%s",
		m.list.View(),
		summary,
		help,
	)
	if m.statusMsg != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6666"))
		view += "\n" + errorStyle.Render(m.statusMsg)
	}
	return view
}

// updateAddMode handles the "Add New Agent" input flow
func (m Model) updateAddMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel add flow
			m.mode = modeList
			m.nameInput.Reset()
			m.branchInput.Reset()
			m.statusMsg = ""
			m.statusMsgTimer = time.Time{}
			return m, nil

		case "enter":
			if m.mode == modeAddName {
				name := m.nameInput.Value()
				if name == "" {
					m.statusMsg = "Name cannot be empty"
					m.statusMsgTimer = time.Now()
					return m, nil
				}
				m.mode = modeAddBranch
				m.branchInput.Focus()
				return m, m.branchInput.Focus()
			}

			if m.mode == modeAddBranch {
				name := m.nameInput.Value()
				branch := m.branchInput.Value()
				if branch == "" {
					m.statusMsg = "Branch cannot be empty"
					m.statusMsgTimer = time.Now()
					return m, nil
				}

				// Create the agent
				_, err := m.fleet.AddAgent(name, branch)
				if err != nil {
					m.statusMsg = err.Error()
					m.statusMsgTimer = time.Now()
					return m, nil
				}

				// Reset inputs and go back to list
				m.mode = modeList
				m.nameInput.Reset()
				m.branchInput.Reset()
				m.statusMsg = ""
				m.statusMsgTimer = time.Time{}

				// Refresh list
				items := buildItems(m.fleet, m.tmux, m.monitor)
				m.list.SetItems(items)
				return m, nil
			}
		}
	}

	// Update the active text input
	var cmd tea.Cmd
	if m.mode == modeAddName {
		m.nameInput, cmd = m.nameInput.Update(msg)
	} else {
		m.branchInput, cmd = m.branchInput.Update(msg)
	}
	return m, cmd
}
