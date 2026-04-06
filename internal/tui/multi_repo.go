package tui

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/global"
	"github.com/teknal/fleet-commander/internal/monitor"
	"github.com/teknal/fleet-commander/internal/tmux"
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

func (d MultiRepoDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	// Repo header
	if h, ok := item.(RepoHeaderItem); ok {
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Underline(true)
		countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

		title := headerStyle.Render(fmt.Sprintf("  ⚓ %s", h.ShortName))
		desc := countStyle.Render(fmt.Sprintf("    %s  (%d agents)", h.RepoPath, h.Count))
		fmt.Fprint(w, title+"\n"+desc+"\n")
		return
	}

	// Agent item (reuse existing rendering logic)
	i, ok := item.(MultiRepoAgentItem)
	if !ok || i.Agent == nil {
		return
	}

	agent := i.Agent

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

	if !agent.HooksOK && agent.Status != "stopped" {
		hooksWarnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6666"))
		indicator += " " + hooksWarnStyle.Render("⚠ hooks")
	}

	repoTag := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("[" + i.RepoShortName + "] ")
	name := repoTag + agent.Name
	if index == m.Index() {
		name = selectedItemStyle.Render("> " + name)
	} else {
		name = itemStyle.Render("  " + name)
	}

	desc := statusStyle.Render(fmt.Sprintf("    %s  %s", agent.Branch, indicator))

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

// multiRepoModel is the TUI model for multi-repo view.
type multiRepoModel struct {
	list        list.Model
	fleets      []*fleet.Fleet
	repoEntries []global.RepoEntry
	tmuxMgrs    map[string]*tmux.Manager  // keyed by repo short name
	monitors    map[string]*monitor.Monitor
	width       int
	height      int
	quitting    bool
	attachAgent string
	attachFleet *fleet.Fleet
	statusMsg   string
	statusTimer time.Time
}

type repoFleetPair struct {
	entry global.RepoEntry
	fleet *fleet.Fleet
	tmux  *tmux.Manager
	mon   *monitor.Monitor
}

func newMultiRepoModel() (multiRepoModel, error) {
	repos, err := global.List()
	if err != nil {
		return multiRepoModel{}, fmt.Errorf("failed to list repos: %w", err)
	}

	var pairs []repoFleetPair
	for _, r := range repos {
		f, err := fleet.Load(r.Path)
		if err != nil {
			continue
		}
		tm := tmux.NewManager(f.TmuxPrefix())
		mon := monitor.NewMonitor(tm)
		pairs = append(pairs, repoFleetPair{entry: r, fleet: f, tmux: tm, mon: mon})
	}

	m := multiRepoModel{
		tmuxMgrs: make(map[string]*tmux.Manager),
		monitors: make(map[string]*monitor.Monitor),
	}

	var items []list.Item
	for _, p := range pairs {
		m.fleets = append(m.fleets, p.fleet)
		m.repoEntries = append(m.repoEntries, p.entry)
		m.tmuxMgrs[p.entry.ShortName] = p.tmux
		m.monitors[p.entry.ShortName] = p.mon

		// Add repo header
		items = append(items, RepoHeaderItem{
			ShortName: p.entry.ShortName,
			RepoPath:  p.entry.Path,
			Count:     len(p.fleet.Agents),
		})

		// Add agent items
		for _, a := range p.fleet.Agents {
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
	for idx, f := range m.fleets {
		entry := m.repoEntries[idx]
		tm := m.tmuxMgrs[entry.ShortName]
		mon := m.monitors[entry.ShortName]

		items = append(items, RepoHeaderItem{
			ShortName: entry.ShortName,
			RepoPath:  entry.Path,
			Count:     len(f.Agents),
		})

		for _, a := range f.Agents {
			snap := mon.CheckWithStateFile(a.Name, a.StateFilePath)
			items = append(items, MultiRepoAgentItem{
				AgentItem: AgentItem{
					Agent:    a,
					State:    snap.State,
					LastLine: snap.LastLine,
				},
				RepoShortName: entry.ShortName,
				Fleet:         f,
				Tmux:          tm,
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
		// Reload fleets in case config changed
		for i, entry := range m.repoEntries {
			reloaded, err := fleet.Load(entry.Path)
			if err == nil {
				m.fleets[i] = reloaded
			}
		}
		items := m.rebuildItems()
		m.list.SetItems(items)
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
			// Skip repo headers
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
				items := m.rebuildItems()
				m.list.SetItems(items)
			}

		case "k":
			if item, ok := m.list.SelectedItem().(MultiRepoAgentItem); ok {
				if !item.Tmux.SessionExists(item.Agent.Name) {
					return m, nil
				}
				item.Tmux.KillSession(item.Agent.Name)
				item.Fleet.UpdateAgent(item.Agent.Name, "stopped", 0)
				items := m.rebuildItems()
				m.list.SetItems(items)
			}

		case "r":
			items := m.rebuildItems()
			m.list.SetItems(items)
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

	// Count states across all repos
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
		len(m.fleets),
		waitingStyle.Render(fmt.Sprintf("⏳ %d waiting", waiting)),
		runningStyle.Render(fmt.Sprintf("● %d working", working)),
		stoppedStyle.Render(fmt.Sprintf("○ %d stopped", stopped)),
	))

	help := helpStyle.Render("enter: attach • s: start • k: kill • r: refresh • q: quit")

	view := fmt.Sprintf("%s\n%s\n%s", m.list.View(), summary, help)
	if m.statusMsg != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6666"))
		view += "\n" + errorStyle.Render(m.statusMsg)
	}
	return view
}

// startMultiRepoAgent starts a tmux session for an agent in the multi-repo view.
func startMultiRepoAgent(item MultiRepoAgentItem) error {
	agent := item.Agent
	f := item.Fleet
	tm := item.Tmux

	statesDir := fmt.Sprintf("%s/states", f.FleetDir)
	if err := makeDir(statesDir); err != nil {
		return err
	}
	stateFilePath := fmt.Sprintf("%s/%s.json", statesDir, agent.Name)

	if err := tm.CreateSession(agent.Name, agent.WorktreePath, nil, stateFilePath); err != nil {
		return err
	}
	f.UpdateAgentStateFile(agent.Name, stateFilePath)
	return nil
}

func makeDir(path string) error {
	return os.MkdirAll(path, 0755)
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

		// Attach to the selected agent
		tm := tmux.NewManager(fm.attachFleet.TmuxPrefix())
		if tmux.IsInsideTmux() {
			tm.SwitchClient(fm.attachAgent)
		} else {
			tm.Attach(fm.attachAgent)
		}

		// Loop back after detach
	}
}
