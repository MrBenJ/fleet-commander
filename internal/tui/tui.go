package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/monitor"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
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
	nameInput      textinput.Model
	branchInput    textinput.Model
	statusMsg      string
	statusMsgTimer time.Time
}

// New creates a new TUI model
func New(f *fleet.Fleet) Model {
	tm := tmux.NewManager(f.TmuxPrefix())
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
	items := []list.Item{AddNewItem{}}
	for _, a := range f.Agents {
		if drv, err := driver.Get(a.Driver); err == nil {
			mon.SetDriver(a.Name, drv)
		}
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
	drv, err := driver.Get(agent.Driver)
	if err != nil {
		return fmt.Errorf("unknown driver %q: %w", agent.Driver, err)
	}

	if err := drv.CheckAvailable(); err != nil {
		return err
	}

	statesDir := filepath.Join(m.fleet.FleetDir, "states")
	if err := os.MkdirAll(statesDir, 0755); err != nil {
		return fmt.Errorf("failed to create states dir: %w", err)
	}
	stateFilePath := filepath.Join(statesDir, agent.Name+".json")

	if err := drv.InjectHooks(agent.WorktreePath); err != nil {
		stateFilePath = ""
		m.fleet.UpdateAgentHooks(agent.Name, false)
	} else {
		m.fleet.UpdateAgentHooks(agent.Name, true)
	}

	if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, drv.InteractiveCommand(), stateFilePath); err != nil {
		return err
	}

	m.monitor.SetDriver(agent.Name, drv)
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
		if !m.statusMsgTimer.IsZero() && time.Since(m.statusMsgTimer) >= 5*time.Second {
			m.statusMsg = ""
			m.statusMsgTimer = time.Time{}
		}
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
				m.statusMsg = ""
				m.statusMsgTimer = time.Time{}
				return m, m.nameInput.Focus()
			}

			if item, ok := m.list.SelectedItem().(AgentItem); ok {
				agent := item.Agent

				if item.State == monitor.StateStopped {
					if err := m.startAgentSession(agent); err != nil {
						m.statusMsg = "⚠ failed to start agent: " + err.Error()
						m.statusMsgTimer = time.Now()
						return m, nil
					}
					pid, err := m.tmux.GetPID(agent.Name)
					if err != nil {
						m.statusMsg = fmt.Sprintf("⚠ started but could not get PID: %v", err)
						m.statusMsgTimer = time.Now()
					}
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
					m.statusMsg = "⚠ failed to start agent: " + err.Error()
					m.statusMsgTimer = time.Now()
					return m, nil
				}
				pid, err := m.tmux.GetPID(agent.Name)
				if err != nil {
					m.statusMsg = fmt.Sprintf("⚠ started but could not get PID: %v", err)
					m.statusMsgTimer = time.Now()
				}
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
				if agent.StateFilePath != "" {
					if err := os.Remove(agent.StateFilePath); err != nil {
						m.statusMsg = "⚠ could not remove state file: " + err.Error()
						m.statusMsgTimer = time.Now()
					}
					m.fleet.UpdateAgentStateFile(agent.Name, "")
				}
				drv, _ := driver.Get(agent.Driver)
				if drv != nil {
					if err := drv.RemoveHooks(agent.WorktreePath); err != nil {
						m.statusMsg = "⚠ could not remove hooks: " + err.Error()
						m.statusMsgTimer = time.Now()
					}
				}
				m.fleet.UpdateAgentHooks(agent.Name, false)
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
		tm := tmux.NewManager(f.TmuxPrefix())

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
