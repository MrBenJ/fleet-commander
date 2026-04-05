# Fleet Commander

A CLI + TUI tool for managing parallel Claude Code sessions, each in its own git worktree. You stay in control -- there is no AI coordinator.

## How It Works

Fleet Commander gives each Claude Code instance its own git worktree and tmux session. You switch between agents using a TUI queue that shows which agents are working and which need your input. The typical workflow is: launch agents, let them work, attend to the ones that need input, repeat.

```
┌─────────────────────────────────────┐
│      Fleet Commander (TUI)          │
│  ┌─────────────────────────────────┐│
│  │ Queue: [A1 ] [A2] [A3]         ││
│  │ Active: agent-2 (bug-42)       ││
│  └─────────────────────────────────┘│
└─────────────┬───────────────────────┘
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

# Initialize fleet for a repo
fleet init ~/projects/my-app
cd ~/projects/my-app

# Start the interactive fleet launcher
# to fire off multiple agents at once
fleet launch

# After launching your fleet of agents, you can
# view the interactive queue to see which agents
# need your input to continue working
fleet queue
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
| `fleet add <name> <branch>` | Add a new agent with its own worktree and branch |
| `fleet remove <name>` | Remove an agent, kill its session, clean up worktree |
| `fleet rename <old> <new>` | Rename an agent and move its worktree (agent must be stopped) |
| `fleet list` | Show all agents with status, branch, hooks, and PID |
| `fleet list --agent-list` | Print only agent names, one per line (useful for piping to `xargs`) |

### Session Management

| Command | Description |
|---------|-------------|
| `fleet start <name>` | Start an agent's tmux session (spawns Claude Code) |
| `fleet stop <name>` | Stop an agent's tmux session (also cleans up hooks and state files) |
| `fleet attach <name>` | Attach to an agent's tmux session |
| `fleet queue` | Open the TUI queue -- see all agents, jump to ones needing input |

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

Fleet Commander stores all its data in `.fleet/` inside the managed repo (automatically added to `.gitignore`):

```
.fleet/
├── config.json              # Fleet config (agents, branches, status)
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

## Prerequisites

- **[Go](https://go.dev/doc/install)** (1.21+) -- to build the binary
- **[git](https://git-scm.com/)** -- worktree creation and branch management
- **[tmux](https://github.com/tmux/tmux/wiki)** -- each agent runs in its own tmux session
- **[Claude Code](https://docs.anthropic.com/en/docs/claude-code)** -- the AI coding agent (`claude` must be on your `PATH`)

## Building

```bash
# Build the binary
go build -o fleet ./cmd/fleet/

# Or install to your PATH
go install ./cmd/fleet/
```

The binary should be placed alongside `fleet.tmux.conf` for the tmux config to be auto-sourced (it looks for the conf file relative to the executable path).
