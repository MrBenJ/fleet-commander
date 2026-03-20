package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/tmux"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	itemStyle = lipgloss.NewStyle().
		PaddingLeft(2)

	selectedItemStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#7D56F4"))

	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	runningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00"))

	stoppedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))
)

// AgentItem represents an agent in the list
type AgentItem struct {
	Agent     *fleet.Agent
	IsRunning bool
}

func (i AgentItem) FilterValue() string { return i.Agent.Name }

// AgentDelegate customizes how agents are rendered
type AgentDelegate struct{}

func (d AgentDelegate) Height() int                                { return 2 }
func (d AgentDelegate) Spacing() int                               { return 1 }
func (d AgentDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd    { return nil }

func (d AgentDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(AgentItem)
	if !ok || i.Agent == nil {
		return
	}

	agent := i.Agent

	// Status indicator (use cached value)
	status := "○"
	statusColor := stoppedStyle
	if i.IsRunning {
		status = "●"
		statusColor = runningStyle
	}

	// Title with status
	title := fmt.Sprintf("%s %s", statusColor.Render(status), agent.Name)
	if index == m.Index() {
		title = selectedItemStyle.Render("> " + title)
	} else {
		title = itemStyle.Render("  " + title)
	}

	// Description
	desc := fmt.Sprintf("    %s • %s", agent.Branch, agent.Status)

	fmt.Fprint(w, title+"\n"+statusStyle.Render(desc))
}

// refreshMsg triggers a status refresh
type refreshMsg struct{}

// Model is the TUI model
type Model struct {
	list        list.Model
	fleet       *fleet.Fleet
	tmux        *tmux.Manager
	width       int
	height      int
	quitting    bool
	attachAgent string // set when user selects an agent to attach to
}

// New creates a new TUI model
func New(f *fleet.Fleet) Model {
	tm := tmux.NewManager("fleet")

	items := buildItems(f, tm)

	delegate := AgentDelegate{}
	l := list.New(items, delegate, 0, 0)
	l.Title = "Fleet Commander"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return Model{
		list:  l,
		fleet: f,
		tmux:  tm,
	}
}

func buildItems(f *fleet.Fleet, tm *tmux.Manager) []list.Item {
	items := make([]list.Item, len(f.Agents))
	for i, a := range f.Agents {
		isRunning := tm.SessionExists(a.Name)
		items[i] = AgentItem{Agent: a, IsRunning: isRunning}
	}
	return items
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)

	case refreshMsg:
		items := buildItems(m.fleet, m.tmux)
		m.list.SetItems(items)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent

				// If session doesn't exist, create it
				if !item.IsRunning {
					if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, ""); err != nil {
						// Don't crash — just stay in TUI
						return m, nil
					}
					pid, _ := m.tmux.GetPID(agent.Name)
					m.fleet.UpdateAgent(agent.Name, "running", pid)
				}

				// Store the agent name and quit — Run() will handle the attach
				m.attachAgent = agent.Name
				m.quitting = true
				return m, tea.Quit
			}

		case "s":
			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent
				if m.tmux.SessionExists(agent.Name) {
					return m, nil
				}
				if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, ""); err != nil {
					return m, nil
				}
				pid, _ := m.tmux.GetPID(agent.Name)
				m.fleet.UpdateAgent(agent.Name, "running", pid)
				items := buildItems(m.fleet, m.tmux)
				m.list.SetItems(items)
			}

		case "k":
			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent
				if !m.tmux.SessionExists(agent.Name) {
					return m, nil
				}
				if err := m.tmux.KillSession(agent.Name); err != nil {
					return m, nil
				}
				m.fleet.UpdateAgent(agent.Name, "stopped", 0)
				items := buildItems(m.fleet, m.tmux)
				m.list.SetItems(items)
			}

		case "r":
			items := buildItems(m.fleet, m.tmux)
			m.list.SetItems(items)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the TUI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 {
		return "Loading..."
	}

	help := helpStyle.Render("enter: attach • s: start • k: kill • r: refresh • q: quit")

	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.list.View(),
		statusStyle.Render(fmt.Sprintf("Repo: %s", m.fleet.RepoPath)),
		help,
	)
}

// Run starts the TUI, and if the user selected an agent, attaches to it after exit
func Run(f *fleet.Fleet) error {
	m := New(f)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// After TUI has fully exited, check if user selected an agent to attach to
	if fm, ok := finalModel.(Model); ok && fm.attachAgent != "" {
		tm := tmux.NewManager("fleet")

		if tmux.IsInsideTmux() {
			return tm.SwitchClient(fm.attachAgent)
		}
		return tm.Attach(fm.attachAgent)
	}

	return nil
}
