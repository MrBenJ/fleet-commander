# Fleet Commander

A multi-agent orchestration system for managing parallel AI coding sessions.

## Core Concept

Fleet Commander lets you run multiple Claude Code (or other AI coding agents) instances in parallel, each working on different branches of the same or different repositories. When agents need user input, they queue up — you handle one, move to the next.

## Key Patterns from Gastown (Simplified for MVP)

| Gastown Concept | Fleet Commander Equivalent |
|----------------|---------------------------|
| Mayor | User (you are the coordinator) |
| Rig | Fleet (a repo + its worktrees) |
| Polecat | Agent (Claude Code instance) |
| Hook | Worktree (git worktree for isolation) |
| Convoy | Queue (pending user inputs) |
| Beads | Task (ticket/branch description) |

## Architecture

```
┌─────────────────────────────────────┐
│      Fleet Commander (TUI)          │
│  ┌─────────────────────────────┐    │
│  │ Queue: [A1↻] [A2] [A3]      │    │  ← Tab to switch
│  │ Active: agent-2 (bug-42)    │    │
│  └─────────────────────────────┘    │
└─────────────┬───────────────────────┘
              │
    ┌─────────┼─────────┐
    ▼         ▼         ▼
┌────────┐ ┌────────┐ ┌────────┐
│claude  │ │claude  │ │claude  │
│code    │ │code    │ │code    │
│--dir   │ │--dir   │ │--dir   │
│/wt-1   │ │/wt-2   │ │/wt-3   │
└────┬───┘ └────┬───┘ └────┬───┘
     │          │          │
┌────▼───┐ ┌────▼───┐ ┌────▼───┐
│worktree│ │worktree│ │worktree│
│/wt-1   │ │/wt-2   │ │/wt-3   │
│branch: │ │branch: │ │branch: │
│feat-123│ │bug-456 │ │refactor│
└────────┘ └────────┘ └────────┘
```

## MVP Features

1. **Fleet Management**
   - `fleet init <repo>` — Initialize a fleet for a repo
   - `fleet add <name> <branch>` — Add a new agent/workspace
   - `fleet list` — Show all agents and their status

2. **Queue System**
   - `fleet queue` — Open TUI with pending inputs
   - Auto-detect when Claude asks for user input
   - Switch between agents with hotkeys

3. **Agent Lifecycle**
   - Spawn Claude Code in worktree
   - Pause/resume agents
   - Kill and cleanup

## Directory Structure

```
~/code/fleet-commander/
├── cmd/
│   └── fleet/
│       └── main.go
├── internal/
│   ├── fleet/
│   │   └── fleet.go      # Fleet management
│   ├── agent/
│   │   └── agent.go      # Agent lifecycle
│   ├── queue/
│   │   └── queue.go      # Queue management
│   ├── tui/
│   │   └── tui.go        # Terminal UI
│   └── worktree/
│       └── worktree.go   # Git worktree ops
├── go.mod
├── go.sum
└── README.md
```

## Usage

```bash
# Initialize fleet for a repo
fleet init ~/projects/my-app

# Add agents for different tasks
fleet add feat-auth "feature/user-authentication"
fleet add bug-login "bugfix/login-redirect"
fleet add refactor-db "refactor/database-models"

# Open queue TUI
fleet queue

# Inside TUI:
#   [1] feat-auth: "Should I use JWT or sessions?"
#   [2] bug-login: "Which redirect URL?"
#   [3] refactor-db: (working...)
#
# Press 1 → drops into feat-auth's Claude session
# Answer question, exit → back to queue
# Press 2 → drops into bug-login's session
# etc.
```

## Tech Stack

- **Language:** Go (for single binary, fast startup)
- **TUI:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Git:** Native git worktrees for isolation
- **Process Management:** OS process spawning/monitoring

## Why Not Just Use Gastown?

Gastown is powerful but opinionated:
- Requires Dolt, Beads, specific workflow patterns
- Heavyweight for simple parallel task management
- Learning curve for the "Mayor" pattern

Fleet Commander is:
- Dead simple: worktrees + queue
- Minimal dependencies: just git and Claude Code
- You stay in control: no AI coordinator, you manage the fleet
