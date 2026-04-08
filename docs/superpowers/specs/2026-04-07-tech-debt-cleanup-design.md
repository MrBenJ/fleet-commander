# Technical Debt Cleanup — Design Spec

**Date:** 2026-04-07
**Approach:** Outside-In (Approach C) — broad hygiene first, then deepen by package in dependency order
**Pace:** Incremental, one phase per PR (or two for Phase 5)
**Strategic direction:** Migrate state detection toward Claude Code hooks; treat pane scraping as legacy fallback to be phased out

---

## Context

Fleet Commander was vibe-coded rapidly for personal utility. It works, but the codebase has never been reviewed. This cleanup serves two goals:

1. **Stability & extensibility** — harden error handling, reduce brittleness, make the code easier to extend
2. **Understanding** — the author will learn the codebase through the cleanup process itself

No behavior changes unless a bug is found during audit. Each phase is a self-contained PR.

---

## Phase 1: Hygiene Sweep

**Goal:** Quick wins across the whole codebase. Low risk, broad familiarity.

### 1.1 Remove debug output
- Delete `fmt.Fprintf(os.Stderr, "DEBUG tmux: tmux %v\n", debugArgs)` at `internal/tmux/tmux.go:153`
- This prints to stderr on every tmux command. It is not the LaunchLogger (`.fleet/launch.log`), which is intentional and stays
- If tmux command logging is needed later, route it through a proper file-based logger

### 1.2 Extract magic strings and numbers into constants
Each constant lives in the package that owns it. No shared constants package.

| Constant | Package | Current state |
|----------|---------|---------------|
| `".fleet"` directory name | `fleet` | String literal scattered across files |
| `"fleet"` tmux prefix | `tmux` | Hardcoded in `NewManager` |
| `".claude"` directory name | `hooks` | String literal |
| 10-minute state file TTL | `monitor` | Magic number at usage site |
| 15-line pane window | `monitor` | Magic number at usage site |

### 1.3 Fix silently ignored errors
- `tm.KillSession()` in agent removal (`cmd/fleet/cmd_agents.go`) — log warning to stderr on failure
- Audit other fire-and-forget calls; add stderr warnings where failure could mask a real problem
- Lock release `//nolint:errcheck` is intentional (defer pattern) — leave as-is but add a one-line comment explaining why

### 1.4 Consistent error message formatting
- Normalize to `"failed to <verb>: %w"` pattern (already the majority)
- Normalize warning output to `fmt.Fprintf(os.Stderr, "warning: ...")`

**Deliverable:** 1 PR

---

## Phase 2: Core Model — `fleet/`, `state/`, `worktree/`

**Goal:** Understand and harden the data model and persistence layer.

### 2.1 `internal/fleet/` (~480 LOC)
- **Audit `AddAgent` rollback logic** — verify that partial failures (worktree created but config write fails) actually clean up the orphaned worktree
- **Audit `RemoveAgent`** — verify worktree removal handles all edge cases (worktree already deleted, branch already gone)
- **Review `Load()` error paths** — this is the entry point for almost every command. Error messages should be clear about what went wrong (fleet not initialized vs. config corrupted vs. permission denied)
- **Lock file deadlock risk** — `withLock` in `lock.go` has no timeout. If a process crashes holding the lock, everything deadlocks. Options:
  - Add a stale lock detection (check if PID in lock file is still alive)
  - Add a timeout with clear error message
  - At minimum, document the risk and add a `fleet unlock` escape hatch
  - Pick one approach during implementation

### 2.2 `internal/state/` (~80 LOC)
- Verify atomic write pattern (temp file + rename) is correct
- TTL constant extracted in Phase 1; confirm it's used consistently

### 2.3 `internal/worktree/` (~127 LOC)
- Review empty-commit edge case (repo with no HEAD)
- Verify `--force` fallback + `os.RemoveAll` removal is safe (won't delete wrong directory)

**Deliverable:** 1 PR

---

## Phase 3: Integration Layer — `tmux/`, `hooks/`, `context/`

**Goal:** Understand how agents run, signal state, and communicate.

### 3.1 `internal/tmux/` (~258 LOC)
- **Session naming safety** — verify special characters in agent names can't break tmux session names
- **`CapturePane` edge cases** — session doesn't exist, pane is empty, session just died
- **`CommandRunner` interface** — verify tests use it effectively; it's good design, make sure it's leveraged

### 3.2 `internal/hooks/` (~198 LOC)
- **Injection safety audit** — what happens if `.claude/settings.json` has unexpected structure? Does injection corrupt the file?
- **Removal path** — when an agent is removed, are hooks cleaned up from the worktree's settings.json?
- **Idempotency** — what if hooks are injected twice? Does it duplicate entries?

### 3.3 `internal/context/` (~366 LOC)
- Apply same lock fix as Phase 2 (consistent pattern across fleet and context locks)
- Verify channel membership validation works (SendToChannel checks membership)
- Verify `ClearContext` clears exactly what it claims — test each combination of flags

**Deliverable:** 1 PR

---

## Phase 4: Monitor Hardening

**Goal:** Make state detection understandable, documented, and positioned for hooks migration.

**Strategic note:** The long-term direction is full Claude Code hooks-based detection. Pane scraping is legacy fallback. New detection work should go into the hooks path. This phase hardens what exists and documents it for eventual deprecation.

### 4.1 Document every detection pattern
Each regex/string match gets a comment explaining:
- What Claude Code UI element it matches (e.g., "permission prompt", "active spinner")
- Why it's checked in this order relative to other patterns
- When it was last verified against Claude Code's actual output

### 4.2 Review the default-to-working fallback
Currently, unrecognized state defaults to `StateWorking`. Consider whether `StateUnknown` would be safer — it surfaces detection failures instead of hiding them. Trade-off: unknown state in the TUI might be confusing. Decide during implementation.

### 4.3 Edge cases
- 15-line window cutoff: what if the waiting prompt is above line 15 in a long output?
- State file TTL: if hooks stop firing (agent crashes), stale state persists for up to 10 minutes. Document this behavior; consider surfacing staleness in the TUI

### 4.4 Staleness indicator
Consider adding "last state update: Xm ago" to the TUI agent list. Not a hard requirement — decide during implementation based on how it feels in practice.

**Deliverable:** 1 PR

---

## Phase 5: Launch Flow Simplification

**Goal:** Make the most complex part of the codebase readable and testable.

This is the highest-risk phase. The launch flow works but is a ~700 LOC state machine that's hard to follow. It also needs tests before and during the refactor to prevent regressions.

### 5.1 Testing plan (do this FIRST)

**Before any refactoring:**
- Add tests for the existing behavior as-is. These are regression guardrails.
- Focus on the observable behavior, not internal state:
  - `GenerateWithClaude()` parsing: structured markdown input → agent specs out. Cover valid input, malformed input, empty input, Claude returning unexpected format
  - Mode transitions: given mode X and input Y, what mode do we land in?
  - Partial failure: if agent 3 of 5 fails to create, what's the state of agents 1-2?
  - YOLO confirmation gate: verify the multi-flag requirement works

**Test strategy:**
- Unit tests for pure functions (parsing, validation)
- Table-driven tests for mode transitions (input mode + key → expected mode)
- Integration-style tests for the full launch → create → start flow (mock tmux via CommandRunner)

### 5.2 Break up the state machine
- Each mode gets its own update handler (file or clearly-separated function)
- The main `Update()` becomes a dispatcher that routes to the right handler based on current mode
- State fields documented: which fields are active in which modes

### 5.3 Reduce state field ambiguity
- Group mode-specific state into sub-structs or clearly comment which fields belong to which mode
- Goal: when reading code for ModeReview, you can immediately see what state is relevant

### 5.4 Clean up `GenerateWithClaude()`
- The markdown-then-JSON-then-fallback parsing chain is fragile
- Simplify extraction logic
- Add better error messages when parsing fails — "expected JSON array but got: ..." not just "parse error"

### 5.5 Audit partial failure path
- If agent 3 of 5 fails to launch, what happens?
- Are agents 1-2 running? Does the user see which ones succeeded and which failed?
- Make sure the results screen clearly shows per-agent status

**Deliverable:** 2 PRs — one for tests (5.1), one for the refactor (5.2-5.5)

---

## Phase 6: Test Coverage Gaps

**Goal:** Fill remaining coverage holes now that the codebase is understood.

Priority order:

### 6.1 Context channels
- `CreateChannel`, `SendToChannel`, membership validation, `TrimChannel`
- Concurrent write safety (multiple goroutines writing simultaneously)

### 6.2 Hook injection edge cases
- Malformed `.claude/settings.json` (invalid JSON, empty file, missing hooks section)
- Double injection (idempotency)
- Removal after agent deletion

### 6.3 Integration test expansion
- Extend existing integration test to cover full lifecycle: init → add → start → check state → stop → remove
- Test fleet commands from subdirectories (exercises `Load()` walk-up behavior)

### 6.4 Concurrent access stress test (optional)
- Multiple goroutines doing `WriteAgent` / `AppendLog` / `Load` simultaneously
- Verify no corruption under contention
- Validates the flock strategy

**Deliverable:** 1-2 PRs

---

## Phase Summary

| Phase | Focus | Risk | Size | PRs |
|-------|-------|------|------|-----|
| 1 | Hygiene sweep | Low | Small | 1 |
| 2 | Core model (fleet, state, worktree) | Low | Small-Medium | 1 |
| 3 | Integration (tmux, hooks, context) | Low-Medium | Medium | 1 |
| 4 | Monitor hardening | Medium | Medium | 1 |
| 5 | Launch flow simplification | Higher | Large | 2 |
| 6 | Test coverage gaps | Low | Medium | 1-2 |

**Total: 7-8 PRs, done incrementally between feature work.**

Each phase leaves the codebase strictly better than it found it. No phase depends on a later phase being completed — if you stop after Phase 3, you still have a cleaner, more understood codebase.
