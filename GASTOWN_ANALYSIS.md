# Gastown Architecture Analysis

## Location
`~/code/forks/gastown/`

## Core Concepts Mapping

| Gastown | Description | Fleet Commander Equivalent |
|---------|-------------|---------------------------|
| **Town** | Workspace root (`~/gt/`) | Fleet directory (`.fleet/`) |
| **Rig** | Project/repo container | Fleet (one per repo) |
| **Crew** | User workspace within rig | (Not needed - user manages directly) |
| **Polecat** | Worker agent | Agent |
| **Hook** | Git worktree for persistence | Worktree |
| **Convoy** | Work tracking with beads | Queue |
| **Mayor** | AI coordinator | User (you) |
| **Beads** | Structured issue tracking | (Not implemented) |

## Key Implementation Patterns

### 1. Tmux Session Management (`internal/tmux/tmux.go`)

**Key features:**
- Socket-based isolation (`-L` flag) for multi-instance support
- Session name validation (alphanumeric + underscore/hyphen only)
- Nudge system for sending commands to sessions
- Health checking via process detection
- Lock mechanism to prevent concurrent nudges

**Session naming:** `<socket>-<polecat>` (e.g., `gastown-pc-001`)

**Important details:**
- Uses `/tmp` for sockets on macOS (not `$TMPDIR`)
- Validates command binaries exist before creating sessions
- Supports both `switch-client` (same socket) and `attach-session` (different socket)

### 2. Worktree Management (`internal/cmd/worktree.go`)

**Pattern:**
```
~/gt/<rig>/crew/<user>/          # User's main workspace
~/gt/<rig>/hooks/<polecat>/       # Agent worktrees
~/gt/<rig>/polecats/<name>/       # Agent clones (if needed)
```

**Key insight:** Gastown uses worktrees for cross-rig work too — a crew member from rig A can create a worktree in rig B while keeping their identity.

### 3. Convoy System (`internal/cmd/convoy.go`)

**Convoy = Collection of work items (beads/issues)**

States:
- `open` — Active, accepting work
- `closed` — Completed or cancelled
- `staged_ready` — Ready to launch
- `staged_warnings` — Has warnings but can launch

**Key commands:**
- `gt convoy create <name> [issues...]` — Create convoy
- `gt sling <issue> <rig>` — Assign work to agent
- `gt convoy list` — Show all convoys

### 4. Session Manager (`internal/polecat/session_manager.go`)

**Session lifecycle:**
1. Create tmux session (detached)
2. Set environment variables (GT_ROLE, GT_POLECAT, etc.)
3. Send initial commands (cd to workdir, run agent)
4. Poll for readiness (check if agent process exists)
5. Handle cleanup on exit

**Environment injection:**
- Uses tmux `set-environment` to set vars
- Uses `send-keys` to send commands to session
- Can override agent per-session (GT_AGENT)

### 5. Mail Queue (`internal/cmd/mail_queue.go`)

**How agents communicate back:**
- Agents write to "mailboxes" (files in `.mail/`)
- Mayor polls mailboxes for updates
- Enables async communication between agents

## Design Decisions to Consider

### What Gastown does that we might want:

1. **Socket isolation** — Multiple towns can coexist
2. **Health checking** — Detect stuck/dead agents
3. **Mail system** — Async agent→user communication
4. **Beads integration** — Structured work tracking
5. **Formulas** — Reusable workflows

### What we're simplifying:

1. **No Mayor AI** — User is the coordinator
2. **No Beads/Dolt** — JSON files for MVP
3. **No crew identity** — Direct user→agent
4. **No formulas** — Manual workflow for now

## Code Patterns to Borrow

### Tmux command wrapper:
```go
func (t *Tmux) Command(args ...string) *exec.Cmd {
    cmd := exec.Command("tmux", args...)
    if t.socket != "" {
        cmd.Args = append([]string{"tmux", "-L", t.socket}, args...)
    }
    return cmd
}
```

### Session existence check:
```go
func (t *Tmux) HasSession(name string) bool {
    err := t.Command("has-session", "-t", name).Run()
    return err == nil
}
```

### Safe session names:
```go
var validSessionNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validateSessionName(name string) error {
    if !validSessionNameRe.MatchString(name) {
        return fmt.Errorf("invalid session name: %s", name)
    }
    return nil
}
```

## Next Steps for Fleet Commander

1. **Add socket isolation** — Support multiple fleets
2. **Add health checking** — Detect if agents are stuck
3. **Add mail/queue system** — Track pending user inputs
4. **Consider beads-lite** — Simple JSON-based work tracking
