# Driver Interface + Claude Code Driver Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract all Claude Code-specific logic behind a `Driver` interface so fleet can support any coding agent.

**Architecture:** New `internal/driver/` package defines the `Driver` interface with 6 methods (Name, BuildCommand, DetectState, InjectHooks, RemoveHooks, CheckAvailable). The existing Claude Code behavior is extracted into a `ClaudeCodeDriver` that implements this interface. All call sites (start, stop, remove, launch, monitor, TUI) are updated to use the driver instead of hardcoded Claude Code logic. The `Agent` struct gets a `Driver` field that defaults to `"claude-code"`.

**Tech Stack:** Go, Cobra CLI, Bubble Tea TUI

---

## File Structure

```
internal/driver/
    driver.go          # Driver interface, AgentState type alias, LaunchOpts
    registry.go        # Get(name) function, Available() list
    claude_code.go     # ClaudeCodeDriver implementation
    claude_code_test.go
```

Modified files:
- `internal/fleet/fleet.go` — add `Driver` field to `Agent`
- `internal/monitor/monitor.go` — accept a driver for DetectState fallback
- `internal/tmux/tmux.go` — remove default "claude" command, always require explicit command
- `cmd/fleet/cmd_agents.go` — use driver in start/stop/remove
- `internal/tui/tui.go` — use driver in startAgentSession, kill
- `internal/tui/launch.go` — use driver in launchCurrent
- `cmd/fleet/main.go` — add --driver flag to addCmd

---

### Task 1: Create Driver Interface

**Files:**
- Create: `internal/driver/driver.go`

- [ ] **Step 1: Create the driver package with interface and types**

```go
package driver

// LaunchOpts contains options passed when launching an agent.
type LaunchOpts struct {
	YoloMode   bool   // Skip all permission prompts
	PromptFile string // Path to file containing the full prompt text
	AgentName  string // The fleet agent name
}

// Driver defines the interface for a coding agent backend.
type Driver interface {
	// Name returns the driver identifier (e.g., "claude-code").
	Name() string

	// BuildCommand returns the launcher script body to run the agent.
	// promptFile is the path to a file containing the prompt text.
	BuildCommand(opts LaunchOpts) string

	// DetectState analyzes tmux pane content to determine agent state.
	// bottomLines is the last ~15 non-empty lines from the pane.
	// Returns nil if the driver can't determine state from the pane content.
	DetectState(bottomLines []string, fullContent string) *AgentState

	// InjectHooks sets up hooks in the worktree for state signaling.
	// Returns nil if the driver doesn't support hooks.
	InjectHooks(worktreePath string) error

	// RemoveHooks cleans up hooks from the worktree.
	RemoveHooks(worktreePath string) error

	// CheckAvailable returns nil if the agent CLI is installed.
	CheckAvailable() error
}
```

Note: `AgentState` is not defined here — it stays in `internal/monitor/` where it already lives. Import it:

```go
import "github.com/MrBenJ/fleet-commander/internal/monitor"

// Re-export for convenience in driver implementations.
type AgentState = monitor.AgentState
```

Wait — this creates an import cycle: `monitor` would import `driver`, and `driver` would import `monitor`. Instead, define `AgentState` as a plain string type in the driver package and have monitor use the driver's type.

Actually, the cleanest approach: keep `AgentState` as a `string` in the driver package. The monitor package already defines it — we'll move the canonical definition to `driver` and have `monitor` import from there.

Update the file to be self-contained:

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
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./internal/driver/`
Expected: success (no output)

- [ ] **Step 3: Commit**

```bash
git add internal/driver/driver.go
git commit -m "feat(driver): add Driver interface and AgentState types"
```

---

### Task 2: Create Claude Code Driver

**Files:**
- Create: `internal/driver/claude_code.go`

- [ ] **Step 1: Implement ClaudeCodeDriver**

```go
package driver

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/MrBenJ/fleet-commander/internal/hooks"
)

// ClaudeCodeDriver implements Driver for Claude Code.
type ClaudeCodeDriver struct{}

func (d *ClaudeCodeDriver) Name() string { return "claude-code" }

func (d *ClaudeCodeDriver) BuildCommand(opts LaunchOpts) string {
	claudeArgs := ""
	if opts.YoloMode {
		claudeArgs = " --dangerously-skip-permissions"
	}
	return fmt.Sprintf("#!/usr/bin/env bash\nprompt=$(cat %q)\nexec claude%s -- \"$prompt\"\n", opts.PromptFile, claudeArgs)
}

func (d *ClaudeCodeDriver) CheckAvailable() error {
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude command not found in PATH")
	}
	return nil
}

func (d *ClaudeCodeDriver) InjectHooks(worktreePath string) error {
	return hooks.Inject(worktreePath)
}

func (d *ClaudeCodeDriver) RemoveHooks(worktreePath string) error {
	return hooks.Remove(worktreePath)
}
```

- [ ] **Step 2: Implement DetectState by extracting logic from monitor**

Move the pattern-matching logic from `internal/monitor/monitor.go:detectState()` into the driver. The driver's `DetectState` method takes `bottomLines` and `fullContent` (already stripped of ANSI):

```go
func (d *ClaudeCodeDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	bottomText := strings.Join(bottomLines, "\n")

	// Empty content = starting up
	if strings.TrimSpace(fullContent) == "" {
		s := StateStarting
		return &s
	}

	// ── WAITING PATTERNS (checked first) ──
	if strings.Contains(bottomText, "Esc to cancel") {
		s := StateWaiting
		return &s
	}
	if strings.Contains(bottomText, "Do you want to proceed") {
		s := StateWaiting
		return &s
	}
	if strings.Contains(bottomText, "accept edits") {
		s := StateWaiting
		return &s
	}
	if strings.Contains(bottomText, "shift+tab to cycle") {
		s := StateWaiting
		return &s
	}
	if strings.Contains(bottomText, "❯") && strings.Contains(bottomText, "1.") && strings.Contains(bottomText, "2.") {
		s := StateWaiting
		return &s
	}
	for _, line := range bottomLines {
		if strings.TrimSpace(line) == "❯" {
			s := StateWaiting
			return &s
		}
	}

	// Question heuristic: line ending with "?" in last 3 lines
	veryBottom := lastN(bottomLines, 3)
	for _, line := range veryBottom {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "?") && len(trimmed) > 10 {
			s := StateWaiting
			return &s
		}
		if strings.Contains(trimmed, "(y/n)") || strings.Contains(trimmed, "[Y/n]") {
			s := StateWaiting
			return &s
		}
	}

	// ── WORKING PATTERNS ──
	if strings.Contains(bottomText, "esc to interrupt") {
		s := StateWorking
		return &s
	}
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, sp := range spinners {
		if strings.Contains(bottomText, sp) {
			s := StateWorking
			return &s
		}
	}

	// Default: working (unrecognized output is usually active generation)
	s := StateWorking
	return &s
}

// lastN returns the last n elements of a slice, or the whole slice if shorter.
func lastN(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./internal/driver/`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/driver/claude_code.go
git commit -m "feat(driver): implement ClaudeCodeDriver"
```

---

### Task 3: Create Driver Registry

**Files:**
- Create: `internal/driver/registry.go`

- [ ] **Step 1: Implement the registry**

```go
package driver

import (
	"fmt"
	"sort"
)

var drivers = map[string]Driver{
	"claude-code": &ClaudeCodeDriver{},
}

// Get returns a driver by name. Empty string defaults to "claude-code".
func Get(name string) (Driver, error) {
	if name == "" {
		name = "claude-code"
	}
	d, ok := drivers[name]
	if !ok {
		return nil, fmt.Errorf("unknown driver %q (available: %v)", name, Available())
	}
	return d, nil
}

// Available returns sorted list of registered driver names.
func Available() []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./internal/driver/`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/driver/registry.go
git commit -m "feat(driver): add driver registry"
```

---

### Task 4: Write Claude Code Driver Tests

**Files:**
- Create: `internal/driver/claude_code_test.go`

- [ ] **Step 1: Write tests for BuildCommand**

```go
package driver

import (
	"strings"
	"testing"
)

func TestClaudeCodeBuildCommand(t *testing.T) {
	d := &ClaudeCodeDriver{}

	t.Run("normal mode", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/prompt.txt",
			AgentName:  "test-agent",
		})
		if !strings.Contains(script, "exec claude") {
			t.Error("expected 'exec claude' in script")
		}
		if strings.Contains(script, "--dangerously-skip-permissions") {
			t.Error("should not contain --dangerously-skip-permissions in normal mode")
		}
		if !strings.Contains(script, "/tmp/prompt.txt") {
			t.Error("expected prompt file path in script")
		}
	})

	t.Run("yolo mode", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			YoloMode:   true,
			PromptFile: "/tmp/prompt.txt",
			AgentName:  "test-agent",
		})
		if !strings.Contains(script, "--dangerously-skip-permissions") {
			t.Error("expected --dangerously-skip-permissions in yolo mode")
		}
	})
}
```

- [ ] **Step 2: Write tests for DetectState**

Port the existing tests from `internal/monitor/detect_test.go` to use the driver interface:

```go
func TestClaudeCodeDetectState(t *testing.T) {
	d := &ClaudeCodeDriver{}

	tests := []struct {
		name        string
		fullContent string
		want        AgentState
	}{
		{"esc to interrupt", "Claude is thinking...\nesc to interrupt", StateWorking},
		{"spinner char", "Processing ⠋", StateWorking},
		{"Esc to cancel", "Do you want to run this?\nEsc to cancel", StateWaiting},
		{"Do you want to proceed", "Do you want to proceed", StateWaiting},
		{"accept edits", "Review the changes\naccept edits", StateWaiting},
		{"shift+tab to cycle", "Select an option\nshift+tab to cycle", StateWaiting},
		{"bare arrow prompt", "Some output\n❯", StateWaiting},
		{"question in last 3 lines", "line1\nline2\nShould I continue with this approach?", StateWaiting},
		{"y/n prompt", "Delete the file? (y/n)", StateWaiting},
		{"Y/n prompt", "Delete the file? [Y/n]", StateWaiting},
		{"empty content", "", StateStarting},
		{"whitespace only", "   \n\n   ", StateStarting},
		{"no matching patterns", "Claude finished the task.", StateWorking},
		{"waiting beats working", "esc to interrupt\n(y/n)", StateWaiting},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.fullContent, "\n")
			// Get bottom 15 non-empty lines (same as monitor)
			var bottomLines []string
			for i := len(lines) - 1; i >= 0 && len(bottomLines) < 15; i-- {
				if strings.TrimSpace(lines[i]) != "" {
					bottomLines = append(bottomLines, lines[i])
				}
			}
			result := d.DetectState(bottomLines, tt.fullContent)
			if result == nil {
				t.Fatal("DetectState returned nil")
			}
			if *result != tt.want {
				t.Errorf("DetectState() = %q, want %q", *result, tt.want)
			}
		})
	}
}

func TestClaudeCodeCheckAvailable(t *testing.T) {
	// Just verify it doesn't panic; actual availability depends on environment
	d := &ClaudeCodeDriver{}
	_ = d.CheckAvailable()
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/driver/ -v`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/driver/claude_code_test.go
git commit -m "test(driver): add ClaudeCodeDriver tests"
```

---

### Task 5: Add Driver Field to Agent Struct

**Files:**
- Modify: `internal/fleet/fleet.go:33-41`

- [ ] **Step 1: Add Driver field to Agent struct**

In `internal/fleet/fleet.go`, add the `Driver` field to the `Agent` struct:

```go
// Agent represents a single agent workspace
type Agent struct {
	Name          string `json:"name"`
	Branch        string `json:"branch"`
	WorktreePath  string `json:"worktree_path"`
	Status        string `json:"status"`
	PID           int    `json:"pid"`
	StateFilePath string `json:"state_file_path,omitempty"`
	HooksOK       bool   `json:"hooks_ok"`
	Driver        string `json:"driver,omitempty"`
}
```

When `Driver` is empty, callers use `driver.Get("")` which defaults to `"claude-code"`.

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/fleet/fleet.go
git commit -m "feat(fleet): add Driver field to Agent struct"
```

---

### Task 6: Add `--driver` Flag to `fleet add`

**Files:**
- Modify: `cmd/fleet/cmd_agents.go:17-39` (addCmd)
- Modify: `cmd/fleet/main.go:69` (init function, register flag)

- [ ] **Step 1: Add --driver flag registration in main.go init()**

In `cmd/fleet/main.go`, in the `init()` function, after `rootCmd.AddCommand(addCmd)` on line 73, add:

```go
addCmd.Flags().String("driver", "claude-code", "Coding agent driver (claude-code, codex, aider, generic)")
```

- [ ] **Step 2: Update addCmd to read driver flag and validate**

In `cmd/fleet/cmd_agents.go`, update the `addCmd` RunE to read and validate the driver flag, and set it on the agent:

```go
var addCmd = &cobra.Command{
	Use:   "add [name] [branch]",
	Short: "Add a new agent workspace",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		branch := args[1]
		driverName, _ := cmd.Flags().GetString("driver")

		// Validate driver name
		if _, err := driver.Get(driverName); err != nil {
			return err
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		agent, err := f.AddAgent(name, branch)
		if err != nil {
			return fmt.Errorf("failed to add agent: %w", err)
		}

		// Set driver on agent (empty means default claude-code)
		if driverName != "claude-code" {
			f.UpdateAgentDriver(name, driverName)
		}

		fmt.Printf("Agent '%s' created on branch '%s'\n", agent.Name, agent.Branch)
		fmt.Printf("Worktree: %s\n", agent.WorktreePath)
		if driverName != "claude-code" {
			fmt.Printf("Driver: %s\n", driverName)
		}
		return nil
	},
}
```

Add the import for the driver package at the top of `cmd/fleet/cmd_agents.go`:
```go
"github.com/MrBenJ/fleet-commander/internal/driver"
```

- [ ] **Step 3: Add UpdateAgentDriver method to fleet.go**

In `internal/fleet/fleet.go`, add:

```go
// UpdateAgentDriver sets the driver for an agent.
func (f *Fleet) UpdateAgentDriver(name, driverName string) error {
	return f.withLock(func() error {
		for _, a := range f.Agents {
			if a.Name == name {
				a.Driver = driverName
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./cmd/fleet/`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add cmd/fleet/main.go cmd/fleet/cmd_agents.go internal/fleet/fleet.go
git commit -m "feat(cli): add --driver flag to fleet add command"
```

---

### Task 7: Update `fleet start` to Use Driver

**Files:**
- Modify: `cmd/fleet/cmd_agents.go:139-197` (startCmd)

- [ ] **Step 1: Rewrite startCmd to use the driver**

Replace the hardcoded hooks.Inject and tmux.CreateSession(... nil ...) calls with driver-based logic:

```go
var startCmd = &cobra.Command{
	Use:   "start [agent-name]",
	Short: "Start an agent's tmux session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		agent, err := f.GetAgent(agentName)
		if err != nil {
			return err
		}

		drv, err := driver.Get(agent.Driver)
		if err != nil {
			return err
		}

		tm := tmux.NewManager(f.TmuxPrefix())
		if !tm.IsAvailable() {
			return fmt.Errorf("tmux is not installed")
		}

		if !tm.SessionExists(agentName) {
			// Check agent CLI is available
			if err := drv.CheckAvailable(); err != nil {
				return err
			}

			statesDir := filepath.Join(f.FleetDir, "states")
			if err := os.MkdirAll(statesDir, 0755); err != nil {
				return fmt.Errorf("failed to create states dir: %w", err)
			}
			stateFilePath := filepath.Join(statesDir, agentName+".json")

			// Inject hooks via driver
			if err := drv.InjectHooks(agent.WorktreePath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not inject hooks for '%s': %v\n", agentName, err)
				stateFilePath = ""
				f.UpdateAgentHooks(agentName, false)
			} else {
				f.UpdateAgentHooks(agentName, true)
			}

			// Create tmux session with driver's default command
			// For `fleet start` (no prompt), just launch the agent bare
			if err := tm.CreateSession(agentName, agent.WorktreePath, nil, stateFilePath); err != nil {
				return fmt.Errorf("failed to create tmux session: %w", err)
			}
			fmt.Printf("Created tmux session for agent '%s'\n", agentName)

			f.UpdateAgentStateFile(agentName, stateFilePath)
		}

		pid, err := tm.GetPID(agentName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not get PID for agent '%s': %v\n", agentName, err)
		}
		f.UpdateAgent(agentName, "running", pid)

		fmt.Printf("Agent '%s' is running in tmux session: %s\n", agentName, tm.SessionName(agentName))
		fmt.Printf("Attach with: fleet attach %s\n", agentName)
		return nil
	},
}
```

- [ ] **Step 2: Update tmux.CreateSession to use driver for default command**

In `internal/tmux/tmux.go`, update `CreateSession` so that when `command` is nil, it no longer defaults to `"claude"`. Instead, it requires either a command or the caller must provide one. The simplest change: when command is empty, just start a shell (the agent runs interactively):

Replace lines 137-162 in `tmux.go`:

```go
	// Check if the specified command is available when using a custom command
	if len(command) > 0 {
		// Verify the first element (the binary) exists
		if _, err := m.runner.LookPath(command[0]); err != nil {
			return fmt.Errorf("%s command not found in PATH", command[0])
		}
	}

	// Build tmux command: new-session -d -s <name> -c <path> <command>
	args := []string{
		"new-session",
		"-d",
		"-s", sessionName,
		"-c", worktreePath,
	}

	// Add environment variables if stateFilePath is provided
	if stateFilePath != "" {
		args = append(args, "-e", fmt.Sprintf("FLEET_AGENT_NAME=%s", agentName))
		args = append(args, "-e", fmt.Sprintf("FLEET_STATE_FILE=%s", stateFilePath))
	}

	if len(command) > 0 {
		args = append(args, command...)
	}
	// When no command is given, tmux starts the user's default shell
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./...`
Expected: success

- [ ] **Step 4: Run tests**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./...`
Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add cmd/fleet/cmd_agents.go internal/tmux/tmux.go
git commit -m "feat(start): use driver interface for agent session creation"
```

---

### Task 8: Update `fleet stop` and `fleet remove` to Use Driver

**Files:**
- Modify: `cmd/fleet/cmd_agents.go:230-274` (stopCmd)
- Modify: `cmd/fleet/cmd_agents.go:276-347` (removeCmd)

- [ ] **Step 1: Update stopCmd to use driver for hook removal**

In the stopCmd RunE, replace the direct `hooks.Remove()` call with driver-based removal:

After `agent, err := f.GetAgent(agentName)` (which is currently done after the state file check on line 256), get the driver and use it:

```go
		// Get driver for hook cleanup
		drv, err := driver.Get(agent.Driver)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: unknown driver %q, skipping hook removal\n", agent.Driver)
		}

		// Clean up state file so monitor doesn't show stale state
		if agent.StateFilePath != "" {
			if err := os.Remove(agent.StateFilePath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not remove state file: %v\n", err)
			}
			f.UpdateAgentStateFile(agentName, "")
		}

		// Remove hooks via driver
		if drv != nil {
			if err := drv.RemoveHooks(agent.WorktreePath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not remove hooks: %v\n", err)
			}
		}
		f.UpdateAgentHooks(agentName, false)
```

- [ ] **Step 2: Update removeCmd to use driver for hook removal**

In removeCmd RunE, replace `hooks.Remove(agent.WorktreePath)` with:

```go
		// Remove hooks via driver
		drv, _ := driver.Get(agent.Driver)
		if drv != nil {
			if err := drv.RemoveHooks(agent.WorktreePath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not remove hooks: %v\n", err)
			}
		}
```

- [ ] **Step 3: Remove the direct hooks import from cmd_agents.go if no longer needed**

Check if `hooks` is still imported anywhere in `cmd/fleet/cmd_agents.go`. After these changes it should not be — remove it from the import block.

- [ ] **Step 4: Verify it compiles and tests pass**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./... && go test ./...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add cmd/fleet/cmd_agents.go
git commit -m "feat(stop/remove): use driver interface for hook removal"
```

---

### Task 9: Update Monitor to Use Driver for DetectState

**Files:**
- Modify: `internal/monitor/monitor.go`

- [ ] **Step 1: Add driver parameter to Monitor**

Update the Monitor struct and constructors to accept a driver (optional):

```go
import "github.com/MrBenJ/fleet-commander/internal/driver"

// Monitor watches agent tmux sessions and detects their state
type Monitor struct {
	tmux      *tmux.Manager
	snapshots map[string]*Snapshot
	drivers   map[string]driver.Driver // agentName -> driver
}

// NewMonitor creates a new agent monitor
func NewMonitor(tm *tmux.Manager) *Monitor {
	return &Monitor{
		tmux:      tm,
		snapshots: make(map[string]*Snapshot),
		drivers:   make(map[string]driver.Driver),
	}
}

// SetDriver registers a driver for an agent's state detection.
func (m *Monitor) SetDriver(agentName string, drv driver.Driver) {
	m.drivers[agentName] = drv
}
```

- [ ] **Step 2: Update Check to use the driver's DetectState**

Modify the `Check` method to use the registered driver when available:

```go
func (m *Monitor) Check(agentName string) *Snapshot {
	snap := &Snapshot{
		AgentName: agentName,
		Timestamp: time.Now(),
	}

	if !m.tmux.SessionExists(agentName) {
		snap.State = StateStopped
		m.snapshots[agentName] = snap
		return snap
	}

	content, err := m.tmux.CapturePane(agentName)
	if err != nil {
		snap.State = StateStopped
		m.snapshots[agentName] = snap
		return snap
	}

	snap.Content = content
	snap.LastLine = getLastNonEmptyLine(content)

	// Try driver-based detection first
	if drv, ok := m.drivers[agentName]; ok {
		stripped := stripANSI(content)
		allLines := strings.Split(stripped, "\n")
		bottomLines := getLastNonEmptyLines(allLines, paneBottomLines)
		if state := drv.DetectState(bottomLines, stripped); state != nil {
			snap.State = AgentState(*state)
			m.snapshots[agentName] = snap
			return snap
		}
	}

	// Fallback to legacy detectState
	snap.State = detectState(snap.LastLine, content)
	m.snapshots[agentName] = snap
	return snap
}
```

Note: The `AgentState` type in monitor and driver are both `string`-based. We need to handle the conversion. The simplest approach: cast `driver.AgentState` to `monitor.AgentState` since they're both string types with the same values:

```go
snap.State = AgentState(*state)
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./...`
Expected: success

- [ ] **Step 4: Run tests**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./...`
Expected: all pass (existing tests don't set a driver, so they use the legacy fallback path)

- [ ] **Step 5: Commit**

```bash
git add internal/monitor/monitor.go
git commit -m "feat(monitor): support driver-based state detection with legacy fallback"
```

---

### Task 10: Update TUI Queue to Use Driver

**Files:**
- Modify: `internal/tui/tui.go:117-140` (startAgentSession)
- Modify: `internal/tui/tui.go:230-256` (kill shortcut)

- [ ] **Step 1: Update startAgentSession to use driver**

Replace the direct hooks.Inject call and nil command:

```go
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

	if err := m.tmux.CreateSession(agent.Name, agent.WorktreePath, nil, stateFilePath); err != nil {
		return err
	}

	// Register driver with monitor for state detection
	m.monitor.SetDriver(agent.Name, drv)

	m.fleet.UpdateAgentStateFile(agent.Name, stateFilePath)
	return nil
}
```

- [ ] **Step 2: Update kill shortcut to use driver for hook removal**

In the `"k"` key handler (~line 232-256), replace `hooks.Remove` with driver:

```go
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
```

- [ ] **Step 3: Update imports — replace hooks import with driver import**

In `internal/tui/tui.go`, replace `"github.com/MrBenJ/fleet-commander/internal/hooks"` with `"github.com/MrBenJ/fleet-commander/internal/driver"`.

- [ ] **Step 4: Register drivers for existing agents in buildItems**

In the `buildItems` function, register drivers with the monitor so `CheckWithStateFile` can use them:

```go
func buildItems(f *fleet.Fleet, tm *tmux.Manager, mon *monitor.Monitor) []list.Item {
	items := []list.Item{AddNewItem{}}

	for _, a := range f.Agents {
		// Register driver for state detection
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
```

- [ ] **Step 5: Verify it compiles and tests pass**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./... && go test ./...`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add internal/tui/tui.go
git commit -m "feat(tui): use driver interface for hooks and state detection"
```

---

### Task 11: Update Launch Flow to Use Driver

**Files:**
- Modify: `internal/tui/launch.go:291-426` (launchCurrent)

- [ ] **Step 1: Update launchCurrent to use driver for hooks and BuildCommand**

In `launchCurrent()`, replace the hardcoded hooks.Inject, launcher script generation, and claude-specific command building with driver-based logic.

The key changes in `launchCurrent()`:

After creating the agent (~line 344), get the driver:

```go
	// Get driver for this agent
	drv, err := driver.Get(item.Driver)
	if err != nil {
		// Default to claude-code for launch items (they don't have a Driver field yet)
		drv, err = driver.Get("")
		if err != nil {
			m.statusMsg = fmt.Sprintf("Failed to get driver: %s", err)
			return m, nil
		}
	}
```

Wait — `LaunchItem` doesn't have a `Driver` field. We need to add one.

- [ ] **Step 2: Add Driver field to LaunchItem**

In `internal/tui/parse.go` (line 10), add the field:

```go
type LaunchItem struct {
	Prompt    string
	AgentName string
	Branch    string
	Driver    string // coding agent driver (default: "claude-code")
}
```

- [ ] **Step 3: Replace hooks and launcher script in launchCurrent**

Replace the hooks injection section (~lines 357-365) with:

```go
	// Inject hooks via driver
	drv, err := driver.Get(item.Driver)
	if err != nil {
		drv, _ = driver.Get("") // default
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
```

Replace the launcher script generation (~lines 390-403) with:

```go
	// Build launcher script via driver
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
```

Also set the agent's driver in the config:

```go
	if item.Driver != "" && item.Driver != "claude-code" {
		m.fleet.UpdateAgentDriver(agent.Name, item.Driver)
	}
```

- [ ] **Step 4: Remove direct hooks import from launch.go**

Replace `"github.com/MrBenJ/fleet-commander/internal/hooks"` with `"github.com/MrBenJ/fleet-commander/internal/driver"` in the import block.

- [ ] **Step 5: Verify it compiles and tests pass**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./... && go test ./...`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add internal/tui/launch.go internal/tui/parse.go
git commit -m "feat(launch): use driver interface for agent launching"
```

---

### Task 12: Update Multi-Repo TUI

**Files:**
- Modify: `internal/tui/multi_repo.go`

- [ ] **Step 1: Check multi_repo.go for hooks/monitor usage**

Read `internal/tui/multi_repo.go` and update any direct `hooks.Inject`/`hooks.Remove` calls or monitor usage to use the driver. The grep showed line 138 has `CheckWithStateFile` — add driver registration there too:

```go
// Register driver for state detection
if drv, err := driver.Get(a.Driver); err == nil {
	p.mon.SetDriver(a.Name, drv)
}
snap := p.mon.CheckWithStateFile(a.Name, a.StateFilePath)
```

Update imports accordingly.

- [ ] **Step 2: Verify it compiles and tests pass**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./... && go test ./...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/tui/multi_repo.go
git commit -m "feat(multi-repo): use driver for state detection"
```

---

### Task 13: Update Descriptions and Help Text

**Files:**
- Modify: `cmd/fleet/main.go:22-29` (rootCmd Long description)
- Modify: `cmd/fleet/cmd_launch.go:14-16` (launchCmd Long description)
- Modify: `cmd/fleet/cmd_utils.go:61` (signalCmd Short description)

- [ ] **Step 1: Update root command description**

```go
Long: `Fleet Commander lets you run multiple AI coding agents in parallel,
each on different branches. When agents need input, they queue up.

Quick start:
  fleet init ~/projects/my-app
  fleet add feat-auth "feature/auth"
  fleet add bug-fix "bugfix/login" --driver codex
  fleet queue`,
```

- [ ] **Step 2: Update launch command description**

```go
Long: `Enter a list of tasks or prompts, review auto-generated agent names
and branches, then launch them all as parallel coding agent sessions.

Each prompt becomes a separate agent with its own git worktree.`,
```

- [ ] **Step 3: Update signal command description**

```go
Short: "Write agent state (called by coding agent hooks)",
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/bjunya/code/fleet-commander && go build ./cmd/fleet/`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add cmd/fleet/main.go cmd/fleet/cmd_launch.go cmd/fleet/cmd_utils.go
git commit -m "docs: update CLI descriptions to be agent-agnostic"
```

---

### Task 14: Final Integration Test

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./... -v`
Expected: all tests pass

- [ ] **Step 2: Build the binary**

Run: `cd /Users/bjunya/code/fleet-commander && go build -o fleet ./cmd/fleet/`
Expected: success

- [ ] **Step 3: Verify driver registry works**

Run: `cd /Users/bjunya/code/fleet-commander && go test -run TestClaudeCode ./internal/driver/ -v`
Expected: all ClaudeCode tests pass

- [ ] **Step 4: Verify no remaining direct hooks imports outside driver package**

Run: `cd /Users/bjunya/code/fleet-commander && grep -r '"github.com/MrBenJ/fleet-commander/internal/hooks"' --include='*.go' | grep -v '_test.go' | grep -v 'internal/driver/' | grep -v 'internal/hooks/'`
Expected: no output (no files outside driver/hooks packages import hooks directly)

If any remain, update them to use the driver package instead.

- [ ] **Step 5: Commit any remaining fixes**

```bash
git add -A
git commit -m "chore: final cleanup for driver interface migration"
```
