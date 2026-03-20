package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/hooks"
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

// AddNewItem is the "add new agent" entry at the top of the list
type AddNewItem struct{}

func (i AddNewItem) FilterValue() string { return "+ Add New Agent" }

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

// inputMode tracks which input field is active
type inputMode int

const (
	modeList inputMode = iota
	modeAddName
	modeAddBranch
)

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
	mode        inputMode
	nameInput   textinput.Model
	branchInput textinput.Model
	addError    string
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

	// Name input
	ni := textinput.New()
	ni.Placeholder = "agent-name"
	ni.CharLimit = 30

	// Branch input
	bi := textinput.New()
	bi.Placeholder = "feature/my-branch"
	bi.CharLimit = 80

	return Model{
		list:        l,
		fleet:       f,
		tmux:        tm,
		monitor:     mon,
		nameInput:   ni,
		branchInput: bi,
	}
}

func buildItems(f *fleet.Fleet, tm *tmux.Manager, mon *monitor.Monitor) []list.Item {
	// First item is always "Add New Agent"
	items := []list.Item{AddNewItem{}}

	for _, a := range f.Agents {
		snap := mon.CheckWithStateFile(a.Name, a.StateFilePath)
		items = append(items, AgentItem{
			Agent:    a,
			State:    snap.State,
			LastLine: snap.LastLine,
		})
	}
	return items
}

// startAgentSession creates a tmux session for an agent, injecting hooks and
// wiring the state file path.
func (m *Model) startAgentSession(agent *fleet.Agent) error {
	statesDir := filepath.Join(m.fleet.FleetDir, "states")
	os.MkdirAll(statesDir, 0755)
	stateFilePath := filepath.Join(statesDir, agent.Name+".json")

	if err := hooks.Inject(agent.WorktreePath); err != nil {
		stateFilePath = "" // degrade gracefully
	}

	if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, "", stateFilePath); err != nil {
		return err
	}

	// Persist so buildItems can pass stateFilePath to the monitor on next refresh
	m.fleet.UpdateAgentStateFile(agent.Name, stateFilePath)
	return nil
}

// Init starts periodic refresh
func (m Model) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return refreshMsg{}
	})
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle input modes first
	if m.mode == modeAddName || m.mode == modeAddBranch {
		return m.updateAddMode(msg)
	}

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
			// Handle "Add New Agent" item
			if _, ok := m.list.SelectedItem().(AddNewItem); ok {
				m.mode = modeAddName
				m.nameInput.Focus()
				m.addError = ""
				return m, m.nameInput.Focus()
			}

			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent

				if item.State == monitor.StateStopped {
					if err := m.startAgentSession(agent); err != nil {
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
				if err := m.startAgentSession(agent); err != nil {
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
			m.addError = ""
			return m, nil

		case "enter":
			if m.mode == modeAddName {
				name := m.nameInput.Value()
				if name == "" {
					m.addError = "Name cannot be empty"
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
					m.addError = "Branch cannot be empty"
					return m, nil
				}

				// Create the agent
				_, err := m.fleet.AddAgent(name, branch)
				if err != nil {
					m.addError = err.Error()
					return m, nil
				}

				// Reset inputs and go back to list
				m.mode = modeList
				m.nameInput.Reset()
				m.branchInput.Reset()
				m.addError = ""

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

		if m.addError != "" {
			s += "\n" + stoppedStyle.Render("  ❌ " + m.addError)
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

	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.list.View(),
		summary,
		help,
	)
}

// Run starts the TUI in a loop.
// When the user selects an agent, it attaches to the tmux session.
// When the user detaches (Ctrl+B, Q), it returns to the queue.
// Press 'q' in the queue to fully exit.
func Run(f *fleet.Fleet) error {
	for {
		m := New(f)
		p := tea.NewProgram(m, tea.WithAltScreen())

		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}

		fm, ok := finalModel.(Model)
		if !ok {
			return nil
		}

		// If user pressed 'q' (no agent selected), exit the loop
		if fm.attachAgent == "" {
			return nil
		}

		// Attach to the selected agent's tmux session
		tm := tmux.NewManager("fleet")

		if tmux.IsInsideTmux() {
			tm.SwitchClient(fm.attachAgent)
		} else {
			tm.Attach(fm.attachAgent)
		}

		// When tmux detach happens (Ctrl+B, Q), we land here
		// and loop back to showing the queue again

		// Reload fleet config in case anything changed
		reloaded, err := fleet.Load(f.RepoPath)
		if err == nil {
			f = reloaded
		}
	}
}
