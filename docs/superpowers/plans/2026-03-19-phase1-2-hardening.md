# Fleet Commander Phase 1 & 2 Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden Fleet Commander for public release: add test coverage for critical state-detection logic, surface hook failures, remove dead code, lock the config file against concurrent access, add rollback on partial failures, and improve TUI error visibility.

**Architecture:** Seven tasks executed in dependency order. Task 3 (config locking) is foundational — Tasks 4 and 5 depend on it. Tasks 1, 2, 6, and 7 are independent of each other.

**Tech Stack:** Go 1.25 · Bubble Tea (TUI) · Cobra (CLI) · `syscall.Flock` for file locking (stdlib, Unix only) · table-driven tests

**Spec:** `docs/superpowers/specs/2026-03-19-phase1-2-hardening-design.md`

---

## File Map

| File | Action | Task |
|---|---|---|
| `internal/monitor/detect_test.go` | Create | 1 |
| `internal/queue/queue.go` | Delete | 2 |
| `internal/agent/agent.go` | Delete | 2 |
| `internal/worktree/worktree.go` | Modify (implement List) | 2 |
| `internal/fleet/lock.go` | Create (locking infra) | 3 |
| `internal/fleet/fleet.go` | Modify (use withLock, remove save()) | 3, 4, 5 |
| `cmd/fleet/main.go` | Modify (HooksOK, list column) | 4 |
| `internal/tui/tui.go` | Modify (HooksOK indicator, statusMsg) | 4, 6 |
| `cmd/fleet/integration_test.go` | Create | 7 |

---

## Task 1: P1-1 — Test `monitor.detectState()`

**Files:**
- Create: `internal/monitor/detect_test.go`

This is pure test addition — no production code changes. The helpers `detectState`, `stripANSI`, and `getLastNonEmptyLines` are unexported, so the test file must use `package monitor` (not `package monitor_test`) to access them directly. The existing `internal/monitor/monitor_test.go` is `package monitor_test` and is left unchanged.

- [ ] **Step 1: Create `internal/monitor/detect_test.go` with the full test suite**

```go
package monitor

import (
	"strings"
	"testing"
)

func TestDetectState(t *testing.T) {
	tests := []struct {
		name        string
		lastLine    string
		fullContent string
		want        AgentState
	}{
		// Working patterns
		{
			name:        "esc to interrupt",
			fullContent: "Claude is thinking...\nesc to interrupt",
			want:        StateWorking,
		},
		{
			name:        "spinner char braille 1",
			fullContent: "Processing ⠋",
			want:        StateWorking,
		},
		{
			name:        "spinner char braille 2",
			fullContent: "Processing ⠹",
			want:        StateWorking,
		},

		// Waiting patterns
		{
			name:        "Esc to cancel",
			fullContent: "Do you want to run this?\nEsc to cancel",
			want:        StateWaiting,
		},
		{
			name:        "Do you want to proceed",
			fullContent: "Do you want to proceed",
			want:        StateWaiting,
		},
		{
			name:        "accept edits",
			fullContent: "Review the changes\naccept edits",
			want:        StateWaiting,
		},
		{
			name:        "shift+tab to cycle",
			fullContent: "Select an option\nshift+tab to cycle",
			want:        StateWaiting,
		},
		{
			name:        "numbered menu with arrow",
			fullContent: "❯ 1. Option one\n  2. Option two",
			want:        StateWaiting,
		},
		{
			name:        "bare arrow prompt",
			fullContent: "Some output\n❯",
			want:        StateWaiting,
		},
		{
			name:        "question ending in last 3 lines",
			fullContent: "line1\nline2\nline3\nline4\nShould I continue with this approach?",
			want:        StateWaiting,
		},
		{
			name:        "y/n prompt",
			fullContent: "Delete the file? (y/n)",
			want:        StateWaiting,
		},
		{
			name:        "Y/n prompt",
			fullContent: "Delete the file? [Y/n]",
			want:        StateWaiting,
		},

		// Edge cases
		{
			name:        "empty content returns StateStarting",
			fullContent: "",
			want:        StateStarting,
		},
		{
			name:        "whitespace-only content returns StateStarting",
			fullContent: "   \n\n   ",
			want:        StateStarting,
		},
		{
			name:        "no matching patterns returns StateWorking",
			fullContent: "Claude finished the task.",
			want:        StateWorking,
		},
		{
			name:        "ansi-wrapped working pattern is stripped before matching",
			fullContent: "\x1b[32mesc to interrupt\x1b[0m",
			want:        StateWorking,
		},
		{
			name:        "short question line does NOT trigger waiting",
			fullContent: "Is it?",
			want:        StateWorking, // len("Is it?") == 6, ≤ 10 → no match
		},
		{
			name: "question in 4th-from-last line does NOT trigger waiting",
			// last 3 non-empty: "line5", "line6", "line7"
			// question is at 4th from the end
			fullContent: "Should I continue with this approach?\nline5\nline6\nline7",
			want:        StateWorking,
		},
		{
			name:        "working pattern beats waiting pattern (working checked first)",
			fullContent: "esc to interrupt\n(y/n)",
			want:        StateWorking,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastLine := getLastNonEmptyLine(tt.fullContent)
			got := detectState(lastLine, tt.fullContent)
			if got != tt.want {
				t.Errorf("detectState(%q) = %q, want %q", tt.fullContent, got, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain string unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "reset sequence removed",
			input: "before\x1b[0mafter",
			want:  "beforeafter",
		},
		{
			name:  "color sequence removed text preserved",
			input: "\x1b[32mgreen text\x1b[0m",
			want:  "green text",
		},
		{
			name:  "multiple sequences",
			input: "\x1b[1m\x1b[32mbold green\x1b[0m",
			want:  "bold green",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetLastNonEmptyLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  []string
	}{
		{
			name:  "returns last N non-empty",
			input: "line1\nline2\nline3\nline4\nline5",
			n:     3,
			want:  []string{"line5", "line4", "line3"},
		},
		{
			name:  "skips trailing empty lines",
			input: "line1\nline2\nline3\n\n\n",
			n:     2,
			want:  []string{"line3", "line2"},
		},
		{
			name:  "returns fewer than N when not enough non-empty lines",
			input: "line1\nline2",
			n:     5,
			want:  []string{"line2", "line1"},
		},
		{
			name:  "empty input returns empty slice",
			input: "",
			n:     3,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.input, "\n")
			got := getLastNonEmptyLines(lines, tt.n)
			if len(got) != len(tt.want) {
				t.Fatalf("getLastNonEmptyLines(%q, %d) = %v (len %d), want %v (len %d)",
					tt.input, tt.n, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to confirm they all pass**

```bash
go test ./internal/monitor/... -v -run "TestDetectState|TestStripANSI|TestGetLastNonEmptyLines"
```

Expected: all test cases PASS. If any fail, the production code in `monitor.go` has different behavior than documented — fix the test expectation to match the actual behavior (the code is the source of truth here).

- [ ] **Step 3: Verify overall monitor coverage improved**

```bash
go test ./internal/monitor/... -cover
```

Expected: coverage ≥ 75% (was 10.9%).

- [ ] **Step 4: Run full test suite to confirm nothing regressed**

```bash
go test ./...
```

Expected: all existing tests still PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/monitor/detect_test.go
git commit -m "test: add table-driven coverage for monitor.detectState, stripANSI, and getLastNonEmptyLines"
```

---

## Task 2: P1-3 — Remove Dead Code, Fix `worktree.List()`

**Files:**
- Delete: `internal/queue/queue.go`
- Delete: `internal/agent/agent.go`
- Modify: `internal/worktree/worktree.go`

No callers exist for either dead package. `worktree.List()` runs the correct git command but discards the output — fix it to return proper paths.

- [ ] **Step 1: Delete the dead packages**

```bash
rm internal/queue/queue.go internal/agent/agent.go
```

- [ ] **Step 2: Confirm the build still passes**

```bash
go build ./...
```

Expected: no errors. If any import of `queue` or `agent` appears, remove it.

- [ ] **Step 3: Implement `worktree.List()` properly**

Open `internal/worktree/worktree.go`. Replace the stub body of `List()` (lines 90–107) with:

```go
// List returns all worktree paths for the repo.
// The first entry is always the main worktree (the repo root itself).
// Callers who want only fleet-managed worktrees must filter it out.
func (m *Manager) List() ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.RepoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			worktrees = append(worktrees, strings.TrimPrefix(line, "worktree "))
		}
	}
	return worktrees, nil
}
```

Check that `strings` is already imported in the file. If not, add it to the import block.

- [ ] **Step 4: Write a test for `worktree.List()`**

Look at `internal/worktree/worktree.go` — there is no existing test file. Create `internal/worktree/worktree_test.go`:

```go
package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/teknal/fleet-commander/internal/worktree"
)

func TestListReturnsMainWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	// Create a real git repo in a temp dir
	repoDir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}
	run("git", "init")
	run("git", "commit", "--allow-empty", "-m", "init")

	m := worktree.NewManager(repoDir)
	paths, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("List() returned empty slice, expected at least the main worktree")
	}
	// First entry should be the repo dir itself
	// git returns absolute paths; resolve the temp dir to match
	absRepo, _ := filepath.Abs(repoDir)
	// Check we get exactly one worktree (the main one) and it matches
	if len(paths) != 1 {
		t.Errorf("expected 1 worktree, got %d: %v", len(paths), paths)
	}
	if paths[0] != absRepo {
		// Some git versions may resolve symlinks differently — just check it exists
		if _, err := os.Stat(paths[0]); err != nil {
			t.Errorf("worktree path %q does not exist: %v", paths[0], err)
		}
	}
}
```

- [ ] **Step 5: Run the new test**

```bash
go test ./internal/worktree/... -v
```

Expected: PASS. If git is not installed, the test is skipped gracefully.

- [ ] **Step 6: Run the full suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git rm internal/queue/queue.go internal/agent/agent.go
git commit -m "refactor: remove unused queue and agent packages, implement worktree.List() parsing"
```

---

## Task 3: P1-4 — Config File Locking

**Files:**
- Create: `internal/fleet/lock.go`
- Modify: `internal/fleet/fleet.go`

This is the biggest structural change. We add a `withLock` helper that: (1) acquires an exclusive `flock` on `.fleet/config.lock`, (2) re-reads the config from disk inside the lock, (3) runs the caller's mutation, (4) writes back, (5) releases the lock. All mutating methods (`AddAgent`, `RemoveAgent`, `UpdateAgent`, `UpdateAgentStateFile`) are refactored to use it. This prevents lost writes when two fleet commands run concurrently.

`syscall.Flock` is only available on Unix. The locking code goes in a separate file with `//go:build !windows` so the constraint is explicit. Fleet Commander requires tmux and is not supported on Windows.

- [ ] **Step 1: Create `internal/fleet/lock.go`**

```go
//go:build !windows

package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquireLock opens .fleet/config.lock and acquires an exclusive flock.
// The caller must call releaseLock when the critical section is done.
// Requires fleetDir to already exist.
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

// releaseLock releases the flock and closes the file.
func releaseLock(lf *os.File) {
	syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck
	lf.Close()
}

// readConfig reads and parses the fleet config at path without acquiring any lock.
// Only call this from within withLock where the lock is already held.
func readConfig(path string) (*Fleet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var f Fleet
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &f, nil
}

// writeConfig marshals f and writes it to disk without acquiring any lock.
// Only call this from within withLock where the lock is already held,
// or from Init() where no concurrent access is possible.
func (f *Fleet) writeConfig() error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	configPath := filepath.Join(f.FleetDir, "config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// withLock acquires an exclusive lock on .fleet/config.lock, re-reads the
// config from disk so fn sees the latest state, runs fn (which mutates f),
// then writes back to disk. This prevents logical write conflicts between
// concurrent fleet commands.
func (f *Fleet) withLock(fn func() error) error {
	lf, err := acquireLock(f.FleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	// Re-read from disk inside the lock so we always have the latest state.
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

- [ ] **Step 2: Refactor `fleet.go` to use the new locking helpers**

In `internal/fleet/fleet.go`, make these changes:

**a) Remove the `save()` method** (lines 115–129) — it is replaced by `writeConfig()` in `lock.go`.

**b) Update `Init()` to call `writeConfig()` instead of `save()`** — change:
```go
if err := f.save(); err != nil {
    return nil, fmt.Errorf("failed to save fleet config: %w", err)
}
```
to:
```go
if err := f.writeConfig(); err != nil {
    return nil, fmt.Errorf("failed to save fleet config: %w", err)
}
```

**c) Refactor `RemoveAgent()`** — replace the whole body with:
```go
func (f *Fleet) RemoveAgent(name string) error {
	return f.withLock(func() error {
		for i, a := range f.Agents {
			if a.Name == name {
				f.Agents = append(f.Agents[:i], f.Agents[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}
```

**d) Refactor `UpdateAgent()`**:
```go
func (f *Fleet) UpdateAgent(name string, status string, pid int) error {
	return f.withLock(func() error {
		for _, a := range f.Agents {
			if a.Name == name {
				a.Status = status
				a.PID = pid
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}
```

**e) Refactor `UpdateAgentStateFile()`**:
```go
func (f *Fleet) UpdateAgentStateFile(name, stateFilePath string) error {
	return f.withLock(func() error {
		for _, a := range f.Agents {
			if a.Name == name {
				a.StateFilePath = stateFilePath
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}
```

**f) Refactor `AddAgent()` — add rollback guard (this fulfils P2-2 at the same time)**:

Replace the entire `AddAgent` function:
```go
// AddAgent creates a new agent workspace with a git worktree.
// If the config save fails after the worktree is created, the worktree is
// cleaned up automatically to avoid leaving orphaned directories.
func (f *Fleet) AddAgent(name, branch string) (*Agent, error) {
	// Fast-fail for duplicate before acquiring lock.
	for _, a := range f.Agents {
		if a.Name == name {
			return nil, fmt.Errorf("agent '%s' already exists", name)
		}
	}

	worktreePath := filepath.Join(f.FleetDir, "worktrees", name)
	wt := worktree.NewManager(f.RepoPath)
	if err := wt.Create(worktreePath, branch); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	var created *Agent
	err := f.withLock(func() error {
		// Re-check inside the lock: another concurrent process may have added
		// an agent with the same name between our fast-fail check and now.
		for _, a := range f.Agents {
			if a.Name == name {
				return fmt.Errorf("agent '%s' already exists", name)
			}
		}
		created = &Agent{
			Name:         name,
			Branch:       branch,
			WorktreePath: worktreePath,
			Status:       "ready",
			PID:          0,
		}
		f.Agents = append(f.Agents, created)
		return nil // withLock calls writeConfig after this returns nil
	})

	if err != nil {
		// Rollback: remove the worktree we already created so it doesn't orphan.
		if removeErr := wt.Remove(worktreePath); removeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not clean up orphaned worktree at %s: %v\n",
				worktreePath, removeErr)
		}
		return nil, err
	}
	return created, nil
}
```

**g) Update `Init()` to also add `config.lock` to `.gitignore`** — after the existing `addToGitignore(absPath, ".fleet")` call, add:
```go
addToGitignore(absPath, ".fleet/config.lock")
```

Note: `.fleet` itself is already gitignored, so `.fleet/config.lock` would already be excluded. This line is a belt-and-suspenders addition that makes the intent explicit.

- [ ] **Step 3: Build to confirm no compilation errors**

```bash
go build ./...
```

Expected: no errors. If you see "undefined: withLock" or similar, check the build tag on `lock.go` is `//go:build !windows` (not `// +build !windows`).

- [ ] **Step 4: Write a concurrency test for `withLock`**

**Important:** `syscall.Flock` is a per-process lock — it only prevents concurrent access from separate OS processes. Goroutines within the same process are NOT protected by `Flock`. The concurrency test must therefore launch separate processes (subprocesses), not goroutines.

The test builds the `fleet` binary and runs N concurrent `fleet add` subprocesses against the same repo. All N agents must be present in the final config.

Create `internal/fleet/fleet_test.go`:

```go
//go:build integration

package fleet_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	run("git", "commit", "--allow-empty", "-m", "init")
	return dir
}

func TestConcurrentAddAgentNoLostWrites(t *testing.T) {
	repoDir := initTestRepo(t)

	// Build the fleet binary so we can run it as a subprocess.
	binaryDir := t.TempDir()
	binaryPath := filepath.Join(binaryDir, "fleet")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/fleet/")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Initialize the fleet using the binary.
	initCmd := exec.Command(binaryPath, "init", repoDir)
	initCmd.Dir = repoDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("fleet init failed: %v\n%s", err, out)
	}

	// Launch N concurrent subprocesses, each adding a different agent.
	// syscall.Flock protects across processes — this tests the actual lock.
	const n = 5
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(binaryPath, "add",
				fmt.Sprintf("agent%d", i),
				fmt.Sprintf("feature/branch%d", i),
			)
			cmd.Dir = repoDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				errs[i] = fmt.Errorf("fleet add agent%d: %v\noutput: %s", i, err, out)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("subprocess %d failed: %v", i, err)
		}
	}

	// All n agents must be in the config — no lost writes.
	data, err := os.ReadFile(filepath.Join(repoDir, ".fleet", "config.json"))
	if err != nil {
		t.Fatalf("could not read config.json: %v", err)
	}
	var config struct {
		Agents []struct{ Name string }
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("could not parse config.json: %v\ncontent: %s", err, data)
	}
	if len(config.Agents) != n {
		t.Errorf("expected %d agents after concurrent adds, got %d — possible lost write\nconfig: %s",
			n, len(config.Agents), data)
	}
}
```

Note: this test is tagged `//go:build integration` because it builds the `fleet` binary. Run it with:
```bash
go test -tags integration ./internal/fleet/... -v -run TestConcurrentAddAgentNoLostWrites
```

- [ ] **Step 5: Run the concurrency test**

```bash
go test -tags integration ./internal/fleet/... -v -run TestConcurrentAddAgentNoLostWrites
```

Expected: PASS with all 5 agents present. If count < 5, the locking implementation has a bug — check that `withLock` re-reads from disk and that the lock file path is consistent across subprocesses.

- [ ] **Step 6: Run the full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/fleet/lock.go internal/fleet/fleet.go internal/fleet/fleet_test.go
git commit -m "feat: add exclusive file locking for fleet config to prevent concurrent write conflicts"
```

---

## Task 4: P1-2 — Hook Injection Failure Visibility

**Files:**
- Modify: `internal/fleet/fleet.go`
- Modify: `cmd/fleet/main.go`
- Modify: `internal/tui/tui.go`

When `hooks.Inject()` fails, the agent currently runs with degraded monitoring and the user has no persistent indicator. This adds a `HooksOK` field to `Agent`, surfaces it in `fleet list`, and shows a warning icon in the TUI.

- [ ] **Step 1: Add `HooksOK` to `Agent` struct and `UpdateAgentHooks` method**

In `internal/fleet/fleet.go`:

**a) Add `HooksOK` to the `Agent` struct:**
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

**b) Add `UpdateAgentHooks` method** (after `UpdateAgentStateFile`):
```go
// UpdateAgentHooks records whether fleet hooks are currently injected for an agent.
func (f *Fleet) UpdateAgentHooks(name string, hooksOK bool) error {
	return f.withLock(func() error {
		for _, a := range f.Agents {
			if a.Name == name {
				a.HooksOK = hooksOK
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}
```

- [ ] **Step 2: Call `UpdateAgentHooks` in `startCmd` (`cmd/fleet/main.go`)**

In `startCmd`, after the hooks injection block (around line 127–131), add calls:

```go
if err := hooks.Inject(agent.WorktreePath); err != nil {
    // Non-fatal: common cause is malformed existing .claude/settings.json — check that file first.
    fmt.Printf("Warning: could not inject hooks into %s (.claude/settings.json may be malformed): %v\n", agent.WorktreePath, err)
    stateFilePath = ""
    f.UpdateAgentHooks(agentName, false) // mark hooks as not injected
} else {
    f.UpdateAgentHooks(agentName, true)
}
```

Also in `stopCmd`, after `hooks.Remove` (around line 217–219), add:
```go
if err := hooks.Remove(agent.WorktreePath); err != nil {
    fmt.Printf("Warning: could not remove hooks: %v\n", err)
}
f.UpdateAgentHooks(agentName, false) // hooks are gone (or never injected)
```

- [ ] **Step 3: Add HOOKS column to `fleet list`**

In `listCmd` (`cmd/fleet/main.go`), update the table output:

```go
fmt.Println("AGENT\t\tBRANCH\t\t\tSTATUS\t\tHOOKS\tPID")
fmt.Println("─────\t\t──────\t\t\t──────\t\t─────\t───")
for _, a := range f.Agents {
    pid := "-"
    if a.PID != 0 {
        pid = fmt.Sprintf("%d", a.PID)
    }
    hooksStatus := "✗"
    if a.HooksOK {
        hooksStatus = "✓"
    }
    fmt.Printf("%-15s %-23s %-10s %-7s %s\n", a.Name, a.Branch, a.Status, hooksStatus, pid)
}
```

- [ ] **Step 4: Add `⚠ hooks` indicator to TUI agent items**

In `internal/tui/tui.go`, update `AgentDelegate.Render()`. After the `indicator` variable is set (around line 97–106), add a warning suffix if hooks are not OK and the agent is not stopped:

```go
// Add hooks warning if monitoring is degraded (use persisted Status, not live state,
// so the warning persists across refreshes even if pane is not captured yet).
if !agent.HooksOK && agent.Status != "stopped" {
    hooksWarnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6666"))
    indicator += " " + hooksWarnStyle.Render("⚠ hooks")
}
```

Also update `startAgentSession` in `tui.go` to call `UpdateAgentHooks`. The current code (around line 222–224):
```go
if err := hooks.Inject(agent.WorktreePath); err != nil {
    stateFilePath = "" // degrade gracefully
}
```

Change to:
```go
if err := hooks.Inject(agent.WorktreePath); err != nil {
    stateFilePath = "" // degrade gracefully
    m.fleet.UpdateAgentHooks(agent.Name, false)
} else {
    m.fleet.UpdateAgentHooks(agent.Name, true)
}
```

In the kill handler (TUI "k" key, around line 323), after `hooks.Remove`:
```go
hooks.Remove(agent.WorktreePath) // best-effort
```
Change to:
```go
hooks.Remove(agent.WorktreePath) // best-effort; error surfaced in Task 6 (P2-3)
m.fleet.UpdateAgentHooks(agent.Name, false)
```

- [ ] **Step 5: Build to confirm no compilation errors**

```bash
go build ./...
```

- [ ] **Step 6: Run the full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/fleet/fleet.go cmd/fleet/main.go internal/tui/tui.go
git commit -m "feat: persist and surface hook injection status per agent in list and TUI"
```

---

## Task 5: P2-2 — Rollback on Partial `fleet add` Failure

This task is **already done** as part of Task 3 (Step 2f). The new `AddAgent` implementation in Task 3 includes the rollback guard: if `withLock` returns an error after the worktree is created, `wt.Remove(worktreePath)` is called to clean up.

**Verify the rollback is in place:**

- [ ] **Step 1: Confirm the rollback guard is present in `fleet.go`**

```bash
grep -A5 "Rollback" internal/fleet/fleet.go
```

Expected output contains:
```
// Rollback: remove the worktree we already created so it doesn't orphan.
if removeErr := wt.Remove(worktreePath); removeErr != nil {
```

If the grep returns nothing, re-apply the `AddAgent` changes from Task 3, Step 2f.

- [ ] **Step 2: Mark complete — no additional code needed**

```bash
echo "P2-2 rollback guard verified as part of Task 3"
```

---

## Task 6: P2-3 — TUI Status Messages

**Files:**
- Modify: `internal/tui/tui.go`

The TUI currently silently discards errors in the kill handler. This adds a `statusMsg` field shown at the bottom of the list view, auto-clearing after 5 seconds via the existing refresh tick. It also replaces the separate `addError` field.

- [ ] **Step 1: Update `Model` struct — add `statusMsg`/`statusMsgTimer`, remove `addError`**

In `internal/tui/tui.go`, change the `Model` struct:

```go
type Model struct {
	list           list.Model
	fleet          *fleet.Fleet
	tmux           *tmux.Manager
	monitor        *monitor.Monitor
	width          int
	height         int
	quitting       bool
	attachAgent    string
	mode           inputMode
	nameInput      textinput.Model
	branchInput    textinput.Model
	statusMsg      string
	statusMsgTimer time.Time
}
```

(Remove `addError string`, add `statusMsg string` and `statusMsgTimer time.Time`.)

- [ ] **Step 2: Update the kill handler to surface errors**

In `Update()`, find the `"k"` case (around lines 308–328). Change the cleanup block from:

```go
if agent.StateFilePath != "" {
    if err := os.Remove(agent.StateFilePath); err != nil {
        // best-effort, don't fail the UI
        _ = err
    }
    m.fleet.UpdateAgentStateFile(agent.Name, "")
    hooks.Remove(agent.WorktreePath) // best-effort
}
```

To:

```go
if agent.StateFilePath != "" {
    if err := os.Remove(agent.StateFilePath); err != nil {
        m.statusMsg = "⚠ could not remove state file: " + err.Error()
        m.statusMsgTimer = time.Now()
    }
    m.fleet.UpdateAgentStateFile(agent.Name, "")
    if err := hooks.Remove(agent.WorktreePath); err != nil {
        m.statusMsg = "⚠ could not remove hooks: " + err.Error()
        m.statusMsgTimer = time.Now()
    }
    m.fleet.UpdateAgentHooks(agent.Name, false)
}
```

- [ ] **Step 3: Surface `startAgentSession` errors in both the "enter" and "s" handlers**

In `Update()`, find both places where `startAgentSession` is called and its error is silently dropped:

```go
if err := m.startAgentSession(agent); err != nil {
    return m, nil
}
```

Change both occurrences to:

```go
if err := m.startAgentSession(agent); err != nil {
    m.statusMsg = "⚠ failed to start agent: " + err.Error()
    m.statusMsgTimer = time.Now()
    return m, nil
}
```

- [ ] **Step 4: Add status message clearing to the `refreshMsg` case**

In `Update()`, find the `refreshMsg` case (around lines 255–260):

```go
case refreshMsg:
    items := buildItems(m.fleet, m.tmux, m.monitor)
    m.list.SetItems(items)
    return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
        return refreshMsg{}
    })
```

Add the clearing check before the return:

```go
case refreshMsg:
    items := buildItems(m.fleet, m.tmux, m.monitor)
    m.list.SetItems(items)
    if !m.statusMsgTimer.IsZero() && time.Since(m.statusMsgTimer) >= 5*time.Second {
        m.statusMsg = ""
        m.statusMsgTimer = time.Time{} // reset timer so it doesn't keep firing
    }
    return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
        return refreshMsg{}
    })
```

- [ ] **Step 5: Replace `addError` references in `updateAddMode` and `View()`**

In `updateAddMode()`, replace all `m.addError = "..."` with the inline pair:
```go
m.statusMsg = "<the message>"
m.statusMsgTimer = time.Now()
```

Specifically:
- Line ~358: `m.addError = "Name cannot be empty"` → `m.statusMsg = "Name cannot be empty"; m.statusMsgTimer = time.Now()`
- Line ~370: `m.addError = "Branch cannot be empty"` → `m.statusMsg = "Branch cannot be empty"; m.statusMsgTimer = time.Now()`
- Line ~377: `m.addError = err.Error()` → `m.statusMsg = err.Error(); m.statusMsgTimer = time.Now()`
- Lines with `m.addError = ""` (ESC cancel ~351, successful add ~385) → change to `m.statusMsg = ""; m.statusMsgTimer = time.Time{}` (reset both fields together so the timer doesn't keep firing on empty messages)

In `Update()`, find where `m.addError = ""` is set when entering add-agent mode (around line 273) — change it to `m.statusMsg = ""; m.statusMsgTimer = time.Time{}` to clear any pending status on add-mode entry.

In `View()`, find the add-agent form branch (around line 433):
```go
if m.addError != "" {
    s += "\n" + stoppedStyle.Render("  ❌ " + m.addError)
}
```
Change to:
```go
if m.statusMsg != "" {
    s += "\n" + stoppedStyle.Render("  ❌ " + m.statusMsg)
}
```

- [ ] **Step 6: Render `statusMsg` in the main list view**

In `View()`, find the list view return (around line 466–471):

```go
return fmt.Sprintf(
    "%s\n%s\n%s",
    m.list.View(),
    summary,
    help,
)
```

Change to:

```go
view := fmt.Sprintf(
    "%s\n%s\n%s",
    m.list.View(),
    summary,
    help,
)
if m.statusMsg != "" {
    errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6666"))
    view += "\n" + errorStyle.Render(m.statusMsg)
}
return view
```

- [ ] **Step 7: Build to confirm no compilation errors**

```bash
go build ./...
```

If you see `m.addError undefined`, you missed a reference — grep for remaining uses:

```bash
grep -n "addError" internal/tui/tui.go
```

Expected: no output.

- [ ] **Step 8: Run the full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/tui.go
git commit -m "feat: surface TUI errors in status bar with auto-clear, consolidate addError into statusMsg"
```

---

## Task 7: P2-1 — CLI Integration Tests

**Files:**
- Create: `cmd/fleet/integration_test.go`

These tests run the actual compiled `fleet` binary against a real git repo. They are gated behind `//go:build integration` so standard `go test ./...` does not run them. Run with `go test -tags integration ./cmd/fleet/ -v`.

- [ ] **Step 1: Create `cmd/fleet/integration_test.go`**

```go
//go:build integration

package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temp git repo and builds the fleet binary.
// Returns the repo path and the path to the compiled fleet binary.
func setupTestRepo(t *testing.T) (repoPath, binaryPath string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	// Create and initialize a temp git repo
	repoPath = t.TempDir()
	run(t, repoPath, "git", "init")
	run(t, repoPath, "git", "config", "user.email", "test@test.com")
	run(t, repoPath, "git", "config", "user.name", "Test")
	run(t, repoPath, "git", "commit", "--allow-empty", "-m", "init")

	// Build the fleet binary
	binaryDir := t.TempDir()
	binaryPath = filepath.Join(binaryDir, "fleet")
	// Run go build from the module root (current working dir when tests run)
	run(t, ".", "go", "build", "-o", binaryPath, "./cmd/fleet/")

	return
}

// run executes a command in dir, failing the test on error.
func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "." {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %q %v in %q failed: %v\noutput: %s", name, args, dir, err, out)
	}
	return string(out)
}

// fleet runs the fleet binary with the given args in repoPath.
func runFleet(t *testing.T, binaryPath, repoPath string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fleet %v failed: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

func TestInitCreatesConfig(t *testing.T) {
	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)

	configPath := filepath.Join(repoPath, ".fleet", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config.json not created: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("config.json is not valid JSON: %v\ncontent: %s", err, data)
	}

	if config["repo_path"] == "" {
		t.Error("config.json missing repo_path field")
	}
}

func TestAddCreatesWorktreeAndConfig(t *testing.T) {
	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)
	runFleet(t, binaryPath, repoPath, "add", "myagent", "feature/my-agent")

	// Worktree directory must exist
	worktreePath := filepath.Join(repoPath, ".fleet", "worktrees", "myagent")
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("worktree directory %q was not created", worktreePath)
	}

	// Config must contain the agent
	data, _ := os.ReadFile(filepath.Join(repoPath, ".fleet", "config.json"))
	if !strings.Contains(string(data), `"myagent"`) {
		t.Errorf("config.json does not contain agent name 'myagent'\ncontent: %s", data)
	}
	if !strings.Contains(string(data), `"feature/my-agent"`) {
		t.Errorf("config.json does not contain branch 'feature/my-agent'\ncontent: %s", data)
	}
}

func TestListShowsAgents(t *testing.T) {
	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)
	runFleet(t, binaryPath, repoPath, "add", "myagent", "feature/my-agent")

	out := runFleet(t, binaryPath, repoPath, "list")

	if !strings.Contains(out, "myagent") {
		t.Errorf("fleet list output does not contain agent name 'myagent'\noutput: %s", out)
	}
	if !strings.Contains(out, "feature/my-agent") {
		t.Errorf("fleet list output does not contain branch 'feature/my-agent'\noutput: %s", out)
	}
}

func TestStopCleansUp(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not in PATH")
	}
	// Verify a tmux server is actually running — tmux binary present ≠ server running.
	// In CI containers, tmux is often installed but no server is started.
	if err := exec.Command("tmux", "list-sessions").Run(); err != nil {
		t.Skip("no running tmux server — start one with 'tmux new-session -d' before running this test")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not in PATH — fleet start requires Claude Code CLI")
	}

	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)
	runFleet(t, binaryPath, repoPath, "add", "myagent", "feature/my-agent")
	runFleet(t, binaryPath, repoPath, "start", "myagent")
	runFleet(t, binaryPath, repoPath, "stop", "myagent")

	// State file must be gone
	stateFile := filepath.Join(repoPath, ".fleet", "states", "myagent.json")
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Errorf("state file %q still exists after stop", stateFile)
	}

	// .claude/settings.json in the worktree must have no _fleet hooks
	settingsPath := filepath.Join(repoPath, ".fleet", "worktrees", "myagent", ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err == nil { // settings.json may not exist if it was never created
		if strings.Contains(string(data), `"_fleet"`) {
			t.Errorf("settings.json still contains _fleet hooks after stop\ncontent: %s", data)
		}
	}
}
```

- [ ] **Step 2: Run the integration tests (init, add, list — skip stop if tmux unavailable)**

```bash
go test -tags integration ./cmd/fleet/ -v -run "TestInitCreatesConfig|TestAddCreatesWorktreeAndConfig|TestListShowsAgents"
```

Expected: PASS for all three. These tests do not require tmux or Claude.

- [ ] **Step 3: Run standard test suite to confirm integration tests are excluded by default**

```bash
go test ./...
```

Expected: integration tests do NOT run (no mention of them). All other tests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/fleet/integration_test.go
git commit -m "test: add CLI integration tests for init, add, list, and stop commands"
```

---

## Final Verification

- [ ] **Run the full test suite one last time**

```bash
go test ./...
go test -race ./internal/fleet/...
go test -tags integration ./cmd/fleet/ -v -run "TestInitCreatesConfig|TestAddCreatesWorktreeAndConfig|TestListShowsAgents"
```

Expected: all PASS. No race conditions.

- [ ] **Check monitor coverage hit the target**

```bash
go test ./internal/monitor/... -cover
```

Expected: ≥ 75%.

- [ ] **Verify dead packages are gone**

```bash
ls internal/queue/ internal/agent/ 2>&1
```

Expected: `No such file or directory` for both.

- [ ] **Verify `worktree.List()` has no stub**

```bash
grep "_ = lines" internal/worktree/worktree.go
```

Expected: no output.

- [ ] **Verify no silent error discards remain in TUI kill handler**

```bash
grep "_ = err" internal/tui/tui.go
```

Expected: no output.
