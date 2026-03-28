# Fleet Commander

A CLI + TUI tool for managing parallel Claude Code sessions, each in its own git worktree. You stay in control -- there is no AI coordinator.

## How It Works

Fleet Commander gives each Claude Code instance its own git worktree and tmux session. You switch between agents using a TUI queue that shows which agents are working and which need your input. The typical workflow is: launch agents, let them work, attend to the ones that need input, repeat.

```
┌─────────────────────────────────────┐
│      Fleet Commander (TUI)          │
│  ┌─────────────────────────────┐    │
│  │ Queue: [A1 ] [A2] [A3]     │    │
│  │ Active: agent-2 (bug-42)   │    │
│  └─────────────────────────────┘    │
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
| `fleet init <repo>` | Initialize a fleet for a repository |
| `fleet add <name> <branch>` | Add a new agent with its own worktree and branch |
| `fleet remove <name>` | Remove an agent, kill its session, clean up worktree |
| `fleet rename <old> <new>` | Rename an agent and move its worktree |
| `fleet list` | Show all agents with status, branch, hooks, and PID |

### Session Management

| Command | Description |
|---------|-------------|
| `fleet start <name>` | Start an agent's tmux session (spawns Claude Code) |
| `fleet stop <name>` | Stop an agent's tmux session |
| `fleet attach <name>` | Attach to an agent's tmux session |
| `fleet queue` | Open the TUI queue -- see all agents, jump to ones needing input |

### Batch Launch

| Command | Description |
|---------|-------------|
| `fleet launch` | Enter tasks in a TUI, Claude generates agent names and branches, review and launch all at once |

### Shared Context

Agents work in isolated worktrees but can coordinate through a shared context store at `.fleet/context.json`. Each agent owns its own section and can read all others.

| Command | Description |
|---------|-------------|
| `fleet context read` | Read all shared context (shared section + all agent sections) |
| `fleet context read <name>` | Read a specific agent's context section |
| `fleet context read --shared` | Read only the shared section |
| `fleet context write <msg>` | Write to your agent's section (must be inside an agent session) |
| `fleet context set-shared <msg>` | Set the shared section (must be outside agent sessions) |

Agents can tag each other in their sections (e.g. `@api-agent merge fleet/auth`) to coordinate asynchronously. Writes are protected by file locking so concurrent agents don't step on each other.

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

Select an agent to attach to its tmux session. When you're done, detach with `Ctrl+B, Q` and you're back in the queue.

## Tmux Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+B, Q` | Detach and return to queue (agent keeps running) |
| `Ctrl+B, D` | Detach (standard tmux) |
| `Ctrl+B, L` | List all fleet sessions |

## State Detection

Fleet Commander injects hooks into each agent's Claude Code session. These hooks signal state changes (`working` / `waiting`) to `.fleet/states/<name>.json`. The monitor reads these state files to determine agent status. If a state file is stale (>10 minutes), it falls back to scraping the tmux pane for Claude Code-specific patterns.

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

## Fleet Directory Structure

Fleet Commander stores all its data in `.fleet/` inside the managed repo (automatically added to `.gitignore`):

```
.fleet/
├── config.json      # Fleet config (agents, branches, status)
├── config.lock      # Exclusive flock for concurrent access
├── context.json     # Shared context store
├── context.lock     # Exclusive flock for context writes
├── states/          # Agent state files (working/waiting)
│   └── <name>.json
└── worktrees/       # Git worktrees (one per agent)
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
