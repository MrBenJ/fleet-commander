package driver

// AgentState represents what the agent is currently doing.
type AgentState string

const (
	StateWorking  AgentState = "working"
	StateWaiting  AgentState = "waiting"
	StateStopped  AgentState = "stopped"
	StateStarting AgentState = "starting"
)

// LaunchOpts contains options passed when launching an agent.
type LaunchOpts struct {
	YoloMode      bool   // Skip all permission prompts / reviews
	PromptFile    string // Path to file containing the full prompt text
	StateFilePath string // Path where the agent should write state updates
	AgentName     string // The fleet agent name
}

// Driver defines the interface for a coding agent backend.
type Driver interface {
	// Name returns the driver identifier (e.g., "claude-code").
	Name() string

	// InteractiveCommand returns the command and args to launch the agent in
	// interactive mode (no prompt). Used by "fleet start" and the queue TUI.
	InteractiveCommand() []string

	// BuildCommand returns a shell script body (with shebang) that launches the
	// agent with a prompt read from opts.PromptFile. Used by "fleet launch".
	BuildCommand(opts LaunchOpts) string

	// PlanCommand runs the agent as a one-shot planner: sends prompt, returns
	// stdout+stderr. Used by "fleet launch" to expand user tasks into structured
	// JSON (agent names, branches, prompts).
	PlanCommand(prompt string) ([]byte, error)

	// DetectState analyzes tmux pane content to determine agent state.
	// bottomLines contains the last ~15 non-empty lines from the pane.
	// fullContent is the complete pane text. Both are already ANSI-stripped.
	// Returns nil if the driver cannot determine state (caller falls back to
	// legacy heuristics or defaults to StateWorking).
	DetectState(bottomLines []string, fullContent string) *AgentState

	// InjectHooks sets up hooks in the worktree for state signaling.
	// Returns nil if the driver doesn't support hooks (no-op).
	InjectHooks(worktreePath string) error

	// RemoveHooks cleans up hooks from the worktree.
	RemoveHooks(worktreePath string) error

	// CheckAvailable returns nil if the agent CLI is installed and accessible.
	CheckAvailable() error
}
