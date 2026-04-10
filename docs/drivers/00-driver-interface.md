# Driver Interface Reference

This document defines the `Driver` interface that all agent drivers must implement. It lives at `internal/driver/driver.go`.

## Interface Definition

```go
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
    // Name returns the driver identifier (e.g., "claude-code", "codex", "aider", "generic").
    Name() string

    // BuildCommand returns the shell command (as a launcher script body) to run
    // the agent with the given prompt. The caller writes this to a .sh file and
    // passes it as the tmux session command.
    //
    // promptFile is the path to a file containing the full prompt text.
    // This avoids shell metacharacter issues.
    BuildCommand(opts LaunchOpts) string

    // DetectState analyzes tmux pane content to determine agent state.
    // bottomLines is the last ~15 non-empty lines from the pane.
    // Returns nil if the driver doesn't support pane-based detection
    // (state file will be used as sole mechanism).
    DetectState(bottomLines []string, fullContent string) *AgentState

    // InjectHooks sets up hooks/integrations in the worktree for state signaling.
    // The hooks should call "fleet signal waiting" and "fleet signal working"
    // at appropriate times. Returns nil if the driver doesn't support hooks.
    InjectHooks(worktreePath string) error

    // RemoveHooks cleans up any hooks/integrations from the worktree.
    RemoveHooks(worktreePath string) error

    // CheckAvailable returns nil if the agent CLI is installed and accessible.
    // Returns a descriptive error if not (e.g., "codex not found in PATH").
    CheckAvailable() error
}
```

## How the Interface Is Used

### Session Creation (`fleet start`, `fleet launch`)
1. `driver.CheckAvailable()` — fail fast if CLI missing
2. `driver.InjectHooks(worktreePath)` — set up state signaling (best-effort)
3. `driver.BuildCommand(opts)` — get the launcher script content
4. Write launcher script to `.fleet/prompts/<agent>.sh`
5. Pass launcher script as tmux session command

### State Detection (`internal/monitor/`)
1. Check state file first (written by `fleet signal` — works for any driver with hooks)
2. If state file absent/stale, call `driver.DetectState(bottomLines, fullContent)`
3. If driver returns nil, fall back to generic heuristics or default to "working"

### Session Teardown (`fleet stop`, `fleet remove`)
1. Kill tmux session
2. `driver.RemoveHooks(worktreePath)` — clean up
3. Remove state file

## Agent Config

The `Agent` struct in `internal/fleet/fleet.go` gets a new `Driver` field:

```go
type Agent struct {
    Name          string `json:"name"`
    Branch        string `json:"branch"`
    WorktreePath  string `json:"worktree_path"`
    Status        string `json:"status"`
    PID           int    `json:"pid"`
    StateFilePath string `json:"state_file_path,omitempty"`
    HooksOK       bool   `json:"hooks_ok"`
    Driver        string `json:"driver,omitempty"` // NEW: "claude-code", "codex", "aider", "generic"
}
```

When `Driver` is empty, it defaults to `"claude-code"` for backward compatibility.

## Driver Registry

A simple registry at `internal/driver/registry.go`:

```go
var drivers = map[string]Driver{
    "claude-code": &ClaudeCodeDriver{},
    "codex":       &CodexDriver{},
    "aider":       &AiderDriver{},
    "generic":     &GenericDriver{},
}

func Get(name string) (Driver, error) {
    if name == "" {
        name = "claude-code"
    }
    d, ok := drivers[name]
    if !ok {
        return nil, fmt.Errorf("unknown driver %q", name)
    }
    return d, nil
}

func Available() []string { /* return sorted keys */ }
```

## File Layout

```
internal/driver/
    driver.go        # Interface + types + AgentState constants
    registry.go      # Driver registry (Get, Available)
    claude_code.go   # Claude Code driver
    codex.go         # Codex CLI driver
    aider.go         # Aider driver
    generic.go       # Generic/custom driver
```
