# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build the binary
go build -o fleet ./cmd/fleet/

# Run directly without building
go run ./cmd/fleet/ <command>

# Install to PATH
go install ./cmd/fleet/

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/monitor/...
```

The binary must be placed alongside `fleet.tmux.conf` for the tmux config to be auto-sourced (it looks for the conf file relative to the executable path).

## Architecture

Fleet Commander is a CLI + TUI tool for managing parallel Claude Code sessions, each in its own git worktree. The user stays in control — there is no AI coordinator.

**Data model:** A `Fleet` owns a repo path and a list of `Agent`s. Each agent maps to one git worktree and one tmux session. State is persisted in `.fleet/config.json` inside the managed repo (not in this repo).

**Package layout:**

- `cmd/fleet/main.go` — Cobra CLI wiring. All commands (`init`, `add`, `list`, `start`, `attach`, `stop`, `queue`, `remove`, `hint`) are defined here.
- `internal/fleet/` — Core data model. `Fleet` and `Agent` structs, JSON config persistence at `.fleet/config.json`. `Load()` walks up the directory tree to find a fleet.
- `internal/tmux/` — Thin wrapper around tmux CLI commands. Sessions are named `fleet-<agentName>`. Sources `fleet.tmux.conf` after creating a session (best-effort).
- `internal/monitor/` — Reads tmux pane content via `capture-pane` and classifies agent state as `working`, `waiting`, `stopped`, or `starting`. State detection inspects only the bottom 15 lines and checks for Claude Code-specific patterns: "esc to interrupt", spinner characters, "Esc to cancel", `❯` prompt, etc.
- `internal/tui/` — Bubble Tea TUI for `fleet queue`. Lists all agents with live state indicators, refreshes every 2 seconds. Selecting an agent starts its session if stopped, then attaches. After detaching (Ctrl+B, Q), the TUI loop restarts automatically.
- `internal/queue/` — In-memory `Queue` / `Request` data structure (currently unused by the TUI; state is derived live from tmux).
- `internal/worktree/` — Git worktree creation helpers called by `fleet.AddAgent`.
- `internal/agent/` — Process management helpers (start/kill Claude Code processes). Used for non-tmux flows; the main path uses `tmux` package directly.

**Key flow — `fleet queue`:**
1. `tui.Run()` loops: show TUI → user selects agent → attach to tmux session → user detaches → reload fleet config → show TUI again.
2. The monitor polls each tmux session's pane and applies regex-like heuristics to determine if Claude is waiting for input vs. actively working.
3. Agents marked `⏳ NEEDS INPUT` are the ones the user should attend to first.

**Tmux session naming:** `fleet-<agentName>` (prefix hardcoded as `"fleet"` in `NewManager`).

**Fleet config location:** `.fleet/config.json` inside the target repo (not the fleet-commander repo). `.fleet/` is automatically added to `.gitignore`.
