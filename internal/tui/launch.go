package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/hooks"
	"github.com/teknal/fleet-commander/internal/tmux"
)

type launchMode int

const (
	launchModeInput launchMode = iota
	launchModeReview
	launchModeEditName
	launchModeEditBranch
	launchModeEditPrompt
)

// LaunchModel is the Bubble Tea model for the fleet launch flow.
type LaunchModel struct {
	fleet *fleet.Fleet
	tmux  *tmux.Manager

	mode launchMode

	// Input phase
	inputArea textarea.Model

	// Parsed prompts
	prompts    []LaunchItem
	currentIdx int

	// Edit phase
	nameInput   textinput.Model
	branchInput textinput.Model
	promptEdit  textarea.Model

	// Results
	launched []string
	skipped  int

	// Layout
	width  int
	height int

	// State
	quitting  bool
	aborted   bool
	statusMsg string
}

func newLaunchModel(f *fleet.Fleet) LaunchModel {
	tm := tmux.NewManager("fleet")

	// Main input textarea
	ta := textarea.New()
	ta.Placeholder = "1. Fix the login validation bug\n2. Add OAuth2 support\n3. Refactor the database layer"
	ta.SetWidth(60)
	ta.SetHeight(8)
	ta.Focus()

	// Edit fields
	ni := textinput.New()
	ni.Placeholder = "agent-name"
	ni.CharLimit = 30

	bi := textinput.New()
	bi.Placeholder = "fleet/branch-name"
	bi.CharLimit = 80

	pe := textarea.New()
	pe.SetWidth(60)
	pe.SetHeight(4)

	return LaunchModel{
		fleet:       f,
		tmux:        tm,
		mode:        launchModeInput,
		inputArea:   ta,
		nameInput:   ni,
		branchInput: bi,
		promptEdit:  pe,
	}
}

func (m LaunchModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m LaunchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.inputArea.SetWidth(min(msg.Width-4, 80))
		return m, nil
	}

	switch m.mode {
	case launchModeInput:
		return m.updateInput(msg)
	case launchModeReview:
		return m.updateReview(msg)
	case launchModeEditName:
		return m.updateEditName(msg)
	case launchModeEditBranch:
		return m.updateEditBranch(msg)
	case launchModeEditPrompt:
		return m.updateEditPrompt(msg)
	}

	return m, nil
}

func (m LaunchModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+d":
			input := m.inputArea.Value()
			items := ParsePrompts(input)
			if len(items) == 0 {
				m.statusMsg = "No prompts found. Enter at least one task."
				return m, nil
			}

			// Deduplicate against existing fleet agents
			var existingNames []string
			for _, a := range m.fleet.Agents {
				existingNames = append(existingNames, a.Name)
			}
			// Re-generate names with fleet context
			for i := range items {
				name, branch := GenerateNames(items[i].Prompt, existingNames)
				items[i].AgentName = name
				items[i].Branch = branch
				existingNames = append(existingNames, name)
			}

			m.prompts = items
			m.currentIdx = 0
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil

		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.inputArea, cmd = m.inputArea.Update(msg)
	return m, cmd
}

func (m LaunchModel) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "l", "L":
			return m.launchCurrent()

		case "e", "E":
			item := m.prompts[m.currentIdx]
			m.nameInput.SetValue(item.AgentName)
			m.nameInput.Focus()
			m.mode = launchModeEditName
			m.statusMsg = ""
			return m, m.nameInput.Focus()

		case "s", "S":
			m.skipped++
			m.statusMsg = ""
			return m.advance()

		case "a", "A", "esc", "ctrl+c":
			m.aborted = true
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m LaunchModel) updateEditName(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := m.nameInput.Value()
			if val == "" {
				m.statusMsg = "Name cannot be empty"
				return m, nil
			}
			m.prompts[m.currentIdx].AgentName = val
			// Move to branch edit
			m.branchInput.SetValue(m.prompts[m.currentIdx].Branch)
			m.branchInput.Focus()
			m.mode = launchModeEditBranch
			m.statusMsg = ""
			return m, m.branchInput.Focus()

		case "esc":
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m LaunchModel) updateEditBranch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := m.branchInput.Value()
			if val == "" {
				m.statusMsg = "Branch cannot be empty"
				return m, nil
			}
			m.prompts[m.currentIdx].Branch = val
			// Move to prompt edit
			m.promptEdit.SetValue(m.prompts[m.currentIdx].Prompt)
			m.promptEdit.Focus()
			m.mode = launchModeEditPrompt
			m.statusMsg = ""
			return m, m.promptEdit.Focus()

		case "esc":
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.branchInput, cmd = m.branchInput.Update(msg)
	return m, cmd
}

func (m LaunchModel) updateEditPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+d":
			val := m.promptEdit.Value()
			if strings.TrimSpace(val) == "" {
				m.statusMsg = "Prompt cannot be empty"
				return m, nil
			}
			m.prompts[m.currentIdx].Prompt = strings.TrimSpace(val)
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil

		case "esc":
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.promptEdit, cmd = m.promptEdit.Update(msg)
	return m, cmd
}

// launchCurrent creates the agent and tmux session for the current prompt.
func (m LaunchModel) launchCurrent() (tea.Model, tea.Cmd) {
	item := m.prompts[m.currentIdx]

	// Create the agent (worktree + config registration)
	agent, err := m.fleet.AddAgent(item.AgentName, item.Branch)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Failed to create agent: %s", err)
		return m, nil
	}

	// Set up state tracking
	statesDir := filepath.Join(m.fleet.FleetDir, "states")
	if err := os.MkdirAll(statesDir, 0755); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to create states dir: %s", err)
		return m, nil
	}
	stateFilePath := filepath.Join(statesDir, agent.Name+".json")

	// Inject hooks for state signaling
	if err := hooks.Inject(agent.WorktreePath); err != nil {
		stateFilePath = ""
		m.fleet.UpdateAgentHooks(agent.Name, false)
	} else {
		m.fleet.UpdateAgentHooks(agent.Name, true)
	}

	// Create tmux session with the prompt passed to Claude
	command := []string{"claude", item.Prompt}
	if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, command, stateFilePath); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to create session: %s", err)
		return m, nil
	}

	// Update state
	m.fleet.UpdateAgentStateFile(agent.Name, stateFilePath)
	pid, err := m.tmux.GetPID(agent.Name)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Launched but could not get PID: %v", err)
	}
	m.fleet.UpdateAgent(agent.Name, "running", pid)
	m.launched = append(m.launched, agent.Name)

	return m.advance()
}

// advance moves to the next prompt or finishes if all are done.
func (m LaunchModel) advance() (tea.Model, tea.Cmd) {
	m.currentIdx++
	if m.currentIdx >= len(m.prompts) {
		m.quitting = true
		return m, tea.Quit
	}
	m.mode = launchModeReview
	return m, nil
}

func (m LaunchModel) View() string {
	if m.quitting {
		if m.aborted {
			return m.renderSummary("Aborted.")
		}
		return m.renderSummary("Done.")
	}

	if m.width == 0 {
		return "Loading..."
	}

	switch m.mode {
	case launchModeInput:
		return m.viewInput()
	case launchModeReview:
		return m.viewReview()
	case launchModeEditName:
		return m.viewEditName()
	case launchModeEditBranch:
		return m.viewEditBranch()
	case launchModeEditPrompt:
		return m.viewEditPrompt()
	}

	return ""
}

func (m LaunchModel) viewInput() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("⚓ Fleet Launch") + "\n\n")
	b.WriteString("  Enter your tasks (one per line, or use bullets/numbers):\n\n")
	b.WriteString("  " + m.inputArea.View() + "\n\n")

	if m.statusMsg != "" {
		b.WriteString("  " + stoppedStyle.Render("❌ "+m.statusMsg) + "\n")
	}

	b.WriteString(helpStyle.Render("  Ctrl+D: submit • Esc: cancel"))

	return b.String()
}

func (m LaunchModel) viewReview() string {
	var b strings.Builder
	item := m.prompts[m.currentIdx]

	b.WriteString(titleStyle.Render(
		fmt.Sprintf("⚓ Fleet Launch — Task %d of %d", m.currentIdx+1, len(m.prompts)),
	) + "\n\n")

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	valueStyle := lipgloss.NewStyle().Bold(true)

	b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Agent: "), valueStyle.Render(item.AgentName)))
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Branch:"), valueStyle.Render(item.Branch)))
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Prompt:"), valueStyle.Render(item.Prompt)))

	b.WriteString("\n")

	actionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	b.WriteString(fmt.Sprintf("  %s Launch  %s Edit  %s Skip  %s Abort\n",
		actionStyle.Render("[L]"),
		actionStyle.Render("[E]"),
		actionStyle.Render("[S]"),
		actionStyle.Render("[A]"),
	))

	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("⚠ "+m.statusMsg))
	}

	// Show launched agents
	if len(m.launched) > 0 {
		b.WriteString("\n")
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
		for _, name := range m.launched {
			b.WriteString("  " + successStyle.Render("✓ Launched: "+name) + "\n")
		}
	}

	return b.String()
}

func (m LaunchModel) viewEditName() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Edit Agent Name") + "\n\n")
	b.WriteString("  " + selectedItemStyle.Render("> Agent name: ") + m.nameInput.View() + "\n")
	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("❌ "+m.statusMsg))
	}
	b.WriteString("\n" + helpStyle.Render("  Enter: next (branch) • Esc: back"))
	return b.String()
}

func (m LaunchModel) viewEditBranch() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Edit Branch Name") + "\n\n")
	b.WriteString("  Agent name:  " + m.prompts[m.currentIdx].AgentName + "\n")
	b.WriteString("  " + selectedItemStyle.Render("> Branch name: ") + m.branchInput.View() + "\n")
	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("❌ "+m.statusMsg))
	}
	b.WriteString("\n" + helpStyle.Render("  Enter: next (prompt) • Esc: back"))
	return b.String()
}

func (m LaunchModel) viewEditPrompt() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Edit Prompt") + "\n\n")
	b.WriteString("  Agent: " + m.prompts[m.currentIdx].AgentName + "\n")
	b.WriteString("  Branch: " + m.prompts[m.currentIdx].Branch + "\n\n")
	b.WriteString("  " + m.promptEdit.View() + "\n")
	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("❌ "+m.statusMsg))
	}
	b.WriteString("\n" + helpStyle.Render("  Ctrl+D: confirm • Esc: back"))
	return b.String()
}

func (m LaunchModel) renderSummary(header string) string {
	var b strings.Builder
	b.WriteString(header + "\n")
	if len(m.launched) > 0 {
		b.WriteString(fmt.Sprintf("Launched %d agent(s): %s\n", len(m.launched), strings.Join(m.launched, ", ")))
	}
	if m.skipped > 0 {
		b.WriteString(fmt.Sprintf("Skipped %d prompt(s)\n", m.skipped))
	}
	return b.String()
}

// RunLaunch starts the launch TUI flow.
func RunLaunch(f *fleet.Fleet) error {
	m := newLaunchModel(f)
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run launch TUI: %w", err)
	}

	return nil
}
