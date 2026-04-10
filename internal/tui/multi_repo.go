package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/global"
	"github.com/MrBenJ/fleet-commander/internal/monitor"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

// RepoHeaderItem is a visual separator/header for a repo group in the multi-repo TUI.
type RepoHeaderItem struct {
	ShortName string
	RepoPath  string
	Count     int
}

func (r RepoHeaderItem) FilterValue() string { return r.ShortName }

// MultiRepoAgentItem extends AgentItem with repo context.
type MultiRepoAgentItem struct {
	AgentItem
	RepoShortName string
	Fleet         *fleet.Fleet
	Tmux          *tmux.Manager
}

func (i MultiRepoAgentItem) FilterValue() string {
	return i.RepoShortName + "/" + i.Agent.Name
}

// MultiRepoDelegate renders items in the multi-repo TUI.
type MultiRepoDelegate struct{}

func (d MultiRepoDelegate) Height() int                             { return 3 }
func (d MultiRepoDelegate) Spacing() int                            { return 0 }
func (d MultiRepoDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

var repoTagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

func (d MultiRepoDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if h, ok := item.(RepoHeaderItem); ok {
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Underline(true)

		title := headerStyle.Render(fmt.Sprintf("  ⚓ %s", h.ShortName))
		desc := statusStyle.Render(fmt.Sprintf("    %s  (%d agents)", h.RepoPath, h.Count))
		fmt.Fprint(w, title+"\n"+desc+"\n")
		return
	}

	i, ok := item.(MultiRepoAgentItem)
	if !ok || i.Agent == nil {
		return
	}

	repoTag := repoTagStyle.Render("[" + i.RepoShortName + "] ")
	fmt.Fprint(w, renderAgentItem(i.Agent, i.State, i.LastLine, repoTag, index, m.Index()))
}

// repoFleetPair bundles a global registry entry with its loaded fleet state.
type repoFleetPair struct {
	entry global.RepoEntry
	fleet *fleet.Fleet
	tmux  *tmux.Manager
	mon   *monitor.Monitor
}

// multiRepoModel is the TUI model for multi-repo view.
type multiRepoModel struct {
	list        list.Model
	repos       []repoFleetPair
	width       int
	height      int
	quitting    bool
	attachAgent string
	attachFleet *fleet.Fleet
	statusMsg   string
	statusTimer time.Time
}

func newMultiRepoModel() (multiRepoModel, error) {
	entries, err := global.List()
	if err != nil {
		return multiRepoModel{}, fmt.Errorf("failed to list repos: %w", err)
	}

	var repos []repoFleetPair
	for _, r := range entries {
		f, err := fleet.Load(r.Path)
		if err != nil {
			continue
		}
		tm := tmux.NewManager(f.TmuxPrefix())
		mon := monitor.NewMonitor(tm)
		repos = append(repos, repoFleetPair{entry: r, fleet: f, tmux: tm, mon: mon})
	}

	m := multiRepoModel{repos: repos}
	items := m.rebuildItems()

	delegate := MultiRepoDelegate{}
	l := list.New(items, delegate, 0, 0)
	l.Title = "⚓ Fleet Commander — All Repos"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	m.list = l
	return m, nil
}

func (m multiRepoModel) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return refreshMsg{}
	})
}

func (m multiRepoModel) rebuildItems() []list.Item {
	var items []list.Item
	for _, p := range m.repos {
		items = append(items, RepoHeaderItem{
			ShortName: p.entry.ShortName,
			RepoPath:  p.entry.Path,
			Count:     len(p.fleet.Agents),
		})

		for _, a := range p.fleet.Agents {
			if drv, err := driver.Get(a.Driver); err == nil {
				p.mon.SetDriver(a.Name, drv)
			}
			snap := p.mon.CheckWithStateFile(a.Name, a.StateFilePath)
			items = append(items, MultiRepoAgentItem{
				AgentItem: AgentItem{
					Agent:    a,
					State:    snap.State,
					LastLine: snap.LastLine,
				},
				RepoShortName: p.entry.ShortName,
				Fleet:         p.fleet,
				Tmux:          p.tmux,
			})
		}
	}
	return items
}

func (m multiRepoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)

	case refreshMsg:
		for i, p := range m.repos {
			reloaded, err := fleet.Load(p.entry.Path)
			if err == nil {
				m.repos[i].fleet = reloaded
			}
		}
		m.list.SetItems(m.rebuildItems())
		if !m.statusTimer.IsZero() && time.Since(m.statusTimer) >= 5*time.Second {
			m.statusMsg = ""
			m.statusTimer = time.Time{}
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
			if _, ok := m.list.SelectedItem().(RepoHeaderItem); ok {
				return m, nil
			}

			if item, ok := m.list.SelectedItem().(MultiRepoAgentItem); ok {
				agent := item.Agent

				if item.State == monitor.StateStopped {
					if err := startMultiRepoAgent(item); err != nil {
						m.statusMsg = "⚠ failed to start agent: " + err.Error()
						m.statusTimer = time.Now()
						return m, nil
					}
					pid, err := item.Tmux.GetPID(agent.Name)
					if err == nil {
						item.Fleet.UpdateAgent(agent.Name, "running", pid)
					}
				}

				m.attachAgent = agent.Name
				m.attachFleet = item.Fleet
				m.quitting = true
				return m, tea.Quit
			}

		case "s":
			if item, ok := m.list.SelectedItem().(MultiRepoAgentItem); ok {
				if item.Tmux.SessionExists(item.Agent.Name) {
					return m, nil
				}
				if err := startMultiRepoAgent(item); err != nil {
					m.statusMsg = "⚠ failed to start: " + err.Error()
					m.statusTimer = time.Now()
					return m, nil
				}
				pid, err := item.Tmux.GetPID(item.Agent.Name)
				if err == nil {
					item.Fleet.UpdateAgent(item.Agent.Name, "running", pid)
				}
				m.list.SetItems(m.rebuildItems())
			}

		case "k":
			if item, ok := m.list.SelectedItem().(MultiRepoAgentItem); ok {
				if !item.Tmux.SessionExists(item.Agent.Name) {
					return m, nil
				}
				item.Tmux.KillSession(item.Agent.Name)
				item.Fleet.UpdateAgent(item.Agent.Name, "stopped", 0)
				m.list.SetItems(m.rebuildItems())
			}

		case "r":
			m.list.SetItems(m.rebuildItems())
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m multiRepoModel) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "Loading..."
	}

	var waiting, working, stopped int
	for _, item := range m.list.Items() {
		if ai, ok := item.(MultiRepoAgentItem); ok {
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
		"%d repos  │  %s  %s  %s",
		len(m.repos),
		waitingStyle.Render(fmt.Sprintf("⏳ %d waiting", waiting)),
		runningStyle.Render(fmt.Sprintf("● %d working", working)),
		stoppedStyle.Render(fmt.Sprintf("○ %d stopped", stopped)),
	))

	help := helpStyle.Render("enter: attach • s: start • k: kill • r: refresh • q: quit")

	view := fmt.Sprintf("%s\n%s\n%s", m.list.View(), summary, help)
	if m.statusMsg != "" {
		view += "\n" + hooksWarnStyle.Render(m.statusMsg)
	}
	return view
}

func startMultiRepoAgent(item MultiRepoAgentItem) error {
	agent := item.Agent
	f := item.Fleet

	drv, err := driver.Get(agent.Driver)
	if err != nil {
		return fmt.Errorf("unknown driver %q: %w", agent.Driver, err)
	}

	if err := drv.CheckAvailable(); err != nil {
		return err
	}

	statesDir := filepath.Join(f.FleetDir, "states")
	if err := os.MkdirAll(statesDir, 0755); err != nil {
		return err
	}
	stateFilePath := filepath.Join(statesDir, agent.Name+".json")

	if err := drv.InjectHooks(agent.WorktreePath); err != nil {
		stateFilePath = ""
		f.UpdateAgentHooks(agent.Name, false)
	} else {
		f.UpdateAgentHooks(agent.Name, true)
	}

	if err := item.Tmux.CreateSession(agent.Name, agent.WorktreePath, drv.InteractiveCommand(), stateFilePath); err != nil {
		return err
	}
	f.UpdateAgentStateFile(agent.Name, stateFilePath)
	return nil
}

// RunMultiRepo starts the multi-repo TUI loop.
func RunMultiRepo() error {
	for {
		m, err := newMultiRepoModel()
		if err != nil {
			return err
		}
		p := tea.NewProgram(m, tea.WithAltScreen())

		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}

		fm, ok := finalModel.(multiRepoModel)
		if !ok {
			return nil
		}

		if fm.attachAgent == "" {
			return nil
		}

		tm := tmux.NewManager(fm.attachFleet.TmuxPrefix())
		if tmux.IsInsideTmux() {
			tm.SwitchClient(fm.attachAgent)
		} else {
			tm.Attach(fm.attachAgent)
		}
	}
}
