package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/teknal/fleet-commander/internal/fleet"
	"github.com/teknal/fleet-commander/internal/hooks"
	"github.com/teknal/fleet-commander/internal/tmux"
)

type launchMode int

const (
	launchModeInput launchMode = iota
	launchModeYoloConfirm
	launchModeGenerating
	launchModeReview
	launchModeEditName
	launchModeEditBranch
	launchModeEditPrompt
)

// claudeResultMsg carries the result of the async Claude CLI call.
type claudeResultMsg struct {
	items []LaunchItem
	err   error
}

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

	// Generating phase
	spinner spinner.Model

	// Edit phase
	nameInput   textinput.Model
	branchInput textinput.Model
	promptEdit  textarea.Model

	// Review phase — scrollable prompt viewport
	promptViewport    viewport.Model
	promptViewportIdx int // tracks which prompt index the viewport was built for

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

	// YOLO mode
	yoloMode         bool
	skipYoloConfirm  bool   // --i-know-what-im-doing flag
	noAutoMerge      bool   // --no-auto-merge flag
	targetBranch     string // root repo's current branch, resolved at launch time
	pendingYoloInput string // input saved from first CTRL+D, waiting for confirmation
}

func newLaunchModel(f *fleet.Fleet, yoloMode bool, skipYoloConfirm bool, noAutoMerge bool) LaunchModel {
	tm := tmux.NewManager("fleet")

	// Main input textarea
	ta := textarea.New()
	ta.Placeholder = "Fix the login validation bug\nAdd OAuth2 support\nRefactor the database layer"
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.SetWidth(60)
	ta.SetHeight(8)
	ta.Focus()

	// Spinner for generating phase
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	// Edit fields
	ni := textinput.New()
	ni.Placeholder = "agent-name"
	ni.CharLimit = 30

	bi := textinput.New()
	bi.Placeholder = "fleet/branch-name"
	bi.CharLimit = 80

	pe := textarea.New()
	pe.ShowLineNumbers = false
	pe.Prompt = ""
	pe.SetWidth(80)
	pe.SetHeight(10)

	return LaunchModel{
		fleet:             f,
		tmux:              tm,
		mode:              launchModeInput,
		inputArea:         ta,
		spinner:           sp,
		nameInput:         ni,
		branchInput:       bi,
		promptEdit:        pe,
		promptViewportIdx: -1,
		yoloMode:        yoloMode,
		skipYoloConfirm: skipYoloConfirm,
		noAutoMerge:     noAutoMerge,
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
		m.promptEdit.SetWidth(min(msg.Width-4, 80))
		m.promptEdit.SetHeight(max(msg.Height-10, 5))
		// Reset viewport so it gets rebuilt with new dimensions
		m.promptViewportIdx = -1
		return m, nil
	}

	// Handle Claude CLI result regardless of mode
	if result, ok := msg.(claudeResultMsg); ok {
		if result.err != nil {
			m.mode = launchModeInput
			m.statusMsg = fmt.Sprintf("Claude generation failed: %s", result.err)
			m.inputArea.Focus()
			return m, nil
		}
		m.prompts = result.items
		m.currentIdx = 0

		// In YOLO mode, skip review and launch everything immediately
		if m.yoloMode {
			return m.launchAll()
		}

		m.mode = launchModeReview
		m.statusMsg = ""
		return m, nil
	}

	switch m.mode {
	case launchModeInput:
		return m.updateInput(msg)
	case launchModeYoloConfirm:
		return m.updateYoloConfirm(msg)
	case launchModeGenerating:
		return m.updateGenerating(msg)
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

// launchAll launches every prompt without review (YOLO mode).
func (m LaunchModel) launchAll() (tea.Model, tea.Cmd) {
	// Resolve target branch once for merge instructions (skip when auto-merge is disabled)
	if !m.noAutoMerge {
		targetBranch, err := m.fleet.CurrentBranch()
		if err != nil {
			m.statusMsg = fmt.Sprintf("Failed to detect current branch: %s", err)
			m.mode = launchModeInput
			m.inputArea.Focus()
			return m, nil
		}
		m.targetBranch = targetBranch
	}

	for m.currentIdx < len(m.prompts) {
		model, _ := m.launchCurrent()
		m = model.(LaunchModel)
		if m.statusMsg != "" {
			// Hit an error — stop launching, show the error
			return m, tea.Quit
		}
	}

	m.quitting = true
	return m, tea.Quit
}

// buildFullPrompt assembles the full prompt: system prompt + agent roster + task.
func buildFullPrompt(systemPrompt string, allItems []LaunchItem, currentItem LaunchItem) string {
	var b strings.Builder

	// 1. System prompt preamble (if present)
	if strings.TrimSpace(systemPrompt) != "" {
		b.WriteString(systemPrompt)
		b.WriteString("\n\n")
	}

	// 2. Agent roster
	b.WriteString("## Active Fleet Agents\n\n")
	b.WriteString(fmt.Sprintf("You are: %s (branch: %s)\n\n", currentItem.AgentName, currentItem.Branch))
	b.WriteString("| Agent | Branch | Task |\n")
	b.WriteString("|-------|--------|------|\n")
	for _, item := range allItems {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", item.AgentName, item.Branch, item.Prompt))
	}
	b.WriteString("\n---\n\n")

	// 3. Original task
	b.WriteString(currentItem.Prompt)

	return b.String()
}

// launchCurrent creates the agent and tmux session for the current prompt.
func (m LaunchModel) launchCurrent() (tea.Model, tea.Cmd) {
	item := m.prompts[m.currentIdx]

	// In YOLO mode, append auto-merge instructions to the prompt (unless suppressed)
	if m.yoloMode && !m.noAutoMerge && m.targetBranch != "" {
		item.Prompt = item.Prompt + fmt.Sprintf(`

IMPORTANT — AUTOMATIC MERGE INSTRUCTIONS:
After this feature is completed successfully, you MUST merge your changes back into the target branch. Do the following:
1. Commit all your changes with a descriptive commit message
2. Run: git checkout %s
3. Run: git merge %s --no-edit
4. If there are merge conflicts, resolve them and commit
5. Run: git checkout %s
This merge step is mandatory. Do not skip it.`, m.targetBranch, item.Branch, item.Branch)
	}

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
		fmt.Fprintf(os.Stderr, "Warning: could not inject hooks for agent '%s' (.claude/settings.json may be malformed): %v\n", agent.Name, err)
		stateFilePath = ""
		m.fleet.UpdateAgentHooks(agent.Name, false)
	} else {
		m.fleet.UpdateAgentHooks(agent.Name, true)
	}

	// Create tmux session with the prompt passed to Claude
	command := []string{"claude"}
	if m.yoloMode {
		command = append(command, "--dangerously-skip-permissions")
	}
	command = append(command, item.Prompt)
	if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, command, stateFilePath); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to create session: %s", err)
		return m, nil
	}

	// Update state
	m.fleet.UpdateAgentStateFile(agent.Name, stateFilePath)
	pid, err := m.tmux.GetPID(agent.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: launched agent '%s' but could not get PID: %v\n", agent.Name, err)
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

// RunLaunch starts the launch TUI flow.
func RunLaunch(f *fleet.Fleet, yoloMode bool, skipYoloConfirm bool, noAutoMerge bool) error {
	m := newLaunchModel(f, yoloMode, skipYoloConfirm, noAutoMerge)
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run launch TUI: %w", err)
	}

	return nil
}
