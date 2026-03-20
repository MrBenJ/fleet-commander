# Fleet Commander — Phase 1 & Phase 2 Hardening Design

**Date:** 2026-03-19
**Status:** Approved
**Scope:** Phase 1 (Hardening) + Phase 2 (Robustness)

---

## Background

Fleet Commander is a Go CLI + Bubble Tea TUI tool for managing parallel Claude Code sessions across git worktrees and tmux. The core loop works, but several well-defined gaps prevent safe public release:

- Critical state-detection code (`monitor.detectState`) has zero test coverage
- Hook injection failures silently degrade monitoring with no user feedback
- Dead code packages (`queue`, `agent`) exist with no integration
- Config writes have no concurrent-access protection
- Partial operation failures leave orphaned resources
- TUI surfaces errors inconsistently (silent discards in kill flow)

This spec covers the seven improvements across Phase 1 and Phase 2 that bring the project from a working personal tool to one safe to ship publicly.

---

## Phase 1 — Hardening

### P1-1: Test `monitor.detectState()`

**Problem:** `detectState(lastLine, fullContent string) AgentState` in `internal/monitor/monitor.go` contains 16 pattern-matching strings and is the single most critical function in the tool — it determines which agents need user attention. It currently has 0% test coverage.

**Affected file:** `internal/monitor/monitor_test.go`

**Design:**

Add a `TestDetectState` table-driven test function alongside the existing tests. Each case provides a `lastLine` and `fullContent` string and asserts the expected `AgentState`.

Cover all 16 patterns explicitly:

| Pattern | Expected state |
|---|---|
| Content contains `"esc to interrupt"` | `StateWorking` |
| Content contains a spinner char (`"⠋"`, `"⠙"`, etc.) | `StateWorking` |
| Content contains `"Esc to cancel"` | `StateWaiting` |
| Content contains `"Do you want to proceed"` | `StateWaiting` |
| Content contains `"accept edits"` | `StateWaiting` |
| Content contains `"shift+tab to cycle"` | `StateWaiting` |
| Last non-empty lines contain `"❯"` + `"1."` + `"2."` | `StateWaiting` |
| Last non-empty line is bare `"❯"` | `StateWaiting` |
| Last 3 lines contain `?`-ending question (len > 10) | `StateWaiting` |
| Content contains `"(y/n)"` | `StateWaiting` |
| Content contains `"[Y/n]"` | `StateWaiting` |
| Content is empty | `StateWorking` (default) |
| Content has no matching patterns | `StateWorking` (default) |

Add additional edge-case tests:
- Content with ANSI escape sequences (verifies `stripANSI` is called before matching)
- Short question line (length ≤ 10) should NOT trigger waiting
- Question pattern appears in the 4th-from-last line (should NOT trigger — only last 3 lines)
- Multiple conflicting patterns (working pattern + waiting pattern) — working should dominate since working check runs first in the function

Also add tests for the two helpers that detectState depends on:

`TestStripANSI` — verifies ESC sequences are removed without corrupting surrounding text.
`TestGetLastNonEmptyLines` — verifies correct extraction of N trailing non-empty lines from multi-line content.

**No new files.** All tests go into `internal/monitor/monitor_test.go`. The helper functions `stripANSI` and `getLastNonEmptyLines` are unexported; tests access them directly (same package).

---

### P1-2: Fix Hook Injection Failure Visibility

**Problem:** When `hooks.Inject()` fails in `fleet start` (line 129, `cmd/fleet/main.go`), a warning is printed to stdout but `stateFilePath` is silently set to `""`. The agent starts without hook-based state reporting. From the TUI, the agent appears permanently in `StateWorking` with no indication anything is wrong.

Same failure mode exists in `tui.go → startAgentSession()` (line 222): hooks failure degrades silently.

**Design:**

Two-part fix:

**Part A — Persist hook status in Agent struct (`internal/fleet/fleet.go`):**

Add a `HooksOK bool` field to the `Agent` struct:

```go
type Agent struct {
    Name          string `json:"name"`
    Branch        string `json:"branch"`
    WorktreePath  string `json:"worktree_path"`
    Status        string `json:"status"`
    PID           int    `json:"pid"`
    StateFilePath string `json:"state_file_path,omitempty"`
    HooksOK       bool   `json:"hooks_ok"`
}
```

Add `UpdateAgentHooks(name string, hooksOK bool) error` to `Fleet` (parallel to existing `UpdateAgentStateFile`).

Call `UpdateAgentHooks` after each hooks inject/remove operation in `cmd/fleet/main.go` (startCmd, stopCmd) and `internal/tui/tui.go` (startAgentSession, kill handler).

**Part B — Surface hook status in TUI and `fleet list`:**

In `AgentDelegate.Render()` (`internal/tui/tui.go`): if `agent.HooksOK == false` and agent is not stopped, append `⚠ hooks` to the status line in dimmed red. This tells users their monitoring is degraded at a glance.

In `listCmd` (`cmd/fleet/main.go`): add a `HOOKS` column to the table that shows `✓` or `✗`.

The `fleet start` warning message already prints to stdout — keep it. These additions make the failure _persistent_ rather than one-time.

---

### P1-3: Remove Dead Code

**Problem:** Two packages exist with real implementations but zero callers. They add maintenance surface and confuse contributors about the intended architecture.

**Dead packages:**
- `internal/queue/` — 92-line `Queue` / `Request` struct, never used by TUI or any command
- `internal/agent/` — 88-line `Runner` struct with `Start`, `StartDetached`, `IsRunning`, `Kill`, never called (main path uses `tmux.Manager` directly)

**Partial stub:**
- `worktree.List()` (`internal/worktree/worktree.go`, lines 90–107) — runs `git worktree list --porcelain` but discards output (`_ = lines`), always returns empty slice

**Design:**

Delete `internal/queue/queue.go` and `internal/agent/agent.go` entirely. Update `go test ./...` should still pass (neither has test files).

For `worktree.List()`: implement proper parsing of the `--porcelain` output format. Each worktree block starts with `worktree <path>`. Extract the path lines and return them. The function signature stays the same. This makes the function correct and usable, rather than either leaving a lie in the codebase or removing a natural utility.

Porcelain format to parse:
```
worktree /path/to/main
HEAD <sha>
branch refs/heads/main

worktree /path/to/feature
HEAD <sha>
branch refs/heads/feature
```

Extract all `worktree <path>` lines, return as `[]string`.

---

### P1-4: File Lock on Fleet Config

**Problem:** `fleet.save()` and `fleet.Load()` perform read-modify-write against `.fleet/config.json` with no synchronization. Running `fleet add` and `fleet queue` simultaneously can corrupt the JSON file. This is unlikely in solo use but likely when running commands from shell scripts or CI.

**Design:**

Use `syscall.Flock` (available in Go stdlib on macOS/Linux) with a dedicated lock file `.fleet/config.lock`.

Add two unexported methods to `Fleet`:

```go
func (f *Fleet) lockConfig() (*os.File, error)
func (f *Fleet) unlockConfig(lf *os.File)
```

`lockConfig` opens `.fleet/config.lock` for writing (creating it if absent), calls `syscall.Flock(fd, syscall.LOCK_EX)` (blocking exclusive lock), and returns the open file handle.

`unlockConfig` calls `syscall.Flock(fd, syscall.LOCK_UN)` and closes the handle.

Wrap all `save()` calls and the body of `loadFromPath()` with lock/unlock:

```go
func (f *Fleet) save() error {
    lf, err := f.lockConfig()
    if err != nil {
        return fmt.Errorf("failed to acquire config lock: %w", err)
    }
    defer f.unlockConfig(lf)
    // ... existing marshal + write logic
}
```

```go
func loadFromPath(path string) (*Fleet, error) {
    lockPath := filepath.Join(filepath.Dir(path), "config.lock")
    lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
    if err != nil {
        return nil, fmt.Errorf("failed to open lock file: %w", err)
    }
    defer func() {
        syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
        lf.Close()
    }()
    if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
        return nil, fmt.Errorf("failed to acquire config lock: %w", err)
    }
    // ... existing read + unmarshal logic
}
```

Add `.fleet/config.lock` to `.gitignore` entries created by `fleet init`.

**No new dependencies.** `syscall` is stdlib.

---

## Phase 2 — Robustness

### P2-1: CLI Integration Tests

**Problem:** `fleet init`, `fleet add`, `fleet list`, `fleet start`, `fleet stop` have zero test coverage. Regressions in these commands (the critical user-facing path) are invisible until a user reports them.

**Design:**

Create `cmd/fleet/integration_test.go` with a build tag `//go:build integration` so they don't run in `go test ./...` by default (they require building the binary and a real git repo). Run with `go test -tags integration ./cmd/fleet/`.

Test helper `setupTestRepo(t) string`:
1. Creates a temp dir via `t.TempDir()`
2. Runs `git init` and `git commit --allow-empty -m "init"` in it
3. Builds the `fleet` binary to a temp path via `go build -o <tmpbinary> ./cmd/fleet/`
4. Returns the temp repo path and binary path

Tests to write:

`TestInitCreatesConfig` — runs `fleet init <tempRepo>`, asserts `.fleet/config.json` exists and is valid JSON.

`TestAddCreatesWorktreeAndConfig` — runs `fleet init` then `fleet add myagent feature/my-agent`, asserts worktree directory exists at `.fleet/worktrees/myagent` and `config.json` contains the agent.

`TestListShowsAgents` — runs init + add, then `fleet list`, asserts stdout contains agent name and branch.

`TestStopCleansUp` — this test mocks tmux (or skips if tmux unavailable via `t.Skip`), runs init + add + start + stop, asserts state file is removed and hooks are removed from `.claude/settings.json`.

Each test runs `exec.Command(binaryPath, ...)` with the temp repo as working directory. Exit codes and stdout/stderr are asserted.

---

### P2-2: Rollback on Partial `fleet add` Failure

**Problem:** `fleet.AddAgent()` in `internal/fleet/fleet.go` (lines 132–164) creates a git worktree, appends the agent to `f.Agents`, then calls `f.save()`. If `f.save()` fails after the worktree is created, the worktree directory exists on disk but the agent is missing from config. The worktree is orphaned with no way to recover it via fleet commands.

**Design:**

In `AddAgent`, after `worktree.Create()` succeeds, wrap the remaining work in a cleanup guard:

```go
wt := worktree.NewManager(f.RepoPath)
if err := wt.Create(worktreePath, branch); err != nil {
    return nil, fmt.Errorf("failed to create worktree: %w", err)
}

// Guard: if anything below fails, remove the worktree we just created
var saveErr error
defer func() {
    if saveErr != nil {
        _ = wt.Remove(worktreePath) // best-effort cleanup
    }
}()

agent := &Agent{...}
f.Agents = append(f.Agents, agent)
saveErr = f.save()
if saveErr != nil {
    f.Agents = f.Agents[:len(f.Agents)-1] // restore slice
    return nil, fmt.Errorf("failed to save fleet config: %w", saveErr)
}
return agent, nil
```

The deferred `wt.Remove` is best-effort. If it also fails, we log the orphan path but still return the original save error. This at minimum prevents silent orphaning.

No rollback is needed for `fleet init` (directory creation failures are already returned as errors and no partial state is left).

---

### P2-3: Better TUI Error Display

**Problem:** The kill handler in `tui.go` (`Update()`, lines 308–328) calls `os.Remove(agent.StateFilePath)` and `hooks.Remove(agent.WorktreePath)` but discards both errors (`_ = err`). Users have no indication that cleanup failed. The add-agent flow has an `addError` field but the kill/start flows have nothing equivalent.

**Design:**

Add a `statusMsg` field and a `statusMsgTimer` field to `Model`:

```go
type Model struct {
    // ... existing fields ...
    statusMsg      string
    statusMsgTimer time.Time
}
```

Add a helper `setStatus(msg string)` that sets `statusMsg` and `statusMsgTimer = time.Now()`.

In `View()`, below the help text line, render `statusMsg` in dimmed red if it is non-empty and `time.Since(statusMsgTimer) < 5*time.Second`. After 5 seconds, the next refresh tick will clear it (the refresh tick already fires every 2 seconds via `tea.Tick`).

Apply `setStatus` to:
- Kill handler: if `os.Remove` fails → `"⚠ could not remove state file: <err>"`
- Kill handler: if `hooks.Remove` fails → `"⚠ could not remove hooks: <err>"`
- `startAgentSession`: if hooks inject fails → `"⚠ hooks injection failed — monitoring degraded"`
- Add-agent flow: promote `addError` to use the same `setStatus` mechanism (removes the need for a separate `addError` field)

This gives users immediate, temporary feedback for non-fatal cleanup errors without blocking their workflow.

---

## Cross-Cutting Concerns

### No new external dependencies

All seven items use only stdlib (`syscall`, `os/exec`, `testing`) or existing dependencies. The `syscall` package is used for `Flock`; it is stdlib and already transitively present on macOS/Linux.

### Build tags for integration tests

Integration tests use `//go:build integration` to avoid requiring a real git repo and built binary in the standard `go test ./...` run. The CI step (when added) should run `go test -tags integration ./cmd/fleet/`.

### `HooksOK` JSON field migration

Adding `HooksOK bool` to `Agent` is backwards-compatible: existing `.fleet/config.json` files without the field will unmarshal with `HooksOK = false` (Go zero value). On next `fleet start`, the field will be set correctly.

### Testing `detectState` without exporting it

`detectState` is unexported. The new tests in `monitor_test.go` are in `package monitor` (not `package monitor_test`), which is already the case for the existing monitor tests. No change to package declaration needed.

---

## Implementation Order

The items within each phase are independent unless noted. Recommended order to minimize risk:

1. **P1-1** (detectState tests) — no code changes, pure additive, builds confidence
2. **P1-3** (dead code removal) — simplifies codebase before other changes touch it
3. **P1-4** (config locking) — foundational, must land before integration tests
4. **P1-2** (hook failure visibility) — requires P1-4 (touches save path)
5. **P2-2** (AddAgent rollback) — requires P1-4 (touches save path)
6. **P2-3** (TUI status messages) — independent, UI-only
7. **P2-1** (integration tests) — last, validates everything else

---

## Files Changed Per Item

| Item | Files Modified | Files Created | Files Deleted |
|---|---|---|---|
| P1-1 | `internal/monitor/monitor_test.go` | — | — |
| P1-2 | `internal/fleet/fleet.go`, `internal/tui/tui.go`, `cmd/fleet/main.go` | — | — |
| P1-3 | `internal/worktree/worktree.go` | — | `internal/queue/queue.go`, `internal/agent/agent.go` |
| P1-4 | `internal/fleet/fleet.go` | — | — |
| P2-1 | — | `cmd/fleet/integration_test.go` | — |
| P2-2 | `internal/fleet/fleet.go` | — | — |
| P2-3 | `internal/tui/tui.go` | — | — |

---

## Success Criteria

- `go test ./...` passes with no failures
- `go test -tags integration ./cmd/fleet/` passes against a real git repo (when tmux is available)
- `monitor` package coverage ≥ 75% (up from 10.9%)
- No silent error discards remain in TUI kill handler
- `fleet list` shows hook status per agent
- `fleet add` leaves no orphaned worktrees on config save failure
- Concurrent `fleet add` calls do not corrupt `.fleet/config.json`
- Dead code packages (`queue`, `agent`) are deleted
- `worktree.List()` returns correct paths
