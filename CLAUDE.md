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

# Build web UI + Go binary with embedded SPA (installs to PATH)
make build-all

# Build web UI only
make build-web
```

The binary must be placed alongside `fleet.tmux.conf` for the tmux config to be auto-sourced (it looks for the conf file relative to the executable path).

## Architecture

Fleet Commander is a CLI + TUI tool for managing parallel Claude Code sessions, each in its own git worktree. The user stays in control — there is no AI coordinator.

**Data model:** A `Fleet` owns a repo path and a list of `Agent`s. Each agent maps to one git worktree and one tmux session. State is persisted in `.fleet/config.json` inside the managed repo (not in this repo).

**Package layout:**

- `cmd/fleet/main.go` — Cobra CLI wiring. All commands (`init`, `add`, `list`, `start`, `attach`, `stop`, `queue`, `remove`, `hint`, `hangar`) are defined here.
- `cmd/fleet/embed.go` — `go:embed` directive for the compiled React SPA (`cmd/fleet/webdist/`).
- `internal/fleet/` — Core data model. `Fleet` and `Agent` structs, JSON config persistence at `.fleet/config.json`. `Load()` walks up the directory tree to find a fleet.
- `internal/tmux/` — Thin wrapper around tmux CLI commands. Sessions are named `fleet-<agentName>`. Sources `fleet.tmux.conf` after creating a session (best-effort).
- `internal/monitor/` — Reads tmux pane content via `capture-pane` and classifies agent state as `working`, `waiting`, `stopped`, or `starting`. State detection inspects only the bottom 15 lines and checks for Claude Code-specific patterns: "esc to interrupt", spinner characters, "Esc to cancel", `❯` prompt, etc.
- `internal/tui/` — Bubble Tea TUI for `fleet queue`. Lists all agents with live state indicators, refreshes every 2 seconds. Selecting an agent starts its session if stopped, then attaches. After detaching (Ctrl+B, Q), the TUI loop restarts automatically.
- `internal/worktree/` — Git worktree creation helpers called by `fleet.AddAgent`.
- `internal/context/` — Shared context store (`.fleet/context.json`). Agents can publish and read context from each other. Uses flock for concurrency safety.
- `internal/state/` — State file read/write for agent state signaling between Claude Code hooks and the monitor.
- `internal/hooks/` — Claude Code hook injection/removal. Injects fleet signal commands into `.claude/settings.json` in each worktree.
- `internal/hangar/` — Web-based squadron mission control (`fleet hangar`). Go HTTP server + embedded React SPA.
  - `internal/hangar/server.go` — HTTP server, route registration, SPA serving (embedded or Vite dev proxy).
  - `internal/hangar/tui.go` — Bubble Tea TUI shown in the terminal while the server runs (zen/log views).
  - `internal/hangar/api/` — REST handlers for fleet info, personas, drivers, squadron launch, agent generation, and stop.
  - `internal/hangar/ws/` — WebSocket hub that polls `.fleet/context.json` every 2s and broadcasts new channel messages.
  - `internal/hangar/terminal/` — WebSocket-to-PTY proxy for `tmux attach-session`, powers the browser terminal.
- `web/` — React 18 + TypeScript + Vite SPA for the hangar UI. Built with `make build-web`, embedded into the Go binary via `make build-all`.
  - `web/src/components/wizard/` — Squadron setup wizard (SetupStep → AgentsStep → PersonaStep → ReviewStep).
  - `web/src/components/mission/` — Mission control view (MissionControl, ContextLog, AgentPill, AgentTooltip).
  - `web/src/components/terminal/` — xterm.js terminal page for "Assume Control".
  - `web/src/hooks/` — `useWebSocket` (auto-reconnect JSON event stream), `useFleet` (fleet data fetching).
  - `web/src/api.ts` — Typed API client for all REST endpoints.

**Key flow — `fleet queue`:**
1. `tui.Run()` loops: show TUI → user selects agent → attach to tmux session → user detaches → reload fleet config → show TUI again.
2. The monitor polls each tmux session's pane and applies regex-like heuristics to determine if Claude is waiting for input vs. actively working.
3. Agents marked `⏳ NEEDS INPUT` are the ones the user should attend to first.

**Squadron mode — `fleet launch squadron`:**
1. Interactive: consensus selector → squadron name → standard launch flow. Always runs yolo + per-agent auto-merge OFF.
2. Headless: `fleet launch squadron --data '<json>'` parses a SquadronData payload and skips the TUI entirely.
3. A fleet context channel `squadron-<name>` is auto-created with all agents as members.
4. Each agent's prompt is assembled with a consensus suffix (+ merger suffix for the designated merger + persona preamble). See `internal/squadron/`.

**Hangar — `fleet hangar`:**
1. Starts a Go HTTP server (default port 4242) and opens the browser.
2. The React SPA is embedded in the binary via `go:embed` (`cmd/fleet/webdist/`). In dev mode (`--dev`), requests proxy to Vite's dev server for hot reload.
3. Wizard flow: Setup (name, consensus, base branch) → Agents (AI generate or manual add, persona selection) → Review (inline edit, launch).
4. Launch calls `squadron.RunHeadless()` directly (no subprocess). Creates worktrees, tmux sessions, context channels, and prompt files.
5. Mission control: WebSocket hub polls `.fleet/context.json` every 2s, broadcasts new channel messages. Agent pills show live status. "Assume Control" opens an xterm.js terminal connected to the agent's tmux session via PTY proxy.
6. Build pipeline: `make build-all` runs `npm run build` in `web/`, copies `web/dist` to `cmd/fleet/webdist/`, runs `go install`, then cleans up `webdist/`.

**Tmux session naming:** `fleet-<agentName>` (prefix hardcoded as `"fleet"` in `NewManager`).

**Fleet config location:** `.fleet/config.json` inside the target repo (not the fleet-commander repo). `.fleet/` is automatically added to `.gitignore`.
