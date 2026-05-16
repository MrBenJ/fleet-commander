# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

For agent-neutral guidance (self-contained), see [`AGENTS.md`](./AGENTS.md). Keep both files in sync when architecture changes.

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

# Run web tests
cd web && npm run test
```

The binary must be placed alongside `fleet.tmux.conf` for the tmux config to be auto-sourced (it looks for the conf file relative to the executable path).

**Tech stack:** Go 1.21+ (backend), React 18 + TypeScript + Vite (frontend), tmux (session management), git worktrees (isolation).

## Architecture

Fleet Commander is a CLI + TUI tool for managing parallel AI coding agent sessions, each in its own git worktree. The user stays in control — there is no AI coordinator.

**Data model:**
- `Fleet` — owns `RepoPath`, `ShortName`, and a list of `Agent`s. Persisted at `.fleet/config.json` inside the managed repo.
- `Agent` — `Name` (unique, used as tmux session: `fleet-<Name>`), `Branch`, `WorktreePath`, `Driver` (one of `claude-code`, `codex`, `aider`, `kimi-code`, `generic`), `StateFile`, optional `Persona`, `FightMode` bool.
- **Global index:** `~/.fleet/repos.json` tracks all registered repos for cross-repo commands (`fleet queue --all`).

**Package layout:**

- `cmd/fleet/` — Cobra CLI wiring split across `main.go`, `cmd_agents.go`, `cmd_context.go`, `cmd_launch.go`, `cmd_repos.go`, `cmd_utils.go`. Commands: `init`, `add`, `list`, `start`, `attach`, `stop`, `queue`, `remove`, `hint`, `hangar`, `context`, `repos`, `launch`, `signal`, `unlock`, `rename`, `clear`.
- `cmd/fleet/embed.go` — `go:embed all:webdist` directive for the compiled React SPA (`cmd/fleet/webdist/`).
- `internal/fleet/` — Core data model. `Fleet` and `Agent` structs, JSON config persistence at `.fleet/config.json`. `Load()` walks up the directory tree to find a fleet.
- `internal/global/` — Global cross-repo state at `~/.fleet/` (repos index, global context/log).
- `internal/tmux/` — Thin wrapper around tmux CLI commands. Sessions are named `fleet-<agentName>`. Sources `fleet.tmux.conf` after creating a session (best-effort).
- `internal/monitor/` — Reads tmux pane content via `capture-pane` and classifies agent state as `working`, `waiting`, `stopped`, or `starting`. State detection inspects only the bottom 15 lines and checks for Claude Code-specific patterns: "esc to interrupt", spinner characters, "Esc to cancel", `❯` prompt, etc.
- `internal/tui/` — Bubble Tea TUI for `fleet queue`. Lists all agents with live state indicators, refreshes every 2 seconds. Selecting an agent starts its session if stopped, then attaches. After detaching (Ctrl+B, Q), the TUI loop restarts automatically.
- `internal/worktree/` — Git worktree CRUD: `Create`, `CreateFromExisting`, `Remove`, `List`, `Move`, `Exists`. `Manager.Create(path, branch)` runs `git worktree add -b <branch> <path>`.
- `internal/context/` — Shared context store (`.fleet/context.json`). Agents can publish and read context from each other. Uses POSIX advisory file locks (`flock`) on `.fleet/context.lock` with a 5-second timeout. Supports named private channels with fixed membership (2-member channels auto-name to `dm-<a>-<b>`).
- `internal/state/` — State file read/write for agent state signaling between Claude Code hooks and the monitor. Atomic writes via temp-file + rename. States live in `.fleet/states/<name>.json`. `IsStale(ttl)` helper for fallback-detection.
- `internal/hooks/` — Claude Code hook injection/removal. Injects `fleet signal working` (PreToolUse) and `fleet signal waiting` (Stop) into `.claude/settings.json` in each worktree. Entries are tagged with `"_fleet": true` for safe removal. Hooks read `FLEET_STATE_FILE` and `FLEET_AGENT_NAME` env vars.
- `internal/hangar/` — Web-based squadron mission control (`fleet hangar`). Go HTTP server + embedded React SPA.
  - `internal/hangar/server.go` — HTTP server, route registration, SPA serving (embedded or Vite dev proxy).
  - `internal/hangar/tui.go` — Bubble Tea TUI shown in the terminal while the server runs (zen/log views).
  - `internal/hangar/api/` — REST handlers for fleet info, personas, drivers, squadron launch, agent generation, and stop.
  - `internal/hangar/ws/` — WebSocket hub that polls `.fleet/context.json` every 2s and broadcasts new channel messages and agent state changes.
  - `internal/hangar/terminal/` — WebSocket-to-PTY proxy for `tmux attach-session`, powers the browser terminal.
- `internal/driver/` — Driver interface and implementations (`claude-code`, `codex`, `aider`, `kimi-code`, `generic`). Each driver implements state detection, hook injection (where supported), and command building.
- `internal/squadron/` — Squadron launch logic. `RunHeadless()` creates worktrees, tmux sessions, context channels, and prompt files. `BuildMergerSuffix()`, `BuildConsensusSuffix()`, `ApplyPersona()`, and fight-mode suffix assembly live here.
- `web/` — React 18 + TypeScript + Vite SPA for the hangar UI. Built with `make build-web`, embedded into the Go binary via `make build-all`.
  - `web/src/components/wizard/` — Squadron setup wizard (SetupStep → AgentsStep → ReviewStep, with PersonaStep as a per-agent modal from AgentsStep).
  - `web/src/components/mission/` — Mission control view (MissionControl, ContextLog, AgentPill, AgentTooltip, MultiView).
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
4. Each agent's prompt is assembled with a consensus suffix (+ merger suffix for the designated merger + persona preamble + fight-mode suffix if enabled). See `internal/squadron/`.
5. **Merge master worktree:** The designated merge master receives instructions to create a dedicated git worktree for integration: `git worktree add -b squadron/<name>-merged ../<name>-merged <base-branch>`. All agent branches are merged sequentially inside this worktree (`git merge --no-ff <branch>`). The resulting branch is `squadron/<name>-merged`.
6. Merger announces `MERGE_COMPLETE: squadron/<name>-merged` or `MERGE_FAILED: <agent> - <reason>` in the squadron channel.
7. If `autoPR` is enabled, the merger pushes the branch and opens a GitHub PR via `gh pr create`, then polls CI with `gh pr checks --watch`.
8. **Non-merger agents** get a `Pull Request Policy` block forbidding them from opening PRs for their individual branches.

**Consensus modes:**
- `universal` — every agent reviews every other agent; all must APPROVE.
- `review_master` — one designated agent reviews everyone else.
- `none` — no review step. Agents announce COMPLETED and the merge master proceeds.

**Hangar — `fleet hangar`:**
1. Starts a Go HTTP server (default port 4242) and opens the browser.
2. The React SPA is embedded in the binary via `go:embed` (`cmd/fleet/webdist/`). In dev mode (`--dev`), requests proxy to Vite's dev server on `localhost:5173` for hot reload. The Vite config proxies `/api` and `/ws` back to the Go server.
3. Wizard flow: Setup (name, consensus, base branch, auto-merge/auto-PR) → Agents (AI generate, manual add, CSV import, squadron-wide fight mode toggle, per-agent persona picker) → Review (inline edit, consensus override, launch).
4. Launch calls `squadron.RunHeadless()` directly (no subprocess). Creates worktrees, tmux sessions, context channels, and prompt files.
5. Mission control: WebSocket hub polls `.fleet/context.json` every 2s, broadcasts new channel messages. Agent pills show live status. "Assume Control" opens an xterm.js terminal connected to the agent's tmux session via PTY proxy.
6. Build pipeline: `make build-all` runs `npm run build` in `web/`, copies `web/dist` to `cmd/fleet/webdist/`, runs `go install` with `-ldflags`, then cleans up `webdist/` leaving only `.gitkeep`.

**Context system — `fleet context`:**
- Shared context is a single JSON file at `.fleet/context.json` with sections: `Shared`, `Agents` (map of agent-name → string), `Log` (timestamped entries), and `Channels` (private named channels).
- All writes acquire a file lock. Read operations do not lock.
- Agents can tag each other in their sections (e.g. `@api-agent merge fleet/auth`) to coordinate asynchronously.
- Cross-repo communication uses `~/.fleet/context.json` (global log).
- Subcommands: `read`, `write`, `set-shared`, `log`, `channel-create`, `channel-send`, `channel-read`, `channel-list`, `global-log`, `global-read`, `export`, `clear`, `trim`.

**State detection:**
1. **Hook-based signaling (primary)** — Fleet injects hooks into each agent's `.claude/settings.json` that call `fleet signal working` and `fleet signal waiting` on Claude Code lifecycle events (`PreToolUse` and `Stop`). State is written to `.fleet/states/<name>.json`. Hooks resolve target paths via `FLEET_STATE_FILE` and `FLEET_AGENT_NAME` env vars.
2. **Tmux pane scraping (fallback)** — If a state file is stale (>10 minutes), the monitor falls back to capturing the bottom of the tmux pane and pattern-matching against Claude Code UI elements.

**Tmux session naming:** `fleet-<agentName>` (prefix hardcoded as `"fleet"` in `NewManager`).

**Fleet config location:** `.fleet/config.json` inside the target repo (not the fleet-commander repo). `.fleet/` is automatically added to `.gitignore`.

## Testing Conventions

- Go tests use standard `testing` + table-driven patterns. Test helpers that create git repos shell out to `git init` and `git commit --allow-empty`.
- Web tests use Vitest + `@testing-library/react` + jsdom. Run with `cd web && npm run test`.
- When modifying prompt templates in `internal/squadron/`, always add or update unit tests in `internal/squadron/squadron_test.go` and/or `internal/squadron/headless_test.go`.
- For UI features, match existing Tailwind/className patterns and component structure under `web/src/components/`.

## Useful References

- `README.md` — human-facing usage guide and command reference
- `AGENTS.md` — agent-neutral guidance (self-contained mirror of this file)
- `docs/drivers/` — driver interface documentation
- `docs/superpowers/` — extended feature docs
- `fleet.tmux.conf` — tmux configuration sourced for every agent session
