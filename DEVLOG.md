# Development Log

## 2026-03-19: MVP Skeleton Complete

### What's Working
- [x] CLI structure with cobra
- [x] `fleet init` - Initialize fleet for a repo
- [x] `fleet add` - Add agent workspaces
- [x] `fleet list` - List agents
- [x] `fleet queue` - Open TUI (basic)
- [x] Git worktree creation
- [x] Fleet config persistence (JSON)
- [x] Basic TUI with Bubble Tea

### Architecture Decisions

1. **Go over Python**: Single binary, fast startup, easy distribution
2. **Git worktrees over clones**: Lightweight, shares object database
3. **JSON config over SQLite**: Simpler for MVP, easy to inspect
4. **User as coordinator**: No "Mayor" AI — you manage the fleet directly

### File Structure
```
~/code/fleet-commander/
├── cmd/fleet/main.go          # CLI entry point
├── internal/
│   ├── fleet/fleet.go         # Fleet management
│   ├── worktree/worktree.go   # Git worktree ops
│   ├── agent/agent.go         # Claude Code process management
│   ├── queue/queue.go         # Request queue
│   └── tui/tui.go             # Terminal UI
├── go.mod, go.sum
├── fleet                      # Compiled binary
└── README.md
```

### Completed ✓

- [x] **tmux integration** — Session creation, attach, switch, kill
- [x] **TUI with tmux status** — Shows running/stopped state
- [x] **Agent lifecycle commands** — `start`, `attach`, `stop`

### Next Steps

1. **Queue System**
   - Manual queue: user hits key to add request
   - Show pending requests in TUI sidebar
   - "Next" command to jump to next pending

2. **Input Detection (Future)**
   - Parse Claude output for "?" prompts
   - Auto-add to queue when Claude asks

3. **Multi-repo Support**
   - Fleet can span multiple repos
   - Global queue across all fleets

### Open Questions

1. How do we detect when Claude needs user input?
   - Claude Code doesn't have a machine-readable API
   - Could parse stdout for prompt patterns
   - Or: manual queue (user adds to queue when needed)

2. How do we switch between sessions?
   - tmux is the standard answer
   - But requires tmux knowledge from users
   - Alternative: built-in terminal multiplexer

3. What happens when main branch moves?
   - For MVP: user handles rebase manually
   - Future: auto-detect and suggest rebase

### Testing

```bash
# Test with a real repo
cd ~/code
git clone https://github.com/example/some-repo.git test-repo
cd test-repo

~/code/fleet-commander/fleet init .
~/code/fleet-commander/fleet add feat-1 "feature/test-one"
~/code/fleet-commander/fleet add feat-2 "feature/test-two"
~/code/fleet-commander/fleet list
~/code/fleet-commander/fleet queue
```

### Build

```bash
cd ~/code/fleet-commander
go build ./cmd/fleet

# Or install to $GOBIN
go install ./cmd/fleet
```
