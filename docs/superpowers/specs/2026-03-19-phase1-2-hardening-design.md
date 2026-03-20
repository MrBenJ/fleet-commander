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

**Problem:** `detectState(lastLine, fullContent string) AgentState` in `internal/monitor/monitor.go` contains ~18 distinct pattern-matching strings and is the single most critical function in the tool — it determines which agents need user attention. It currently has 0% test coverage.

**Affected files:**
- `internal/monitor/monitor_test.go` — existing file, `package monitor_test`, leave unchanged
- `internal/monitor/detect_test.go` — **new file**, `package monitor` (internal package access required for unexported functions)

Both files can coexist in the same directory. The new file uses `package monitor` (not `package monitor_test`) because `detectState`, `stripANSI`, and `getLastNonEmptyLines` are unexported and can only be called from within the same package.

**Design:**

Create `internal/monitor/detect_test.go` with:

**`TestDetectState`** — table-driven test. Each case provides `lastLine` and `fullContent` strings and asserts the returned `AgentState`.

Cover all patterns explicitly:

| Input | Expected state |
|---|---|
| `fullContent` contains `"esc to interrupt"` | `StateWorking` |
| `fullContent` contains spinner char `"⠋"` (or any of: `⠙⠹⠸⠼⠴⠦⠧⠇⠏`) | `StateWorking` |
| `fullContent` contains `"Esc to cancel"` | `StateWaiting` |
| `fullContent` contains `"Do you want to proceed"` | `StateWaiting` |
| `fullContent` contains `"accept edits"` | `StateWaiting` |
| `fullContent` contains `"shift+tab to cycle"` | `StateWaiting` |
| Last non-empty lines contain `"❯"` and `"1."` and `"2."` | `StateWaiting` |
| Last non-empty line is bare `"❯"` | `StateWaiting` |
| Last 3 lines contain a `?`-ending line with length > 10 | `StateWaiting` |
| `fullContent` contains `"(y/n)"` | `StateWaiting` |
| `fullContent` contains `"[Y/n]"` | `StateWaiting` |
| `fullContent` is empty (all whitespace) | `StateStarting` (not `StateWorking` — the actual early-return on empty input at lines 119–121 of monitor.go) |
| `fullContent` has no matching patterns | `StateWorking` (default) |

Edge-case tests:
- Content with ANSI escape sequences wrapping a pattern (e.g., `"\x1b[32mesc to interrupt\x1b[0m"`) — verifies `stripANSI` runs before matching, result should be `StateWorking`
- Question line with length ≤ 10 (e.g., `"Is it?\n"`) — should NOT trigger waiting (`StateWorking`)
- Question line appears only in the 4th-from-last position — should NOT trigger waiting (only last 3 lines checked)
- `fullContent` contains both a working pattern (`"esc to interrupt"`) and a waiting pattern (`"(y/n)"`) — should return `StateWorking` because working checks run first in detectState

**`TestStripANSI`** — verify that `stripANSI` removes ESC sequences without corrupting surrounding text. Test at minimum:
- Plain string → unchanged
- `"\x1b[0m"` sequences removed, text around them preserved
- Multiple consecutive sequences collapsed

**`TestGetLastNonEmptyLines`** — verify extraction of N trailing non-empty lines. Note: the function signature is `getLastNonEmptyLines(lines []string, n int) []string` — it takes a pre-split `[]string`, not a raw multi-line string. Test cases must pass `strings.Split(content, "\n")` as the first argument. Verify:
- Trailing empty strings are skipped
- Returns at most N lines
- Returns fewer than N if not enough non-empty lines exist

---

### P1-2: Fix Hook Injection Failure Visibility

**Problem:** When `hooks.Inject()` fails in `fleet start` (line 127 of `cmd/fleet/main.go`, not 129 — line 127 is the `hooks.Inject` call; line 129 is `stateFilePath = ""`), a warning is printed to stdout but `stateFilePath` is silently set to `""`. The agent starts without hook-based state reporting. From the TUI, the agent appears permanently in `StateWorking` with no indication anything is wrong.

Same failure mode exists in `tui.go → startAgentSession()` (line 222): hooks failure degrades silently.

**Note:** The `internal/fleet/fleet.go` changes in this item depend on P1-4 landing first (they add a new `save()` call path that should be locked). The `tui.go` changes are independent of P1-4.

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

`HooksOK` defaults to `false` on JSON unmarshal (Go zero value), so existing `config.json` files without the field are backwards-compatible. On the next `fleet start`, the field will be set correctly.

Add `UpdateAgentHooks(name string, hooksOK bool) error` to `Fleet` (parallel to existing `UpdateAgentStateFile`).

**Call sites and the boolean value to pass at each:**

| Location | Operation | Value to pass |
|---|---|---|
| `startCmd` after `hooks.Inject` succeeds | Inject succeeded | `true` |
| `startCmd` after `hooks.Inject` fails (warning branch) | Inject failed | `false` |
| `startAgentSession` in `tui.go` after `hooks.Inject` succeeds | Inject succeeded | `true` |
| `startAgentSession` in `tui.go` after `hooks.Inject` fails | Inject failed | `false` |
| `stopCmd` after `hooks.Remove` (success or failure) | Hooks removed | `false` |
| Kill handler in `tui.go` after `hooks.Remove` | Hooks removed | `false` |

**`removeCmd` exclusion:** `removeCmd` calls `hooks.Remove` then immediately calls `f.RemoveAgent(agentName)` which deletes the agent from config entirely. Adding `UpdateAgentHooks` before `RemoveAgent` would write a value that gets immediately discarded. Do not add `UpdateAgentHooks` in `removeCmd`.

**Part B — Surface hook status in TUI and `fleet list`:**

In `AgentDelegate.Render()` (`internal/tui/tui.go`): if `agent.HooksOK == false` and agent status is not `"stopped"`, append `⚠ hooks` (styled in dimmed red) to the status indicator line. This tells users their monitoring is degraded at a glance.

In `listCmd` (`cmd/fleet/main.go`): add a `HOOKS` column to the table output that shows `✓` or `✗`.

The `fleet start` warning message already prints to stdout — keep it. These additions make the failure _persistent_ rather than one-time.

---

### P1-3: Remove Dead Code

**Problem:** Two packages exist with real implementations but zero callers. They add maintenance surface and confuse contributors about the intended architecture.

**Dead packages:**
- `internal/queue/` — 91-line `Queue` / `Request` struct, never used by TUI or any command
- `internal/agent/` — 87-line `Runner` struct with `Start`, `StartDetached`, `IsRunning`, `Kill`, never called (main path uses `tmux.Manager` directly)

**Partial stub:**
- `worktree.List()` (`internal/worktree/worktree.go`, lines 90–107) — runs `git worktree list --porcelain` but discards output (`_ = lines`), always returns empty slice

**Design:**

Delete `internal/queue/queue.go` and `internal/agent/agent.go` entirely. Neither has test files. `go test ./...` should still pass after deletion.

For `worktree.List()`: implement proper parsing of the `--porcelain` output format.

Porcelain format:
```
worktree /path/to/main
HEAD <sha>
branch refs/heads/main

worktree /path/to/feature
HEAD <sha>
branch refs/heads/feature
```

Extract all `worktree <path>` lines by splitting the output on newlines and matching lines with `strings.HasPrefix(line, "worktree ")`. Return the paths as `[]string`.

**Important:** The first entry is always the main worktree (the repo root). Callers who want only fleet-managed worktrees must filter this out themselves. Document this in a comment above the function.

---

### P1-4: File Lock on Fleet Config

**Problem:** `fleet.save()` and mutating methods (`AddAgent`, `UpdateAgent`, etc.) perform read-modify-write against `.fleet/config.json` with no synchronization. The race is logical, not just a torn-write: two concurrent processes load the same fleet state, each mutates their in-memory copy, each writes back. The second write wins and the first's changes are lost.

**Platform note:** This implementation uses `syscall.Flock`, which is available on macOS and Linux but does not exist on Windows. Add `//go:build !windows` at the top of the modified `fleet.go` file, or extract the locking code to a `fleet_unix.go` file with that build tag. Fleet Commander is a tmux-based tool and will not realistically target Windows; documenting this constraint is sufficient.

**Design:**

**`.fleet` directory prerequisite:** The lock file is `.fleet/config.lock`. `lockConfig()` calls `os.OpenFile` on this path. The `.fleet/` directory always exists before any locking call because: `Init` creates it before calling `save()`, and all other operations only run after `Load()` successfully found `config.json` inside `.fleet/`. This invariant is load-bearing — do not call `lockConfig` in code paths where `.fleet/` might not exist.

**Core locking primitive:**

Add two unexported functions to `internal/fleet/fleet.go`:

```go
// acquireLock opens and exclusively flocks .fleet/config.lock.
// The caller must call releaseLock when done.
// Requires f.FleetDir to exist.
func acquireLock(fleetDir string) (*os.File, error) {
    lf, err := os.OpenFile(filepath.Join(fleetDir, "config.lock"), os.O_CREATE|os.O_WRONLY, 0600)
    if err != nil {
        return nil, fmt.Errorf("failed to open lock file: %w", err)
    }
    if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
        lf.Close()
        return nil, fmt.Errorf("failed to acquire config lock: %w", err)
    }
    return lf, nil
}

func releaseLock(lf *os.File) {
    syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
    lf.Close()
}
```

**Why free functions (not methods):** `loadFromPath` is a free function with no `*Fleet` receiver. Using free functions avoids two implementations of the lock path logic.

**Atomic mutation helper — `withLock`:**

Add to `Fleet`:

```go
// withLock acquires an exclusive lock, re-reads the config from disk
// to get the latest state, runs fn (which may mutate f), then saves.
// This prevents logical write conflicts between concurrent fleet commands.
func (f *Fleet) withLock(fn func() error) error {
    lf, err := acquireLock(f.FleetDir)
    if err != nil {
        return err
    }
    defer releaseLock(lf)

    // Re-read from disk while holding the lock so fn sees the latest state.
    fresh, err := readConfig(filepath.Join(f.FleetDir, "config.json"))
    if err != nil {
        return fmt.Errorf("failed to re-read config: %w", err)
    }
    f.Agents = fresh.Agents

    if err := fn(); err != nil {
        return err
    }
    return f.writeConfig()
}
```

`readConfig(path string) (*Fleet, error)` is an unexported helper that reads and unmarshals the JSON file without acquiring any lock (used only inside `withLock` where the lock is already held).

`writeConfig() error` is an unexported helper that marshals and writes `f` to `filepath.Join(f.FleetDir, "config.json")` without acquiring any lock.

**Refactor all mutating operations to use `withLock`:**

`AddAgent`, `RemoveAgent`, `UpdateAgent`, `UpdateAgentStateFile`, and the new `UpdateAgentHooks` (from P1-2) all call `f.save()` today. Change each of these to call `f.withLock(func() error { ... })` instead. The mutation logic moves inside the closure. The closure sees the re-read agent list before it mutates.

`loadFromPath` stays as-is — it is called at process startup before concurrent access is a concern, and it does not need a lock.

`save()` becomes `writeConfig()` (internal only). The old exported-ish `save()` is no longer needed as a standalone path; it's replaced by `withLock` + `writeConfig`.

**Add `.fleet/config.lock` to the gitignore** entry written by `fleet init` (alongside the existing `.fleet` entry).

**No new external dependencies.** `syscall` is stdlib.

---

## Phase 2 — Robustness

### P2-1: CLI Integration Tests

**Problem:** `fleet init`, `fleet add`, `fleet list`, `fleet start`, `fleet stop` have zero test coverage. Regressions in these commands are invisible until a user reports them.

**Design:**

Create `cmd/fleet/integration_test.go` with build tag `//go:build integration`. Run with `go test -tags integration ./cmd/fleet/`. Standard `go test ./...` does not run these.

**Test helper:**

```go
func setupTestRepo(t *testing.T) (repoPath, binaryPath string) {
    t.Helper()
    repoPath = t.TempDir()
    // git init + empty initial commit so worktrees work
    run(t, repoPath, "git", "init")
    run(t, repoPath, "git", "commit", "--allow-empty", "-m", "init")
    // build fleet binary into a temp path
    binaryPath = filepath.Join(t.TempDir(), "fleet")
    run(t, ".", "go", "build", "-o", binaryPath, "./cmd/fleet/")
    return
}
```

`run(t, dir string, name string, args ...string)` is a helper that runs a command in `dir`, calls `t.Fatal` if it fails.

**Tests:**

`TestInitCreatesConfig` — calls `fleet init <repoPath>`, asserts `.fleet/config.json` exists and decodes as valid JSON.

`TestAddCreatesWorktreeAndConfig` — calls `fleet init` then `fleet add myagent feature/my-agent`. Asserts `.fleet/worktrees/myagent` directory exists and `config.json` contains an agent with name `"myagent"`.

`TestListShowsAgents` — calls init + add, then `fleet list`. Asserts stdout contains `"myagent"` and `"feature/my-agent"`.

`TestStopCleansUp` — requires tmux. Skip with `t.Skip("requires tmux")` if `tmux` is not in PATH (check via `exec.LookPath("tmux")`). Calls init + add + start + stop. Asserts the agent's state file does not exist and `.claude/settings.json` in the worktree has no `_fleet` hook entries. No mocking mechanism is available since the binary calls `exec.Command("tmux", ...)` internally; this test is tmux-only.

---

### P2-2: Rollback on Partial `fleet add` Failure

**Problem:** `fleet.AddAgent()` in `internal/fleet/fleet.go` (lines 132–164) creates a git worktree, appends the agent to `f.Agents`, then calls `f.save()` (which after P1-4 becomes `f.withLock(...)`). If the save fails after the worktree is created, the worktree directory exists on disk but the agent is missing from config — orphaned with no recovery path.

**Design:**

This item lands after P1-4 since `AddAgent` will call `f.withLock`. Inside the `withLock` closure, the worktree is already created before the in-memory mutation and save. Structure the closure so that if the save fails, the worktree is cleaned up:

```go
func (f *Fleet) AddAgent(name, branch string) (*Agent, error) {
    // Duplicate name check (before acquiring lock, fast fail)
    for _, a := range f.Agents {
        if a.Name == name {
            return nil, fmt.Errorf("agent %q already exists", name)
        }
    }

    worktreePath := filepath.Join(f.FleetDir, "worktrees", name)
    wt := worktree.NewManager(f.RepoPath)
    if err := wt.Create(worktreePath, branch); err != nil {
        return nil, fmt.Errorf("failed to create worktree: %w", err)
    }

    var created *Agent
    err := f.withLock(func() error {
        // Re-check for duplicate inside lock (another process may have added)
        for _, a := range f.Agents {
            if a.Name == name {
                return fmt.Errorf("agent %q already exists", name)
            }
        }
        created = &Agent{
            Name:         name,
            Branch:       branch,
            WorktreePath: worktreePath,
            Status:       "ready",
        }
        f.Agents = append(f.Agents, created)
        return nil // writeConfig called by withLock after this returns nil
    })

    if err != nil {
        // Rollback: remove the worktree we already created
        if removeErr := wt.Remove(worktreePath); removeErr != nil {
            // Log the orphan path but return the original error
            fmt.Fprintf(os.Stderr, "warning: could not clean up orphaned worktree at %s: %v\n", worktreePath, removeErr)
        }
        return nil, err
    }
    return created, nil
}
```

The duplicate-name re-check inside the lock handles the case where two concurrent `fleet add <same-name>` calls race — the second one to acquire the lock will see the first's agent and fail cleanly.

---

### P2-3: Better TUI Error Display

**Problem:** The kill handler in `tui.go` (`Update()`, lines 308–328) calls `os.Remove(agent.StateFilePath)` and `hooks.Remove(agent.WorktreePath)` but discards both errors. Users have no indication that cleanup failed.

**Design:**

Add two fields to `Model`:

```go
type Model struct {
    // ... existing fields ...
    statusMsg      string
    statusMsgTimer time.Time
}
```

**Important — value receiver constraint:** All `Model` methods use value receivers (`func (m Model)`). A `setStatus` method cannot work because it would mutate a copy. Instead, set the fields as inline assignments within the `Update()` function body, which already mutates `m` and returns it:

```go
// Inside Update(), wherever an error should be surfaced:
m.statusMsg = "⚠ could not remove hooks: " + err.Error()
m.statusMsgTimer = time.Now()
```

**Clearing in `Update()`:** `View()` is read-only in Bubble Tea and cannot clear model state. The refresh tick (`refreshMsg`) fires every 2 seconds. In the `refreshMsg` case of `Update()`, add:

```go
case refreshMsg:
    // ... existing refresh logic ...
    if !m.statusMsgTimer.IsZero() && time.Since(m.statusMsgTimer) >= 5*time.Second {
        m.statusMsg = ""
    }
```

**In `View()`:** After the help text line, if `m.statusMsg != ""`, render it in dimmed red. No time check needed in `View()` — clearing is handled by `Update()` on the next tick.

**Apply to these locations in `Update()`:**
- Kill handler: `os.Remove` fails → set `m.statusMsg`
- Kill handler: `hooks.Remove` fails → set `m.statusMsg`
- `startAgentSession` returns an error → set `m.statusMsg`

**Consolidate `addError`:** The existing `addError string` field on `Model` serves the same purpose. Replace it with `statusMsg` + `statusMsgTimer`. Update the add-agent flow to set `m.statusMsg` instead of `m.addError`. Remove the `addError` field.

---

## Cross-Cutting Concerns

### No new external dependencies

All seven items use only stdlib (`syscall`, `os/exec`, `testing`) or existing dependencies.

### Build tag for Unix-only locking

`syscall.Flock` is available on macOS and Linux but not Windows. Add `//go:build !windows` to the file in `internal/fleet/` that contains the locking code. If the locking helpers are in `fleet.go`, either move them to a new `fleet_unix.go` file or add the build tag to `fleet.go`. Fleet Commander is a tmux-based tool; Windows is not a target. Document this constraint in the README's requirements section.

### Build tags for integration tests

`//go:build integration` on `cmd/fleet/integration_test.go`. Standard `go test ./...` does not run them. CI (when added) should run `go test -tags integration ./cmd/fleet/` in an environment with tmux available for `TestStopCleansUp`.

### `HooksOK` JSON field migration

Backwards-compatible: existing `config.json` files without the field unmarshal with `HooksOK = false`. On next `fleet start`, the field is written correctly.

### Testing `detectState` and helpers

New file `internal/monitor/detect_test.go` uses `package monitor` for access to unexported symbols. The existing `internal/monitor/monitor_test.go` remains `package monitor_test`. Both can coexist.

---

## Implementation Order

Recommended order (dependencies noted):

1. **P1-1** — detectState tests. Pure additive, no code changes, builds confidence.
2. **P1-3** — Dead code removal. Simplifies codebase before other changes touch fleet.go.
3. **P1-4** — Config locking. Foundational. Must land before items that touch `save()` / mutation ops.
4. **P1-2** — Hook failure visibility. The `fleet.go` changes (new `UpdateAgentHooks`) require P1-4; the `tui.go` changes are independent.
5. **P2-2** — AddAgent rollback. Requires P1-4 (AddAgent restructured around `withLock`).
6. **P2-3** — TUI status messages. Independent; UI-only.
7. **P2-1** — Integration tests. Last; validates everything else end-to-end.

---

## Files Changed Per Item

| Item | Files Modified | Files Created | Files Deleted |
|---|---|---|---|
| P1-1 | — | `internal/monitor/detect_test.go` | — |
| P1-2 | `internal/fleet/fleet.go`, `internal/tui/tui.go`, `cmd/fleet/main.go` | — | — |
| P1-3 | `internal/worktree/worktree.go` | — | `internal/queue/queue.go`, `internal/agent/agent.go` |
| P1-4 | `internal/fleet/fleet.go` (or split to `fleet_unix.go`) | — | — |
| P2-1 | — | `cmd/fleet/integration_test.go` | — |
| P2-2 | `internal/fleet/fleet.go` | — | — |
| P2-3 | `internal/tui/tui.go` | — | — |

---

## Success Criteria

- `go test ./...` passes with no failures
- `go test -tags integration ./cmd/fleet/` passes (when tmux is available)
- `internal/monitor` package coverage ≥ 75% (up from 10.9%)
- No silent error discards remain in TUI kill handler
- `fleet list` shows hook status (`✓`/`✗`) per agent
- `fleet add` leaves no orphaned worktrees on save failure
- Concurrent `fleet add` calls produce no lost writes to `.fleet/config.json`
- Dead code packages (`internal/queue`, `internal/agent`) are deleted
- `worktree.List()` returns correct paths (including main worktree as first entry)
