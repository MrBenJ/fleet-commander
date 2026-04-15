package tui

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/squadron"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
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
	launchModeSquadronConsensus
	launchModeSquadronName
)

// claudeResultMsg carries the result of the async Claude CLI call.
type claudeResultMsg struct {
	items []LaunchItem
	err   error
}

// LaunchModel is the Bubble Tea model for the fleet launch flow.
// Phase-specific state is held in the phase structs (InputPhase,
// GeneratingPhase, ReviewPhase, SquadronPhase) so that each mode's
// state is self-contained. Shared state lives directly on the model.
type LaunchModel struct {
	fleet *fleet.Fleet
	tmux  *tmux.Manager

	mode launchMode

	// Phase-specific state
	input      InputPhase
	generating GeneratingPhase
	review     ReviewPhase
	squadron   SquadronPhase

	// Parsed prompts (populated after Generating, used in Review and launch)
	prompts    []LaunchItem
	currentIdx int

	// Results (accumulated across launches)
	launched []string
	skipped  int

	// Layout
	width  int
	height int

	// State
	quitting  bool
	aborted   bool
	statusMsg string

	// YOLO mode flags (set once at creation, read during launch)
	yoloMode        bool
	skipYoloConfirm bool   // --i-know-what-im-doing flag
	noAutoMerge     bool   // --no-auto-merge flag
	targetBranch    string // root repo's current branch, resolved at launch time

	// System prompt (lazy-loaded on first launchCurrent call)
	systemPrompt       string
	systemPromptLoaded bool

	// Jump.sh integration
	useJumpSh bool

	// Squadron mode (set once at creation)
	squadronMode           bool
	squadronName           string
	consensusType          string // "universal" | "review_master" | "none"
	reviewMaster           string
	mergeMaster            string
	autoMerge              bool
	baseBranch             string
	squadronChannelCreated bool
	personas               map[string]string // agent name -> persona key

	// Debug logger
	log *LaunchLogger
}

func newLaunchModel(f *fleet.Fleet, yoloMode bool, skipYoloConfirm bool, noAutoMerge bool, useJumpSh bool) LaunchModel {
	tm := tmux.NewManager(f.TmuxPrefix())

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

	log := NewLaunchLogger(f.FleetDir)
	log.Log("Mode: yolo=%v, skipConfirm=%v, noAutoMerge=%v, useJumpSh=%v", yoloMode, skipYoloConfirm, noAutoMerge, useJumpSh)

	return LaunchModel{
		fleet: f,
		tmux:  tm,
		mode:  launchModeInput,
		input: InputPhase{
			area: ta,
		},
		generating: GeneratingPhase{
			spinner: sp,
		},
		review: ReviewPhase{
			nameInput:   ni,
			branchInput: bi,
			promptEdit:  pe,
			viewportIdx: -1,
		},
		yoloMode:        yoloMode,
		skipYoloConfirm: skipYoloConfirm,
		noAutoMerge:     noAutoMerge,
		useJumpSh:       useJumpSh,
		log:             log,
	}
}

func newSquadronLaunchModel(f *fleet.Fleet, useJumpSh bool) LaunchModel {
	m := newLaunchModel(f, true, true, true, useJumpSh)
	m.squadronMode = true
	m.autoMerge = true
	m.personas = map[string]string{}

	ni := textinput.New()
	ni.Placeholder = "alpha"
	ni.CharLimit = 30
	m.squadron.nameInput = ni

	m.mode = launchModeSquadronConsensus
	return m
}

func (m LaunchModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m LaunchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.area.SetWidth(min(msg.Width-4, 80))
		m.input.area.SetHeight(max(msg.Height-10, 5))
		m.review.promptEdit.SetWidth(min(msg.Width-4, 80))
		m.review.promptEdit.SetHeight(max(msg.Height-10, 5))
		// Reset viewport so it gets rebuilt with new dimensions
		m.review.viewportIdx = -1
		return m, nil
	}

	// Handle Claude CLI result regardless of mode
	if result, ok := msg.(claudeResultMsg); ok {
		if result.err != nil {
			m.log.Log("ERROR: Claude generation failed: %s", result.err)
			m.mode = launchModeInput
			m.statusMsg = fmt.Sprintf("Claude generation failed: %s", result.err)
			m.input.area.Focus()
			return m, nil
		}
		m.log.Log("Claude generated %d prompt(s)", len(result.items))
		for i, item := range result.items {
			m.log.Log("  [%d] agent=%q branch=%q prompt_len=%d", i, item.AgentName, item.Branch, len(item.Prompt))
		}
		m.prompts = result.items
		m.currentIdx = 0

		// In YOLO mode, skip review and launch everything immediately
		if m.yoloMode {
			m.log.Log("YOLO mode: launching all %d agents", len(result.items))
			return m.launchAll()
		}

		m.mode = launchModeReview
		m.statusMsg = ""
		return m, nil
	}

	// After all agents launch (e.g. yolo mode), Bubble Tea may deliver one
	// more message before the quit takes effect. Bail out early.
	if m.quitting {
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
	case launchModeSquadronConsensus:
		return m.updateSquadronConsensus(msg)
	case launchModeSquadronName:
		return m.updateSquadronName(msg)
	}

	return m, nil
}

// launchAll launches every prompt without review (YOLO mode).
func (m LaunchModel) launchAll() (tea.Model, tea.Cmd) {
	// Resolve target branch once for merge instructions (skip when auto-merge is disabled)
	if !m.noAutoMerge {
		targetBranch, err := m.fleet.CurrentBranch()
		if err != nil {
			m.log.Log("ERROR: Failed to detect current branch: %s", err)
			m.statusMsg = fmt.Sprintf("Failed to detect current branch: %s", err)
			m.mode = launchModeInput
			m.input.area.Focus()
			return m, nil
		}
		m.targetBranch = targetBranch
		m.log.Log("Target branch for auto-merge: %s", targetBranch)
	}

	for m.currentIdx < len(m.prompts) {
		m.log.Log("Launching agent %d/%d: %q", m.currentIdx+1, len(m.prompts), m.prompts[m.currentIdx].AgentName)
		model, _ := m.launchCurrent()
		m = model.(LaunchModel)
		if m.statusMsg != "" {
			// Hit an error — stop launching, show the error
			m.log.Log("ERROR: Launch failed at agent %d: %s", m.currentIdx+1, m.statusMsg)
			m.quitting = true
			return m, tea.Quit
		}
		m.log.Log("Successfully launched agent: %q", m.prompts[m.currentIdx-1].AgentName)
	}

	m.log.Log("All %d agents launched successfully", len(m.prompts))
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
	// Load system prompt once (lazily on first launch)
	if !m.systemPromptLoaded {
		m.systemPromptLoaded = true
		sp, err := fleet.LoadSystemPrompt(m.fleet.FleetDir)
		if err != nil {
			m.log.Log("WARNING: could not load system prompt: %v", err)
			fmt.Fprintf(os.Stderr, "warning: could not load system prompt: %v\n", err)
		} else {
			m.log.Log("System prompt loaded (%d bytes)", len(sp))
		}
		m.systemPrompt = sp

		if m.useJumpSh {
			m.systemPrompt += "\n\nYour workbranch will be able to be accessed via a local dev instance via a tool called 'https://jump.sh/' - Jump SH. Fetch this web URL to see what it does and how it works. Upon initialization, use this locally hosted web server as a way to access a local development environment for yourself"
		}
	}

	item := m.prompts[m.currentIdx]
	m.log.Log("launchCurrent: agent=%q branch=%q prompt_len=%d", item.AgentName, item.Branch, len(item.Prompt))

	if m.squadronMode && !m.squadronChannelCreated {
		if m.baseBranch == "" {
			if cb, err := m.fleet.CurrentBranch(); err == nil {
				m.baseBranch = cb
			} else {
				m.log.Log("WARNING: could not resolve base branch: %v", err)
				m.baseBranch = "main"
			}
		}

		agentNames := make([]string, 0, len(m.prompts))
		for _, p := range m.prompts {
			agentNames = append(agentNames, p.AgentName)
		}

		if m.consensusType == "review_master" && m.reviewMaster == "" {
			m.reviewMaster = agentNames[rand.Intn(len(agentNames))]
			m.log.Log("Squadron review master selected: %s", m.reviewMaster)
		}
		if m.autoMerge && m.mergeMaster == "" {
			m.mergeMaster = agentNames[rand.Intn(len(agentNames))]
			m.log.Log("Squadron merge master selected: %s", m.mergeMaster)
		}

		channelName := "squadron-" + m.squadronName
		description := fmt.Sprintf("Squadron %s (%s)", m.squadronName, m.consensusType)
		if _, err := fleetctx.CreateChannel(m.fleet.FleetDir, channelName, description, agentNames); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				m.log.Log("ERROR: squadron channel create failed: %v", err)
				return m, tea.Quit
			}
		} else {
			m.log.Log("Squadron channel created: %s", channelName)
		}
		m.squadronChannelCreated = true
	}

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
	m.log.Log("Creating agent: name=%q branch=%q worktree=%q", item.AgentName, item.Branch, filepath.Join(m.fleet.FleetDir, "worktrees", item.AgentName))
	agent, err := m.fleet.AddAgent(item.AgentName, item.Branch)
	if err != nil {
		// If the agent already exists, reuse it instead of failing
		if strings.Contains(err.Error(), "already exists") {
			m.log.Log("Agent %q already exists, reusing", item.AgentName)
			agent, err = m.fleet.GetAgent(item.AgentName)
			if err != nil {
				m.log.Log("ERROR: GetAgent failed: %s", err)
				m.statusMsg = fmt.Sprintf("Failed to get existing agent: %s", err)
				return m, nil
			}
		} else {
			m.log.Log("ERROR: AddAgent failed: %s", err)
			m.statusMsg = fmt.Sprintf("Failed to create agent: %s", err)
			return m, nil
		}
	}
	m.log.Log("Agent created: worktree=%q", agent.WorktreePath)

	// Set up state tracking
	statesDir := filepath.Join(m.fleet.FleetDir, "states")
	if err := os.MkdirAll(statesDir, 0755); err != nil {
		m.log.Log("ERROR: Failed to create states dir: %s", err)
		m.statusMsg = fmt.Sprintf("Failed to create states dir: %s", err)
		return m, nil
	}
	stateFilePath := filepath.Join(statesDir, agent.Name+".json")

	// Inject hooks for state signaling
	drv, err := driver.GetForAgent(agent)
	if err != nil {
		drv, _ = driver.Get("") // default to claude-code
	}
	if err := drv.InjectHooks(agent.WorktreePath); err != nil {
		m.log.Log("WARNING: Hook injection failed for %q: %v", agent.Name, err)
		fmt.Fprintf(os.Stderr, "warning: could not inject hooks for agent '%s': %v\n", agent.Name, err)
		stateFilePath = ""
		m.fleet.UpdateAgentHooks(agent.Name, false)
	} else {
		m.log.Log("Hooks injected for %q", agent.Name)
		m.fleet.UpdateAgentHooks(agent.Name, true)
	}

	// Assemble full prompt with system prompt and roster
	fullPrompt := buildFullPrompt(m.systemPrompt, m.prompts, item)
	fullPrompt = m.applySquadronSuffixes(item.AgentName, fullPrompt)
	m.log.Log("Full prompt assembled: %d bytes", len(fullPrompt))

	// Write prompt to file to avoid shell metacharacter issues in tmux.
	// tmux concatenates command args and runs them through the shell, so
	// $PORT, ${...}, backticks, quotes etc. in the prompt would be expanded.
	promptsDir := filepath.Join(m.fleet.FleetDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		m.log.Log("ERROR: Failed to create prompts dir: %s", err)
		m.statusMsg = fmt.Sprintf("Failed to create prompts dir: %s", err)
		return m, nil
	}
	promptFile := filepath.Join(promptsDir, agent.Name+".txt")
	if err := os.WriteFile(promptFile, []byte(fullPrompt), 0644); err != nil {
		m.log.Log("ERROR: Failed to write prompt file: %s", err)
		m.statusMsg = fmt.Sprintf("Failed to write prompt file: %s", err)
		return m, nil
	}
	m.log.Log("Prompt written to file: %s (%d bytes)", promptFile, len(fullPrompt))

	// Create a launcher script that reads the prompt from file.
	launcherFile := filepath.Join(promptsDir, agent.Name+".sh")
	launcherScript := drv.BuildCommand(driver.LaunchOpts{
		YoloMode:   m.yoloMode,
		PromptFile: promptFile,
		AgentName:  agent.Name,
	})
	if err := os.WriteFile(launcherFile, []byte(launcherScript), 0755); err != nil {
		m.log.Log("ERROR: Failed to write launcher script: %s", err)
		m.statusMsg = fmt.Sprintf("Failed to write launcher script: %s", err)
		return m, nil
	}
	m.log.Log("Launcher script written: %s", launcherFile)

	command := []string{launcherFile}
	m.log.Log("Creating tmux session: name=%q launcher=%q prompt_bytes=%d", agent.Name, launcherFile, len(fullPrompt))
	if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, command, stateFilePath); err != nil {
		m.log.Log("ERROR: CreateSession failed: %s", err)
		m.statusMsg = fmt.Sprintf("Failed to create session: %s", err)
		return m, nil
	}
	m.log.Log("Tmux session created for %q", agent.Name)

	// Update state
	m.fleet.UpdateAgentStateFile(agent.Name, stateFilePath)
	pid, err := m.tmux.GetPID(agent.Name)
	if err != nil {
		m.log.Log("WARNING: Could not get PID for %q: %v", agent.Name, err)
		fmt.Fprintf(os.Stderr, "warning: launched agent '%s' but could not get PID: %v\n", agent.Name, err)
	} else {
		m.log.Log("Agent %q PID: %d", agent.Name, pid)
	}
	m.fleet.UpdateAgent(agent.Name, "running", pid)
	if item.Driver != "" && item.Driver != "claude-code" {
		m.fleet.UpdateAgentDriver(agent.Name, item.Driver)
	}
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
	// Rebuild viewport immediately so View() shows the new prompt content
	m.review.viewportIdx = -1
	m.setupPromptViewport()
	return m, nil
}

// applySquadronSuffixes assembles the final prompt for a squadron agent by
// appending consensus and merger suffixes then prepending any assigned persona.
// Returns basePrompt unchanged when squadronMode is false.
func (m LaunchModel) applySquadronSuffixes(agentName, basePrompt string) string {
	if !m.squadronMode {
		return basePrompt
	}

	agentNames := make([]string, 0, len(m.prompts))
	for _, p := range m.prompts {
		agentNames = append(agentNames, p.AgentName)
	}

	result := basePrompt

	switch m.consensusType {
	case "universal", "review_master", "none":
		if m.consensusType == "review_master" && agentName == m.reviewMaster {
			result += "\n" + squadron.BuildReviewMasterReviewerSuffix(m.squadronName, agentNames, m.baseBranch)
		} else {
			if suffix := squadron.BuildConsensusSuffix(m.consensusType, m.squadronName, agentNames, m.reviewMaster, m.baseBranch); suffix != "" {
				result += "\n" + suffix
			}
		}
	}

	if m.autoMerge && agentName == m.mergeMaster && m.mergeMaster != "" {
		agentBranches := make([]squadron.AgentBranch, 0, len(m.prompts))
		for _, p := range m.prompts {
			agentBranches = append(agentBranches, squadron.AgentBranch{Name: p.AgentName, Branch: p.Branch})
		}
		result += "\n" + squadron.BuildMergerSuffix(m.squadronName, m.baseBranch, agentBranches)
	}

	if key, ok := m.personas[agentName]; ok && key != "" {
		if p, ok := squadron.LookupPersona(key); ok {
			result = squadron.ApplyPersona(p, result)
		}
	}

	return result
}

// RunLaunch starts the launch TUI flow.
func RunLaunch(f *fleet.Fleet, yoloMode bool, skipYoloConfirm bool, noAutoMerge bool, useJumpSh bool) error {
	m := newLaunchModel(f, yoloMode, skipYoloConfirm, noAutoMerge, useJumpSh)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if m.log != nil {
		logPath := m.log.Path()
		m.log.Close()
		if logPath != "" {
			fmt.Fprintf(os.Stderr, "Launch log: %s\n", logPath)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to run launch TUI: %w", err)
	}

	// Show error from the model if launch failed
	if fm, ok := finalModel.(LaunchModel); ok && fm.statusMsg != "" {
		fmt.Fprintf(os.Stderr, "Launch error: %s\n", fm.statusMsg)
	}

	return nil
}

func RunSquadronLaunch(f *fleet.Fleet, useJumpSh bool) error {
	m := newSquadronLaunchModel(f, useJumpSh)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if m.log != nil {
		logPath := m.log.Path()
		m.log.Close()
		if logPath != "" {
			fmt.Fprintf(os.Stderr, "Launch log: %s\n", logPath)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to run squadron launch TUI: %w", err)
	}

	if fm, ok := finalModel.(LaunchModel); ok && fm.statusMsg != "" {
		fmt.Fprintf(os.Stderr, "Launch error: %s\n", fm.statusMsg)
	}
	return nil
}
