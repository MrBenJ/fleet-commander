package tui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/monitor"
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

	waitingStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD700")).
		Bold(true)

	stoppedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))
)

// AgentItem represents an agent in the list
type AgentItem struct {
	Agent    *fleet.Agent
	State    monitor.AgentState
	LastLine string
}

func (i AgentItem) FilterValue() string { return i.Agent.Name }

// AgentDelegate customizes how agents are rendered
type AgentDelegate struct{}

func (d AgentDelegate) Height() int                             { return 3 }
func (d AgentDelegate) Spacing() int                            { return 0 }
func (d AgentDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d AgentDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(AgentItem)
	if !ok || i.Agent == nil {
		return
	}

	agent := i.Agent

	// Status indicator based on monitor state
	var indicator string
	switch i.State {
	case monitor.StateWaiting:
		indicator = waitingStyle.Render("⏳ NEEDS INPUT")
	case monitor.StateWorking:
		indicator = runningStyle.Render("● working")
	case monitor.StateStarting:
		indicator = runningStyle.Render("◐ starting")
	default:
		indicator = stoppedStyle.Render("○ stopped")
	}

	// Agent name
	name := agent.Name
	if index == m.Index() {
		name = selectedItemStyle.Render("> " + name)
	} else {
		name = itemStyle.Render("  " + name)
	}

	// Branch + status line
	desc := statusStyle.Render(fmt.Sprintf("    %s  %s", agent.Branch, indicator))

	// Last line preview (truncated)
	preview := ""
	if i.LastLine != "" && (i.State == monitor.StateWaiting || i.State == monitor.StateWorking) {
		line := i.LastLine
		if len(line) > 60 {
			line = line[:57] + "..."
		}
		if i.State == monitor.StateWaiting {
			preview = waitingStyle.Render(fmt.Sprintf("    💬 %s", line))
		} else {
			preview = statusStyle.Render(fmt.Sprintf("    … %s", line))
		}
	}

	fmt.Fprint(w, name+"\n"+desc+"\n"+preview)
}

// refreshMsg triggers a status refresh
type refreshMsg struct{}

// Model is the TUI model
type Model struct {
	list        list.Model
	fleet       *fleet.Fleet
	tmux        *tmux.Manager
	monitor     *monitor.Monitor
	width       int
	height      int
	quitting    bool
	attachAgent string
}

// New creates a new TUI model
func New(f *fleet.Fleet) Model {
	tm := tmux.NewManager("fleet")
	mon := monitor.NewMonitor(tm)

	items := buildItems(f, tm, mon)

	delegate := AgentDelegate{}
	l := list.New(items, delegate, 0, 0)
	l.Title = "⚓ Fleet Commander"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return Model{
		list:    l,
		fleet:   f,
		tmux:    tm,
		monitor: mon,
	}
}

func buildItems(f *fleet.Fleet, tm *tmux.Manager, mon *monitor.Monitor) []list.Item {
	items := make([]list.Item, len(f.Agents))
	for i, a := range f.Agents {
		snap := mon.Check(a.Name)
		items[i] = AgentItem{
			Agent:    a,
			State:    snap.State,
			LastLine: snap.LastLine,
		}
	}
	return items
}

// Init starts periodic refresh
func (m Model) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return refreshMsg{}
	})
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)

	case refreshMsg:
		items := buildItems(m.fleet, m.tmux, m.monitor)
		m.list.SetItems(items)
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return refreshMsg{}
		})

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent

				if item.State == monitor.StateStopped {
					if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, ""); err != nil {
						return m, nil
					}
					pid, _ := m.tmux.GetPID(agent.Name)
					m.fleet.UpdateAgent(agent.Name, "running", pid)
				}

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
				items := buildItems(m.fleet, m.tmux, m.monitor)
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
				items := buildItems(m.fleet, m.tmux, m.monitor)
				m.list.SetItems(items)
			}

		case "r":
			items := buildItems(m.fleet, m.tmux, m.monitor)
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

	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.list.View(),
		summary,
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

	// After TUI has fully exited, attach to selected agent
	if fm, ok := finalModel.(Model); ok && fm.attachAgent != "" {
		tm := tmux.NewManager("fleet")

		if tmux.IsInsideTmux() {
			return tm.SwitchClient(fm.attachAgent)
		}
		return tm.Attach(fm.attachAgent)
	}

	return nil
}
