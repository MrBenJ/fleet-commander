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
	Name() string
	BuildCommand(opts LaunchOpts) string
	DetectState(bottomLines []string, fullContent string) *AgentState
	InjectHooks(worktreePath string) error
	RemoveHooks(worktreePath string) error
	CheckAvailable() error
}
