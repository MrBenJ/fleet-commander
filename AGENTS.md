# AGENTS.md

Guidance for AI coding agents working in the Fleet Commander repository.

For Claude-specific conventions, see [`CLAUDE.md`](./CLAUDE.md). This file is intended to be self-contained for any agent.

---

## Build & Run

```bash
# Build the CLI binary
go build -o fleet ./cmd/fleet/

# Run without building
go run ./cmd/fleet/ <command>

# Install to PATH
go install ./cmd/fleet/

# Run all Go tests
go test ./...

# Run a specific package's tests
go test ./internal/squadron/...

# Build the React SPA only
make build-web

# Build SPA + embed into Go binary + install to PATH
make build-all
```

The binary must be placed alongside `fleet.tmux.conf` for the tmux config to be auto-sourced (it looks for the conf file relative to the executable path).

**Tech stack:** Go 1.21+ (backend), React 18 + TypeScript + Vite (frontend), tmux (session management), git worktrees (isolation).

---

## Data Model

A **Fleet** represents a managed repository. It stores:
- `RepoPath` — absolute path to the repo root
- `ShortName` — human-readable alias (defaults to directory basename)
- `Agents` — list of `Agent` structs
- Fleet metadata is persisted in `.fleet/config.json` inside the managed repo

An **Agent** represents one coding agent session:
- `Name` — unique identifier (used for tmux session name: `fleet-<Name>`)
- `Branch` — git branch the agent works on
- `WorktreePath` — path to the git worktree
- `Driver` — which agent CLI drives this session (`claude-code`, `codex`, `aider`, `kimi-code`, `generic`)
- `StateFile` — path to `.fleet/states/<Name>.json` (used by hooks)
- `Persona` — optional built-in personality key
- `FightMode` — whether the agent roasts squadron mates

**Global index:** `~/.fleet/repos.json` tracks all registered repos for multi-repo commands like `fleet queue --all`.

---

## Package Layout

| Package | Responsibility |
|---------|---------------|
| `cmd/fleet/` | Cobra CLI commands, `go:embed` for the web SPA, `main.go` entrypoint |
| `internal/fleet/` | `Fleet`/`Agent` structs, JSON config persistence, `Load()` / `Init()` |
| `internal/tmux/` | tmux session creation, attachment, killing; session names are `fleet-<agent>` |
| `internal/monitor/` | Agent state detection: hook-based primary, tmux pane-scrape fallback |
| `internal/tui/` | Bubble Tea TUI for `fleet queue` and the hangar terminal overlay |
| `internal/worktree/` | Git worktree CRUD: `Create`, `CreateFromExisting`, `Remove`, `Move`, `List`, `Exists` |
| `internal/context/` | Shared JSON store (`.fleet/context.json`) with file locking; channels, logs, agent sections |
| `internal/state/` | Atomic read/write of `.fleet/states/<agent>.json`; `IsStale(ttl)` helper |
| `internal/hooks/` | Inject/remove fleet signal hooks in `.claude/settings.json` per worktree |
| `internal/driver/` | Driver interface + implementations for each supported agent CLI |
| `internal/squadron/` | Squadron launch logic, prompt suffix assembly (consensus, merger, fight mode, personas) |
| `internal/hangar/` | Web server, REST API, WebSocket hub, terminal proxy, embedded SPA serving |
| `web/` | React SPA source. Built to `web/dist`, copied to `cmd/fleet/webdist` for embedding |

---

## Squadron & Merger Workflow

A **squadron** is a group of agents that coordinate through a fleet context channel, review each other's work, and converge onto a single merged branch.

### Launch Flow (`internal/squadron/headless.go`)

1. Parse and validate `SquadronData` JSON (name, consensus, base branch, agents, etc.)
2. Resolve base branch (explicit or current branch)
3. Select a **merge master** randomly (or from `mergeMaster` field) if `autoMerge` is enabled
4. Create the squadron channel: `fleetctx.CreateChannel(..., "squadron-<name>", ..., agentNames)`
5. For each agent:
   - Add/get agent in fleet config
   - Build the full prompt: system prompt + agent roster table + agent-specific task prompt
   - Append **consensus suffix** (`universal`, `review_master`, or none)
   - Append **merger suffix** if this agent is the merge master and `autoMerge` is on
   - Append **fight-mode suffix** if `fightMode` is true
   - Apply **persona preamble** if a persona is set
   - Write prompt to `.fleet/prompts/<name>.txt`
   - Build launcher script via driver
   - Write launcher to `.fleet/prompts/<name>.sh`
   - Kill existing tmux session
   - Create new tmux session with launcher

### Consensus Modes

- `universal` — every agent reviews every other agent. All must APPROVE.
- `review_master` — one designated agent reviews everyone else.
- `none` — no review step. Agents announce COMPLETED and the merge master proceeds.

### Merge Master Duties

The merge master receives a special prompt suffix (`internal/squadron/squadron.go` → `BuildMergerSuffix`) instructing them to:

1. **Create a dedicated integration worktree:**
   ```bash
   git worktree add -b squadron/<name>-merged ../<name>-merged <base-branch>
   ```
   This worktree serves as the integration point where all agent branches are merged.

2. Change into that worktree and merge each agent's branch sequentially with `git merge --no-ff <branch>`.

3. Resolve conflicts using each agent's original prompt as context.

4. Announce `MERGE_COMPLETE: squadron/<name>-merged` or `MERGE_FAILED: <agent> - <reason>` in the squadron channel.

5. If `autoPR` is enabled, push the branch and open a GitHub PR via `gh pr create`, then poll CI with `gh pr checks --watch`.

**Non-merger agents** get a `Pull Request Policy` block forbidding them from opening PRs for their individual branches.

### Hangar Wizard Flow

The web UI wizard (`web/src/components/wizard/`) walks users through:
1. **SetupStep** — squadron name, consensus mode, base branch, auto-merge/auto-PR toggles
2. **AgentsStep** — add agents (AI generate, manual, CSV import), squadron-wide fight mode toggle, per-agent persona picker
3. **ReviewStep** — inline edit agents, consensus override, launch

Persona selection is a modal sub-step (`PersonaStep`) launched from the agent list in `AgentsStep`.

---

## State Monitoring

Fleet Commander uses a two-layer state detection system:

### Layer 1: Hook-Based Signaling (Primary)

When an agent starts, `internal/hooks/` injects entries into the worktree's `.claude/settings.json`:

- `PreToolUse` event → runs `fleet signal working`
- `Stop` event → runs `fleet signal waiting`

These write to `.fleet/states/<agent>.json` with atomic temp-file + rename. The `FLEET_STATE_FILE` and `FLEET_AGENT_NAME` environment variables tell the `fleet signal` command where to write.

### Layer 2: Tmux Pane Scraping (Fallback)

If a state file is stale (>10 minutes), `internal/monitor/` captures the bottom 15 lines of the tmux pane and pattern-matches for Claude Code UI elements:
- **Waiting:** `❯` prompt, `Esc to cancel`, `(y/n)`, permission prompts
- **Working:** spinner characters, `esc to interrupt`

### WebSocket Broadcasting

The hangar's WebSocket hub (`internal/hangar/ws/hub.go`) polls agent states every 2 seconds and broadcasts `agent_state` events to all connected browsers, updating the live status pills in mission control.

---

## Context System

Agents coordinate through `.fleet/context.json` (per-repo) and `~/.fleet/context.json` (cross-repo global log).

### Sections

- `Shared` — a single string anyone can set
- `Agents` — map of `agentName → string` (each agent owns their own key)
- `Log` — timestamped, attributed shared log
- `Channels` — named private channels with fixed membership

### CLI Surface

```bash
fleet context read [agent-name]          # read context
fleet context write <msg>                # write to your agent section
fleet context set-shared <msg>           # set shared section
fleet context log <msg>                  # append to shared log
fleet context channel-create <name> <agents...> [--description <text>]
fleet context channel-send <channel> <msg>
fleet context channel-read <channel>
fleet context channel-list
fleet context global-log <msg>           # cross-repo log
fleet context global-read
fleet context export [--format json|text] [--log-only] [-o file]
fleet context clear [--yes] [--all] [--channel <name>] [--all-channels]
```

All write operations acquire a POSIX advisory file lock (`flock`) on `.fleet/context.lock` with a 5-second timeout.

---

## Web UI Build Pipeline

```
make build-all
  └─> make build-web
        └─> cd web && npm install && npm run build
              └─> tsc && vite build  →  web/dist/
      └─> rm -rf cmd/fleet/webdist
      └─> cp -r web/dist cmd/fleet/webdist
      └─> go clean -cache
      └─> go install -ldflags "..." ./cmd/fleet/
      └─> rm -rf cmd/fleet/webdist
      └─> mkdir -p cmd/fleet/webdist && touch cmd/fleet/webdist/.gitkeep
```

**Dev mode:** `fleet hangar --dev` starts the Go server and reverse-proxies all unmatched requests to the Vite dev server on `localhost:5173`. The Vite config in `web/vite.config.ts` proxies `/api` and `/ws` back to the Go server on `localhost:4242`. This gives you hot-reload for frontend development.

**Production:** `make build-all` embeds the compiled SPA via `//go:embed all:webdist` in `cmd/fleet/embed.go`. The hangar serves these static files directly with SPA fallback (routes without a dot rewrite to `/`).

---

## Testing Conventions

- Go tests use standard `testing` + table-driven patterns.
- Test helpers that create git repos shell out to `git init` and `git commit --allow-empty`.
- Web tests use Vitest + `@testing-library/react` + jsdom. Run with `cd web && npm run test`.
- When adding UI features, match existing Tailwind/className patterns and component structure.
- When modifying prompt templates in `internal/squadron/`, always add or update unit tests in `internal/squadron/squadron_test.go` and/or `internal/squadron/headless_test.go`.

---

## Useful References

- `README.md` — human-facing usage guide and command reference
- `CLAUDE.md` — Claude Code specific guidance (build commands, architecture overview)
- `docs/drivers/` — driver interface documentation
- `docs/superpowers/` — extended feature docs
- `fleet.tmux.conf` — tmux configuration sourced for every agent session
