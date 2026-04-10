# Aider Driver Implementation Plan

## Overview

Implement the `aider` driver for Fleet Commander, enabling [Aider](https://aider.chat) as a coding agent backend. This driver implements the `Driver` interface defined in `internal/driver/driver.go`.

**Prerequisites:** The driver interface and Claude Code driver must already exist at `internal/driver/`. Read `docs/drivers/00-driver-interface.md` for the full interface definition.

## Background

Aider is an open-source AI pair programming tool. Key characteristics:
- Invoked as `aider` from the command line
- Accepts a message via `--message` or `-m` flag: `aider --message "do something"`
- Has `--yes` flag to auto-confirm prompts (equivalent to yolo mode)
- Has `--no-auto-commits` flag (may want to let fleet control commits)
- Supports multiple LLM backends via `--model` flag
- Uses various API key env vars depending on model (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.)
- Does NOT have a hook system — state detection is pane-scraping only
- Has a distinctive REPL-style interface with `aider>` prompt
- Shows file change diffs inline

## Implementation Steps

### Step 1: Create `internal/driver/aider.go`

Implement the `Driver` interface:

```go
package driver

import "fmt"

type AiderDriver struct{}

func (d *AiderDriver) Name() string { return "aider" }
```

#### `BuildCommand`

```go
func (d *AiderDriver) BuildCommand(opts LaunchOpts) string {
    flags := "--no-auto-commits"
    if opts.YoloMode {
        flags += " --yes"
    }
    // Aider uses --message for non-interactive prompt, but stays in REPL after.
    // Use --message to send the initial task, then aider drops to its REPL
    // for follow-up interaction.
    return fmt.Sprintf(`#!/usr/bin/env bash
prompt=$(cat %q)
exec aider %s --message "$prompt"
`, opts.PromptFile, flags)
}
```

**Design decision:** Use `--no-auto-commits` because fleet manages branches and the user reviews work via PRs. Aider's auto-commit behavior would create many small commits that clutter history. If users want auto-commits, they can use the generic driver with custom flags.

**Alternative considered:** Using `--yes-always` instead of `--yes`. The `--yes` flag confirms file edits but still asks about adding new files. `--yes-always` skips all confirmations. Use `--yes` for yolo mode since it's safer and more predictable.

#### `DetectState`

Aider's terminal patterns:

**Waiting patterns (check first):**
- `aider>` — the main REPL prompt, agent is waiting for user input
- `Add .* to the chat?` — asking to add a file
- `[Y/n]` or `[y/n]` — confirmation prompts
- `Run shell command?` — asking to run a command
- `Apply edit?` or `Edit .* ?` — asking to apply changes
- Lines ending with `?` (length > 10) — general questions

**Working patterns:**
- `Tokens:` lines — token usage display during generation
- `Editing...` or `Committing...` status
- Streaming output (partial lines, rapid updates)
- `Applying edit to` — actively modifying files
- Spinner characters

```go
func (d *AiderDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
    bottomText := strings.Join(bottomLines, "\n")

    // WAITING patterns (checked first)
    for _, line := range bottomLines {
        trimmed := strings.TrimSpace(line)
        if trimmed == "aider>" || strings.HasPrefix(trimmed, "aider>") {
            state := StateWaiting
            return &state
        }
    }
    if strings.Contains(bottomText, "Add") && strings.Contains(bottomText, "to the chat?") {
        state := StateWaiting
        return &state
    }
    if strings.Contains(bottomText, "[Y/n]") || strings.Contains(bottomText, "[y/n]") {
        state := StateWaiting
        return &state
    }
    if strings.Contains(bottomText, "Run shell command?") {
        state := StateWaiting
        return &state
    }

    // WORKING patterns
    if strings.Contains(bottomText, "Tokens:") {
        state := StateWorking
        return &state
    }
    if strings.Contains(bottomText, "Editing") || strings.Contains(bottomText, "Committing") {
        state := StateWorking
        return &state
    }

    return nil // unknown — let monitor decide
}
```

**Important:** These patterns should be validated empirically by running aider in a tmux session and observing `tmux capture-pane -p` output. Aider's output format may vary between versions.

#### `InjectHooks` / `RemoveHooks`

Aider does not have a hook system. Return nil (no-op):

```go
func (d *AiderDriver) InjectHooks(worktreePath string) error { return nil }
func (d *AiderDriver) RemoveHooks(worktreePath string) error { return nil }
```

#### `CheckAvailable`

```go
func (d *AiderDriver) CheckAvailable() error {
    if _, err := exec.LookPath("aider"); err != nil {
        return fmt.Errorf("aider command not found in PATH (install: pip install aider-chat)")
    }
    return nil
}
```

### Step 2: Register the Driver

In `internal/driver/registry.go`, add:
```go
"aider": &AiderDriver{},
```

### Step 3: Write Tests

Create `internal/driver/aider_test.go`:

1. **TestAiderBuildCommand** — verify script output with and without YoloMode, confirm `--no-auto-commits` is always present
2. **TestAiderBuildCommand_PromptFileEscaping** — verify paths with spaces/special chars
3. **TestAiderDetectState_Waiting** — test `aider>` prompt, `[Y/n]`, `Add X to the chat?`, `Run shell command?`
4. **TestAiderDetectState_Working** — test `Tokens:` display, `Editing`, `Committing`
5. **TestAiderDetectState_Unknown** — returns nil for unrecognized content
6. **TestAiderCheckAvailable** — mock LookPath

### Step 4: Verify Integration

After implementing, verify:
1. `go test ./internal/driver/...` passes
2. `go build -o fleet ./cmd/fleet/` compiles
3. The full test suite `go test ./...` passes
4. If you have `aider` installed, manually test: `fleet add test-aider feature/test --driver aider && fleet start test-aider`

## Notes

- Aider operates in REPL mode after the initial `--message`. This means the agent stays running and interactive, similar to Claude Code. The user can send follow-up messages by typing in the tmux session.
- Aider's `--no-auto-commits` flag is important because fleet manages branches. Without it, aider creates many small commits that make PR review harder.
- The `aider>` prompt is the most reliable waiting indicator. It's consistently shown when aider is ready for input.
- Aider supports many models (`--model gpt-4o`, `--model claude-3.5-sonnet`, etc.). The driver doesn't need to handle model selection — users configure this in their aider config or environment.
