# Tmux Agent Name Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add defense-in-depth agent name validation to the tmux package so it rejects unsafe names before they reach tmux commands.

**Architecture:** Add a `validateAgentName` function using the same regex as `internal/fleet/fleet.go`. Call it as a guard at the top of every method that accepts an `agentName` parameter and passes it to tmux. `SessionExists` stays bool-only and returns `false` for invalid names.

**Tech Stack:** Go, regexp, existing `fakeRunner` test infrastructure

---

### Task 1: Add validation function and tests for invalid names on CreateSession

**Files:**
- Modify: `internal/tmux/tmux.go:1-9` (add `regexp` import and validation function)
- Modify: `internal/tmux/tmux.go:115` (add guard to `CreateSession`)
- Modify: `internal/tmux/tmux_test.go` (add rejection test)

- [ ] **Step 1: Write the failing test for CreateSession rejecting unsafe names**

Add to `internal/tmux/tmux_test.go`:

```go
func TestCreateSession_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)

	badNames := []string{
		"foo:bar",
		"foo.bar",
		"../etc/passwd",
		"name with spaces",
		"",
		"-starts-with-dash",
		"has;semicolon",
		"has$dollar",
	}
	for _, name := range badNames {
		err := m.CreateSession(name, "/tmp/worktree", nil, "")
		if err == nil {
			t.Errorf("expected error for agent name %q, got nil", name)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("expected 0 tmux calls for invalid names, got %d", len(f.calls))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/tmux/... -run TestCreateSession_RejectsUnsafeName -v`
Expected: FAIL — CreateSession does not validate names yet, so no error is returned.

- [ ] **Step 3: Add validateAgentName function and guard in CreateSession**

In `internal/tmux/tmux.go`, add `"regexp"` to the import block:

```go
import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)
```

Add after the import block, before the `CommandRunner` interface:

```go
// safeAgentName validates that an agent name contains only safe characters
// before it is used in tmux commands. This is defense-in-depth — the fleet
// layer also validates, but the tmux layer must not trust its callers.
var safeAgentName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func validateAgentName(name string) error {
	if !safeAgentName.MatchString(name) {
		return fmt.Errorf("unsafe agent name %q: must be alphanumeric with hyphens/underscores", name)
	}
	return nil
}
```

Add as the first line of `CreateSession`:

```go
func (m *Manager) CreateSession(agentName, worktreePath string, command []string, stateFilePath string) error {
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	// ... rest unchanged
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/tmux/... -run TestCreateSession_RejectsUnsafeName -v`
Expected: PASS

- [ ] **Step 5: Run all existing tmux tests to check for regressions**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/tmux/... -v`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tmux/tmux.go internal/tmux/tmux_test.go
git commit -m "feat(tmux): add agent name validation to CreateSession"
```

---

### Task 2: Add validation guards to KillSession, SendKeys, Attach

**Files:**
- Modify: `internal/tmux/tmux.go:170` (guard in `Attach`)
- Modify: `internal/tmux/tmux.go:188` (guard in `KillSession`)
- Modify: `internal/tmux/tmux.go:216` (guard in `SendKeys`)
- Modify: `internal/tmux/tmux_test.go` (add rejection tests)

- [ ] **Step 1: Write failing tests for KillSession, Attach, and SendKeys**

Add to `internal/tmux/tmux_test.go`:

```go
func TestKillSession_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.KillSession("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestAttach_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.Attach("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestSendKeys_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.SendKeys("bad;name", "echo hello")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/tmux/... -run "TestKillSession_RejectsUnsafeName|TestAttach_RejectsUnsafeName|TestSendKeys_RejectsUnsafeName" -v`
Expected: FAIL — no validation guards yet.

- [ ] **Step 3: Add validation guards**

In `internal/tmux/tmux.go`, add the guard as the first line of each method:

`Attach` (line 170):
```go
func (m *Manager) Attach(agentName string) error {
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	// ... rest unchanged
```

`KillSession` (line 188):
```go
func (m *Manager) KillSession(agentName string) error {
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	// ... rest unchanged
```

`SendKeys` (line 216):
```go
func (m *Manager) SendKeys(agentName string, keys string) error {
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	// ... rest unchanged
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/tmux/... -run "TestKillSession_RejectsUnsafeName|TestAttach_RejectsUnsafeName|TestSendKeys_RejectsUnsafeName" -v`
Expected: PASS

- [ ] **Step 5: Run all tmux tests**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/tmux/... -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tmux/tmux.go internal/tmux/tmux_test.go
git commit -m "feat(tmux): add agent name validation to KillSession, Attach, SendKeys"
```

---

### Task 3: Add validation guards to CapturePane, GetPID, SwitchClient, SessionExists

**Files:**
- Modify: `internal/tmux/tmux.go:76` (guard in `SessionExists`)
- Modify: `internal/tmux/tmux.go:222` (guard in `CapturePane`)
- Modify: `internal/tmux/tmux.go:232` (guard in `GetPID`)
- Modify: `internal/tmux/tmux.go:254` (guard in `SwitchClient`)
- Modify: `internal/tmux/tmux_test.go` (add rejection tests)

- [ ] **Step 1: Write failing tests**

Add to `internal/tmux/tmux_test.go`:

```go
func TestSessionExists_ReturnsFalseForUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	if m.SessionExists("bad;name") {
		t.Fatal("SessionExists should return false for unsafe name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestCapturePane_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	_, err := m.CapturePane("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestGetPID_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	_, err := m.GetPID("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}

func TestSwitchClient_RejectsUnsafeName(t *testing.T) {
	f := &fakeRunner{}
	m := NewManagerWithRunner("fleet", f)
	err := m.SwitchClient("bad;name")
	if err == nil {
		t.Fatal("expected error for unsafe agent name")
	}
	if len(f.calls) != 0 {
		t.Error("no tmux commands should execute for invalid names")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/tmux/... -run "TestSessionExists_ReturnsFalseForUnsafeName|TestCapturePane_RejectsUnsafeName|TestGetPID_RejectsUnsafeName|TestSwitchClient_RejectsUnsafeName" -v`
Expected: FAIL

- [ ] **Step 3: Add validation guards**

`SessionExists` — returns `false` for invalid names (no signature change):
```go
func (m *Manager) SessionExists(agentName string) bool {
	if validateAgentName(agentName) != nil {
		return false
	}
	// ... rest unchanged
```

`CapturePane`:
```go
func (m *Manager) CapturePane(agentName string) (string, error) {
	if err := validateAgentName(agentName); err != nil {
		return "", err
	}
	// ... rest unchanged
```

`GetPID`:
```go
func (m *Manager) GetPID(agentName string) (int, error) {
	if err := validateAgentName(agentName); err != nil {
		return 0, err
	}
	// ... rest unchanged
```

`SwitchClient`:
```go
func (m *Manager) SwitchClient(agentName string) error {
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	// ... rest unchanged
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./internal/tmux/... -run "TestSessionExists_ReturnsFalseForUnsafeName|TestCapturePane_RejectsUnsafeName|TestGetPID_RejectsUnsafeName|TestSwitchClient_RejectsUnsafeName" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/bjunya/code/fleet-commander && go test ./... 2>&1 | tail -20`
Expected: All packages PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tmux/tmux.go internal/tmux/tmux_test.go
git commit -m "feat(tmux): add agent name validation to remaining methods"
```
