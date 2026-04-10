# Generic Driver Implementation Plan

## Overview

Implement the `generic` driver for Fleet Commander, enabling any terminal-based coding agent or command as a backend. This is the escape hatch — users provide their own command and optionally their own state detection patterns.

**Prerequisites:** The driver interface and Claude Code driver must already exist at `internal/driver/`. Read `docs/drivers/00-driver-interface.md` for the full interface definition.

## Background

The generic driver is for agents that don't have a dedicated driver — Cursor CLI, custom scripts, or any future agent. Users configure:
1. The command to run (required)
2. Optional regex patterns for detecting "waiting" state
3. Optional regex patterns for detecting "working" state

If no patterns are provided, state detection relies solely on the state file mechanism (if the agent supports `fleet signal`) or defaults to "working".

## Configuration

The generic driver reads its configuration from the fleet config. The `Agent` struct gets an optional `DriverConfig` field:

```go
type Agent struct {
    Name          string            `json:"name"`
    Branch        string            `json:"branch"`
    WorktreePath  string            `json:"worktree_path"`
    Status        string            `json:"status"`
    PID           int               `json:"pid"`
    StateFilePath string            `json:"state_file_path,omitempty"`
    HooksOK       bool              `json:"hooks_ok"`
    Driver        string            `json:"driver,omitempty"`
    DriverConfig  *DriverConfig     `json:"driver_config,omitempty"` // NEW
}

type DriverConfig struct {
    Command         string   `json:"command"`                    // e.g., "cursor-cli" or "/path/to/my-agent"
    Args            []string `json:"args,omitempty"`             // additional CLI args
    YoloArgs        []string `json:"yolo_args,omitempty"`        // args added in yolo mode
    PromptFlag      string   `json:"prompt_flag,omitempty"`      // flag for passing prompt, e.g., "--message" (default: positional)
    PromptFromFile  bool     `json:"prompt_from_file,omitempty"` // if true, pass prompt file path instead of content
    WaitingPatterns []string `json:"waiting_patterns,omitempty"` // regex patterns indicating "waiting for input"
    WorkingPatterns []string `json:"working_patterns,omitempty"` // regex patterns indicating "working"
}
```

### Example Fleet Config

```json
{
  "agents": [
    {
      "name": "my-agent",
      "branch": "fleet/my-agent",
      "driver": "generic",
      "driver_config": {
        "command": "cursor-cli",
        "args": ["--no-gui"],
        "yolo_args": ["--auto-approve"],
        "prompt_flag": "--message",
        "waiting_patterns": ["\\$\\s*$", "\\?>\\s*$", "\\[Y/n\\]"],
        "working_patterns": ["Running\\.\\.\\.", "Processing"]
      }
    }
  ]
}
```

## Implementation Steps

### Step 1: Add `DriverConfig` to the Agent Struct

In `internal/fleet/fleet.go`, add the `DriverConfig` struct and field to `Agent`. See the Configuration section above.

**Important:** `DriverConfig` is defined in the `fleet` package (not `driver`) because it's part of the persisted config. The generic driver receives it as a parameter.

### Step 2: Create `internal/driver/generic.go`

```go
package driver

import (
    "fmt"
    "os/exec"
    "regexp"
    "strings"
)

// GenericConfig mirrors fleet.DriverConfig but lives in the driver package.
// Passed to the generic driver at construction time.
type GenericConfig struct {
    Command         string
    Args            []string
    YoloArgs        []string
    PromptFlag      string
    PromptFromFile  bool
    WaitingPatterns []*regexp.Regexp
    WorkingPatterns []*regexp.Regexp
}

type GenericDriver struct {
    config GenericConfig
}

func NewGenericDriver(config GenericConfig) *GenericDriver {
    return &GenericDriver{config: config}
}

func (d *GenericDriver) Name() string { return "generic" }
```

#### `BuildCommand`

```go
func (d *GenericDriver) BuildCommand(opts LaunchOpts) string {
    var sb strings.Builder
    sb.WriteString("#!/usr/bin/env bash\n")

    args := make([]string, len(d.config.Args))
    copy(args, d.config.Args)

    if opts.YoloMode {
        args = append(args, d.config.YoloArgs...)
    }

    if d.config.PromptFromFile {
        // Pass the prompt file path directly
        if d.config.PromptFlag != "" {
            args = append(args, d.config.PromptFlag, opts.PromptFile)
        } else {
            args = append(args, opts.PromptFile)
        }
        sb.WriteString(fmt.Sprintf("exec %s %s\n", d.config.Command, strings.Join(quoteArgs(args), " ")))
    } else {
        // Read prompt from file and pass as argument
        sb.WriteString(fmt.Sprintf("prompt=$(cat %q)\n", opts.PromptFile))
        if d.config.PromptFlag != "" {
            args = append(args, d.config.PromptFlag, "$prompt")
        } else {
            args = append(args, "\"$prompt\"")
        }
        sb.WriteString(fmt.Sprintf("exec %s %s\n", d.config.Command, strings.Join(args, " ")))
    }

    return sb.String()
}
```

#### `DetectState`

```go
func (d *GenericDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
    if len(d.config.WaitingPatterns) == 0 && len(d.config.WorkingPatterns) == 0 {
        return nil // no patterns configured, fall back to default
    }

    bottomText := strings.Join(bottomLines, "\n")

    // Check waiting patterns first (same priority as Claude Code driver)
    for _, pat := range d.config.WaitingPatterns {
        if pat.MatchString(bottomText) {
            state := StateWaiting
            return &state
        }
    }

    // Check working patterns
    for _, pat := range d.config.WorkingPatterns {
        if pat.MatchString(bottomText) {
            state := StateWorking
            return &state
        }
    }

    return nil
}
```

#### `InjectHooks` / `RemoveHooks`

No-op. Generic agents don't have a standard hook mechanism:

```go
func (d *GenericDriver) InjectHooks(worktreePath string) error { return nil }
func (d *GenericDriver) RemoveHooks(worktreePath string) error { return nil }
```

However, document that users CAN integrate with fleet's state signaling manually by having their agent/script call `fleet signal waiting` and `fleet signal working`. The environment variables `FLEET_AGENT_NAME` and `FLEET_STATE_FILE` are set in the tmux session regardless of driver.

#### `CheckAvailable`

```go
func (d *GenericDriver) CheckAvailable() error {
    if d.config.Command == "" {
        return fmt.Errorf("generic driver requires a 'command' in driver_config")
    }
    if _, err := exec.LookPath(d.config.Command); err != nil {
        return fmt.Errorf("%s command not found in PATH", d.config.Command)
    }
    return nil
}
```

### Step 3: Driver Registry Integration

The generic driver is special — it's not a singleton like the others. It needs config from the agent. Update the registry to handle this:

```go
// GetForAgent returns the driver for an agent, constructing GenericDriver
// with agent-specific config if needed.
func GetForAgent(agent *fleet.Agent) (Driver, error) {
    if agent.Driver == "generic" {
        if agent.DriverConfig == nil {
            return nil, fmt.Errorf("generic driver requires driver_config on the agent")
        }
        config, err := parseGenericConfig(agent.DriverConfig)
        if err != nil {
            return nil, fmt.Errorf("invalid driver_config: %w", err)
        }
        return NewGenericDriver(config), nil
    }
    return Get(agent.Driver)
}

func parseGenericConfig(dc *fleet.DriverConfig) (GenericConfig, error) {
    config := GenericConfig{
        Command:        dc.Command,
        Args:           dc.Args,
        YoloArgs:       dc.YoloArgs,
        PromptFlag:     dc.PromptFlag,
        PromptFromFile: dc.PromptFromFile,
    }

    for _, p := range dc.WaitingPatterns {
        re, err := regexp.Compile(p)
        if err != nil {
            return config, fmt.Errorf("invalid waiting pattern %q: %w", p, err)
        }
        config.WaitingPatterns = append(config.WaitingPatterns, re)
    }
    for _, p := range dc.WorkingPatterns {
        re, err := regexp.Compile(p)
        if err != nil {
            return config, fmt.Errorf("invalid working pattern %q: %w", p, err)
        }
        config.WorkingPatterns = append(config.WorkingPatterns, re)
    }

    return config, nil
}
```

### Step 4: CLI Support for `--driver` and `--driver-config`

Update `fleet add` to accept generic driver config. Two approaches:

**Option A (recommended):** Accept `--driver generic --command "my-agent" --prompt-flag "--message"` as individual flags when driver is generic.

**Option B:** Accept `--driver-config '{"command": "my-agent", ...}'` as raw JSON.

Go with Option A for common fields and fall back to editing `.fleet/config.json` for advanced patterns. This keeps the CLI simple:

```
fleet add my-agent feature/test --driver generic --command cursor-cli --prompt-flag "--message"
```

Add these flags to `addCmd`:
- `--driver` (string, default "claude-code")
- `--command` (string, for generic driver)
- `--prompt-flag` (string, for generic driver)
- `--yolo-args` (string slice, for generic driver)

### Step 5: Write Tests

Create `internal/driver/generic_test.go`:

1. **TestGenericBuildCommand_Positional** — prompt as positional arg
2. **TestGenericBuildCommand_WithFlag** — prompt via `--message` flag
3. **TestGenericBuildCommand_FromFile** — `prompt_from_file: true`
4. **TestGenericBuildCommand_YoloArgs** — verify yolo args appended
5. **TestGenericDetectState_WaitingPatterns** — user-defined waiting regex
6. **TestGenericDetectState_WorkingPatterns** — user-defined working regex
7. **TestGenericDetectState_NoPatterns** — returns nil when unconfigured
8. **TestGenericDetectState_InvalidRegex** — parseGenericConfig errors on bad regex
9. **TestGenericCheckAvailable_NoCommand** — errors when command is empty
10. **TestGenericCheckAvailable_NotFound** — errors when command not in PATH

### Step 6: Verify Integration

After implementing, verify:
1. `go test ./internal/driver/...` passes
2. `go build -o fleet ./cmd/fleet/` compiles
3. The full test suite `go test ./...` passes
4. Manual test with a simple command: `fleet add test-generic feature/test --driver generic --command bash`

## State Detection Without Hooks or Patterns

When a generic agent has no hooks and no patterns configured, the monitor will:
1. Check state file — empty (no hooks to write it)
2. Call `DetectState` — returns nil (no patterns)
3. Fall back to default — `StateWorking`

This means unconfigurated generic agents always show as "working". This is acceptable — the user can:
- Add regex patterns to improve detection
- Have their agent call `fleet signal waiting/working` manually
- Simply check on agents periodically

## Notes

- The generic driver is the most flexible but least reliable for state detection. Document this trade-off clearly.
- Users who build custom agents that integrate with `fleet signal` get the same quality state detection as Claude Code, regardless of driver choice.
- The `PromptFromFile` option exists for agents that can read prompts from files directly, avoiding the shell variable expansion step entirely.
- Regex patterns are compiled once when the driver is constructed, not on every state check. This is important for performance since state detection runs every 2 seconds per agent.
