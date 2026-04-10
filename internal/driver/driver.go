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
	YoloMode   bool
	PromptFile string
	AgentName  string
}

// Driver defines the interface for a coding agent backend.
type Driver interface {
	// Name returns the driver identifier (e.g., "claude-code").
	Name() string

	// BuildCommand returns a shell script body (with shebang) that launches the
	// agent. The caller writes this to a .sh file and passes it to tmux.
	BuildCommand(opts LaunchOpts) string

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
