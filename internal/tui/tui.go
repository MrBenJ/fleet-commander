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
	Agent  *fleet.Agent
	Tmux   *tmux.Manager
}

func (i AgentItem) FilterValue() string { return i.Agent.Name }

// AgentDelegate customizes how agents are rendered
type AgentDelegate struct{}

func (d AgentDelegate) Height() int                             { return 2 }
func (d AgentDelegate) Spacing() int                            { return 1 }
func (d AgentDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d AgentDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(AgentItem)
	if !ok {
		return
	}
	
	agent := i.Agent
	
	// Status indicator
	status := "○"
	statusColor := stoppedStyle
	if i.Tmux.SessionExists(agent.Name) {
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

// Model is the TUI model
type Model struct {
	list    list.Model
	fleet   *fleet.Fleet
	tmux    *tmux.Manager
	width   int
	height  int
	quitting bool
}

// New creates a new TUI model
func New(f *fleet.Fleet) Model {
	tm := tmux.NewManager("fleet")
	
	items := make([]list.Item, len(f.Agents))
	for i, a := range f.Agents {
		items[i] = AgentItem{Agent: a, Tmux: tm}
	}
	
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
		m.list.SetSize(msg.Width, msg.Height-6)
		
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
			
		case "enter":
			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent
				
				// If session doesn't exist, create it
				if !m.tmux.SessionExists(agent.Name) {
					if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, ""); err != nil {
						return m, tea.Printf("Error: %v", err)
					}
					// Update status
					pid, _ := m.tmux.GetPID(agent.Name)
					m.fleet.UpdateAgent(agent.Name, "running", pid)
				}
				
				// Switch to the session
				m.quitting = true
				return m, tea.Sequence(
					tea.Quit,
					func() tea.Msg {
						// This will run after the TUI closes
						if tmux.IsInsideTmux() {
							m.tmux.SwitchClient(agent.Name)
						} else {
							m.tmux.Attach(agent.Name)
						}
						return nil
					},
				)
			}
			
		case "s":
			// Start selected agent
			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent
				if m.tmux.SessionExists(agent.Name) {
					return m, tea.Printf("Agent '%s' is already running", agent.Name)
				}
				if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, ""); err != nil {
					return m, tea.Printf("Error starting agent: %v", err)
				}
				pid, _ := m.tmux.GetPID(agent.Name)
				m.fleet.UpdateAgent(agent.Name, "running", pid)
			}
			
		case "k":
			// Kill selected agent
			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent
				if !m.tmux.SessionExists(agent.Name) {
					return m, tea.Printf("Agent '%s' is not running", agent.Name)
				}
				if err := m.tmux.KillSession(agent.Name); err != nil {
					return m, tea.Printf("Error stopping agent: %v", err)
				}
				m.fleet.UpdateAgent(agent.Name, "stopped", 0)
			}
			
		case "r":
			// Refresh list
			// TODO: Reload fleet config
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
	
	var status string
	if m.tmux.IsAvailable() {
		sessions, _ := m.tmux.ListSessions()
		status = fmt.Sprintf("tmux: %d sessions", len(sessions))
	} else {
		status = "tmux: not installed"
	}
	
	help := helpStyle.Render("enter: attach • s: start • k: kill • r: refresh • q: quit")
	
	return fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		m.list.View(),
		statusStyle.Render(fmt.Sprintf("Repo: %s", m.fleet.RepoPath)),
		statusStyle.Render(status),
		help,
	)
}

// Run starts the TUI
func Run(f *fleet.Fleet) error {
	m := New(f)
	p := tea.NewProgram(m, tea.WithAltScreen())
	
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	
	return nil
}
