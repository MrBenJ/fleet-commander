# Codex CLI Driver Implementation Plan

## Overview

Implement the `codex` driver for Fleet Commander, enabling OpenAI's Codex CLI as a coding agent backend. This driver implements the `Driver` interface defined in `internal/driver/driver.go`.

**Prerequisites:** The driver interface and Claude Code driver must already exist at `internal/driver/`. Read `docs/drivers/00-driver-interface.md` for the full interface definition.

## Background

Codex CLI (`codex`) is OpenAI's terminal-based coding agent. Key characteristics:
- Invoked as `codex` from the command line
- Accepts prompts as positional arguments: `codex "do something"`
- Has an `--approval-mode` flag: `suggest` (default, asks before changes), `auto-edit` (auto-applies file edits), `full-auto` (no confirmations)
- Uses `OPENAI_API_KEY` environment variable
- Does NOT have a hook system like Claude Code — state detection is pane-scraping only
- Terminal output patterns differ from Claude Code

## Implementation Steps

### Step 1: Create `internal/driver/codex.go`

Implement the `Driver` interface:

```go
package driver

import "fmt"

type CodexDriver struct{}

func (d *CodexDriver) Name() string { return "codex" }
```

#### `BuildCommand`

```go
func (d *CodexDriver) BuildCommand(opts LaunchOpts) string {
    approvalMode := "suggest"
    if opts.YoloMode {
        approvalMode = "full-auto"
    }
    // Read prompt from file to avoid shell metacharacter issues
    return fmt.Sprintf(`#!/usr/bin/env bash
prompt=$(cat %q)
exec codex --approval-mode %s "$prompt"
`, opts.PromptFile, approvalMode)
}
```

#### `DetectState`

Codex CLI terminal patterns to detect:

**Waiting patterns (check first):**
- `[Y/n]` or `[y/N]` — confirmation prompts
- `Accept?` — file change acceptance
- Lines ending with `?` (length > 10) — questions to the user
- `>` prompt on its own line — input prompt
- `sandbox$` or similar shell-like prompts when codex spawns a sandbox

**Working patterns:**
- Spinner characters (various unicode spinners)
- `Running...` or `Executing...` text
- `Reading` / `Writing` / `Searching` status lines
- Active streaming output (lines appearing rapidly)

```go
func (d *CodexDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
    // Implement pattern matching against bottomLines
    // Return nil for unknown (will fall back to state file or default)
}
```

**Important:** Codex's terminal patterns will need to be discovered empirically. Start with the patterns above, then refine by running `codex` in a tmux session and observing output via `tmux capture-pane`. Add a note in the code that patterns may need tuning.

#### `InjectHooks` / `RemoveHooks`

Codex CLI does not have a hook system. Return nil (no-op):

```go
func (d *CodexDriver) InjectHooks(worktreePath string) error { return nil }
func (d *CodexDriver) RemoveHooks(worktreePath string) error { return nil }
```

**Implication:** State detection for Codex relies entirely on pane scraping. The `HooksOK` field on the agent will be `false`, and there will be no state file. The monitor must handle this gracefully — when there's no state file AND the driver's `DetectState` returns nil, default to `StateWorking`.

#### `CheckAvailable`

```go
func (d *CodexDriver) CheckAvailable() error {
    if _, err := exec.LookPath("codex"); err != nil {
        return fmt.Errorf("codex command not found in PATH (install: npm i -g @openai/codex)")
    }
    return nil
}
```

### Step 2: Register the Driver

In `internal/driver/registry.go`, add:
```go
"codex": &CodexDriver{},
```

### Step 3: Write Tests

Create `internal/driver/codex_test.go`:

1. **TestCodexBuildCommand** — verify script output with and without YoloMode
2. **TestCodexBuildCommand_PromptFileEscaping** — verify paths with spaces/special chars are properly quoted
3. **TestCodexDetectState_Waiting** — test each waiting pattern
4. **TestCodexDetectState_Working** — test each working pattern
5. **TestCodexDetectState_Unknown** — returns nil for unrecognized content
6. **TestCodexCheckAvailable** — mock LookPath to test both found and not-found cases

### Step 4: Update Meta-Prompt in `internal/tui/claude.go`

The `buildMetaPrompt` function references "Claude Code" explicitly. This was already updated by the Claude Code driver work to be agent-agnostic. No changes needed here — the meta-prompt is only used when claude is the planning agent, and the prompts it generates are agent-neutral task descriptions.

### Step 5: Verify Integration

After implementing, verify:
1. `go test ./internal/driver/...` passes
2. `go build -o fleet ./cmd/fleet/` compiles
3. The full test suite `go test ./...` passes
4. If you have `codex` installed, manually test: `fleet add test-codex feature/test --driver codex && fleet start test-codex`

## Environment Variable Forwarding

Codex requires `OPENAI_API_KEY`. The tmux session inherits the parent environment, so this should work automatically. No special handling needed unless the user's shell doesn't export it. Document this in the error message if codex fails to start.

## Notes

- Codex doesn't support hooks, so pane scraping is the only state detection mechanism. This is less reliable than hooks but functional.
- The `--approval-mode full-auto` flag is the Codex equivalent of Claude Code's `--dangerously-skip-permissions`. Map it in yolo mode.
- Codex may have different prompt format preferences than Claude Code. The prompt is passed as-is — the system prompt and agent roster are still prepended by fleet.
