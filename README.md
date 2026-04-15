# Fleet Commander

A CLI + TUI tool for managing parallel coding-agent sessions across multiple repositories, each agent in its own git worktree. You stay in control -- there is no AI coordinator.

Fleet Commander is agent-agnostic: Claude Code is the default, but Codex CLI, Aider, and arbitrary terminal-based agents are supported via the driver system.

## Prerequisites

- **[Go](https://go.dev/doc/install)** (1.21+) -- to build the binary
- **[Node.js](https://nodejs.org/)** (18+) -- required for building the web UI (`make build-all`)
- **[git](https://git-scm.com/)** -- worktree creation and branch management
- **[tmux](https://github.com/tmux/tmux/wiki)** -- each agent runs in its own tmux session
- **[Claude Code](https://docs.anthropic.com/en/docs/claude-code)** -- the AI coding agent (`claude` must be on your `PATH`)

## How It Works

Fleet Commander gives each agent its own git worktree and tmux session. You switch between agents using a TUI queue that shows which agents are working and which need your input. With multi-repo support, you can manage fleets across different repositories from a single interface.

```
┌─────────────────────────────────────────────┐
│         Fleet Commander (TUI)               │
│  ┌─────────────────────────────────────────┐│
│  │ my-app (3 agents)                       ││
│  │   [A1 feat-auth] [A2 bug-login] [A3]   ││
│  │ api-service (2 agents)                  ││
│  │   [A4 endpoints] [A5 caching]           ││
│  └─────────────────────────────────────────┘│
└─────────────┬───────────────────────────────┘
              │
    ┌─────────┼─────────┐
    ▼         ▼         ▼
┌────────┐ ┌────────┐ ┌────────┐
│ claude │ │ claude │ │ claude │
│  code  │ │  code  │ │  code  │
│ /wt-1  │ │ /wt-2  │ │ /wt-3  │
│feat-123│ │bug-456 │ │refactor│
└────────┘ └────────┘ └────────┘
```

## Quick Start

```bash
# Build and install
go install ./cmd/fleet/

# Initialize fleet for a repo (registers it globally)
fleet init ~/projects/my-app
fleet init ~/projects/api-service --name api

# Work within a single repo
cd ~/projects/my-app

# Start the interactive fleet launcher
# to fire off multiple agents at once
fleet launch

# View the interactive queue for this repo
fleet queue

# Or view agents across ALL registered repos
fleet queue --all
```

Additionally, there are more commands used under the hood for more granular control, or to have an agentic coordinator control the fleet in lieu of yourself.

```bash
# Add agents for different tasks
fleet add feat-auth feature/user-authentication
fleet add bug-login bugfix/login-redirect
fleet add refactor-db refactor/database-models

# Start agents
fleet start feat-auth
fleet start bug-login
fleet start refactor-db

# Open the queue TUI -- see all agents, jump to the ones that need input
fleet queue
```

## Commands

### Fleet Management

| Command | Description |
|---------|-------------|
| `fleet init <repo>` | Initialize a fleet for a repository (also creates `.fleet/FLEET_SYSTEM_PROMPT.md`) |
| `fleet init <repo> --name <short>` | Initialize with a custom short name (defaults to directory basename) |
| `fleet add <name> <branch>` | Add a new agent with its own worktree and branch |
| `fleet add <name> <branch> --driver <name>` | Add an agent backed by a specific driver (`claude-code`, `codex`, `aider`, `generic`) |
| `fleet remove <name>` | Remove an agent, kill its session, clean up worktree |
| `fleet clear [--force]` | Remove every agent: kill sessions, tear down worktrees, drop from config (branches kept) |
| `fleet rename <old> <new>` | Rename an agent and move its worktree (agent must be stopped) |
| `fleet list` | Show all agents with status, branch, hooks, and PID |
| `fleet list --agent-list` | Print only agent names, one per line (useful for piping to `xargs`) |
| `fleet list --all` | List agents across all registered repos (format: `repo/agent`) |

### Multi-Repo Management

Fleet Commander maintains a global index at `~/.fleet/repos.json` that tracks all registered repositories. Each repo gets a short name for easy identification.

| Command | Description |
|---------|-------------|
| `fleet repos list` | List all registered repositories |
| `fleet repos add <path>` | Register an existing fleet repo in the global index |
| `fleet repos remove <name>` | Unregister a repo from the global index |

### Session Management

| Command | Description |
|---------|-------------|
| `fleet start <name>` | Start an agent's tmux session (spawns Claude Code) |
| `fleet stop <name>` | Stop an agent's tmux session (also cleans up hooks and state files) |
| `fleet attach <name>` | Attach to an agent's tmux session |
| `fleet queue` | Open the TUI queue -- see all agents, jump to ones needing input |
| `fleet queue --all` | Open the multi-repo TUI -- agents grouped by repository |

### Batch Launch

| Command | Description |
|---------|-------------|
| `fleet launch` | Enter tasks in a TUI, Claude generates agent names and branches, review and launch all at once |

The launch TUI walks you through a multi-step workflow:

1. **Input** -- Type your tasks (one per line or free-form)
2. **Generation** -- Claude expands your tasks into agent names, branch names, and detailed prompts
3. **Review** -- Edit each generated agent's name, branch, and prompt before confirming
4. **Launch** -- All agents fire off in parallel

Each agent receives a system prompt from `.fleet/FLEET_SYSTEM_PROMPT.md` (created on `fleet init`, editable by you) and an automatically generated roster table showing all active fleet agents, their branches, and tasks. This gives every agent awareness of what the others are working on.

### Shared Context

Agents work in isolated worktrees but can coordinate through a shared context store at `.fleet/context.json`. Each agent owns its own section and can read all others.

#### Basic Context

| Command | Description |
|---------|-------------|
| `fleet context read` | Read all shared context (shared section + all agent sections) |
| `fleet context read <name>` | Read a specific agent's context section |
| `fleet context read --shared` | Read only the shared section |
| `fleet context write <msg>` | Write to your agent's section (must be inside an agent session) |
| `fleet context set-shared <msg>` | Set the shared section (must be outside agent sessions) |

#### Shared Log

A timestamped, attributed log that all agents can append to:

| Command | Description |
|---------|-------------|
| `fleet context log <msg>` | Append a message to the shared log (must be inside an agent session) |
| `fleet context trim [--keep N]` | Trim the shared log to the last N entries (default 500) |

#### Private Channels

Named channels with fixed membership for private agent-to-agent communication:

| Command | Description |
|---------|-------------|
| `fleet context channel-create <name> <agent1> <agent2> [...]` | Create a private channel (2-member channels auto-name to `dm-<a>-<b>`) |
| `fleet context channel-send <channel> <msg>` | Send a message to a channel |
| `fleet context channel-read <channel>` | Read channel messages |
| `fleet context channel-list` | List all channels |
| `fleet context trim --channel <name> [--keep N]` | Trim a specific channel's log |

Agents can tag each other in their sections (e.g. `@api-agent merge fleet/auth`) to coordinate asynchronously. All writes are protected by file locking so concurrent agents don't step on each other.

#### Cross-Repo Communication

For coordination across repositories, Fleet Commander provides a global log at `~/.fleet/context.json`:

| Command | Description |
|---------|-------------|
| `fleet context global-log <msg>` | Append a message to the cross-repo shared log |
| `fleet context global-read` | Read all cross-repo log entries |

Global log entries are automatically attributed with the repo short name and agent name.

#### Export & Clear

| Command | Description |
|---------|-------------|
| `fleet context export` | Export context to stdout (text format) |
| `fleet context export --format json` | Export context as JSON |
| `fleet context export -o file.txt` | Export context to a file |
| `fleet context export --log-only` | Export only the shared log |
| `fleet context clear [--yes]` | Clear log entries (prompts for confirmation) |
| `fleet context clear --all` | Clear everything: shared context, agent sections, log, and channels |
| `fleet context clear --all-channels` | Clear all channel logs |
| `fleet context clear --channel <name>` | Clear a specific channel's log |

### Utilities

| Command | Description |
|---------|-------------|
| `fleet hint` | Show keyboard shortcuts and workflow tips |

## The Queue TUI

`fleet queue` is the main interface. It shows all agents with live status indicators, refreshed every 2 seconds:

- **NEEDS INPUT** -- Claude is waiting for you. Attend to this one.
- **working** -- Claude is actively working. Leave it alone.
- **starting** -- Session is spinning up.
- **stopped** -- Session is not running.

Select an agent to attach to its tmux session. You can also add new agents directly from the queue. When you're done, detach with `Ctrl+B, Q` and you're back in the queue.

### Multi-Repo Queue

`fleet queue --all` shows agents grouped by repository. Each repo appears as a header with its agents listed below. You can start, stop, and attach to agents across all your repos from one place.

## Tmux Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+B, Q` | Detach and return to queue (agent keeps running) |
| `Ctrl+B, D` | Detach (standard tmux) |
| `Ctrl+B, L` | List all fleet sessions |

## State Detection

Fleet Commander uses a two-layer approach to detect agent state:

1. **Hook-based signaling (primary)** -- Fleet injects hooks into each agent's `.claude/settings.json` that call `fleet signal working` and `fleet signal waiting` on Claude Code lifecycle events (`PreToolUse` and `Stop`). State is written to `.fleet/states/<name>.json`.

2. **Tmux pane scraping (fallback)** -- If a state file is stale (>10 minutes), the monitor falls back to capturing the bottom of the tmux pane and pattern-matching against Claude Code UI elements (permission prompts, spinner characters, input prompts, etc.).

Hooks are automatically injected when an agent starts and cleaned up when it stops.

## System Prompt

`fleet init` creates `.fleet/FLEET_SYSTEM_PROMPT.md` with sensible defaults that teach agents how to use the shared context system, identify themselves via the `FLEET_AGENT_NAME` environment variable, and coordinate with other agents. You can edit this file to customize agent behavior across your entire fleet.

## Drivers

Fleet Commander talks to coding agents through a `Driver` interface, so you're not locked into Claude Code. Pick a driver per agent with `fleet add <name> <branch> --driver <driver>`.

| Driver | Agent | Notes |
|--------|-------|-------|
| `claude-code` (default) | [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | Full support: hook-based state signaling, system prompts, YOLO mode |
| `codex` | [Codex CLI](https://github.com/openai/codex) | Pane-scrape state detection |
| `aider` | [Aider](https://aider.chat) | Pane-scrape state detection |
| `generic` | Any terminal-based agent | Supply `--command`, optional `--prompt-flag` and `--yolo-args` |

Example -- add a Codex agent and a custom agent alongside Claude Code:

```bash
fleet add feat-auth feature/auth                         # Claude Code (default)
fleet add codex-refactor refactor/api --driver codex     # Codex CLI
fleet add my-agent feature/x --driver generic \
  --command my-cli --prompt-flag --prompt                # arbitrary agent
```

Each driver implements its own state detection (so the queue TUI still shows "needs input" vs "working"), hook injection where supported, and command building. See `docs/drivers/` for interface details.

## YOLO Mode

For those who like to live dangerously -- and we mean *dangerously* dangerously -- Fleet Commander ships with YOLO mode.

```bash
fleet launch --ultra-dangerous-yolo-mode
```

This skips all review steps, passes `--dangerously-skip-permissions` to Claude, and auto-merges on completion. Every agent fires off simultaneously with zero human oversight. You are giving a bunch of overconfident YC graduates AWS administrator access and they all have clearly been railing fat lines in the Planet Fitness bathroom for the past 4 days. Use with caution.

When you hit `Ctrl+D` to submit your prompts, you'll be met with a sobering moment of reflection:

```
  ARE YOU ABSOLUTELY SURE THIS IS READY?

This will run and you cannot stop it.
Ensure you have enough usage in your account to make it through the end of this.
Please don't destroy humanity.
Please be sober.

Ctrl+D: confirm and launch / Esc: go back
```

Hit `Ctrl+D` again to confirm you are, in fact, built different. Or hit `Esc` to return to the safety of rational decision-making.

### The "Hold My Beer" Flag

If even *two* whole keypresses feels like too much friction between you and mass unsupervised code generation:

```bash
fleet launch --ultra-dangerous-yolo-mode --i-know-what-im-doing
```

This skips the confirmation entirely. No warning. No safety net. Just you, your prompts, and the unshakeable confidence of someone who has never been burned by a rogue `rm -rf`.

We are not responsible for what happens next. Godspeed.

### No Auto-Merge

If you want the YOLO speed but prefer to review the results before merging:

```bash
fleet launch --ultra-dangerous-yolo-mode --no-auto-merge
```

Agents still run unattended with `--dangerously-skip-permissions`, but they stop when done and leave their worktrees intact for you to review. All the recklessness, with a smidge of responsibility.

### Jump.sh Integration

If you use [jump.sh](https://github.com/nickarellano/jump-sh) for local dev servers:

```bash
fleet launch --use-jump-sh
```

This includes jump.sh instructions in the system prompt so agents can spin up local dev servers in their worktrees.

## Fleet Directory Structure

### Per-Repository (`.fleet/`)

Fleet Commander stores per-repo data in `.fleet/` inside the managed repo (automatically added to `.gitignore`):

```
.fleet/
├── config.json              # Fleet config (agents, branches, short name)
├── config.lock              # Exclusive flock for concurrent access
├── context.json             # Shared context store (sections, log, channels)
├── context.lock             # Exclusive flock for context writes
├── launch.log               # Debug log for the launch TUI
├── FLEET_SYSTEM_PROMPT.md   # System prompt sent to all agents (editable)
├── states/                  # Agent state files (working/waiting)
│   └── <name>.json
└── worktrees/               # Git worktrees (one per agent)
    └── <name>/
```

### Global (`~/.fleet/`)

The global directory stores the multi-repo index and cross-repo communication:

```
~/.fleet/
├── repos.json               # Global index of all registered repos
├── repos.lock               # Exclusive flock for index writes
├── context.json             # Cross-repo shared log
└── context.lock             # Exclusive flock for global context
```

## Building

```bash
# Build the binary
go build -o fleet ./cmd/fleet/

# Or install to your PATH
go install ./cmd/fleet/
```

The binary should be placed alongside `fleet.tmux.conf` for the tmux config to be auto-sourced (it looks for the conf file relative to the executable path).
