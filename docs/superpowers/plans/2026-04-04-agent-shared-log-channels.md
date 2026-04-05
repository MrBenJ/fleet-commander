# Agent Shared Log & Channels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an append-only shared log and private channel system so fleet agents can communicate with each other and post findings to a shared bulletin board.

**Architecture:** Extend the existing `Context` struct in `internal/context/context.go` with a `Log []LogEntry` field and a `Channels map[string]*Channel` field. All operations use the same flock-based read-modify-write pattern already established. CLI commands are added to `cmd/fleet/cmd_context.go` following the existing subcommand pattern.

**Tech Stack:** Go, Cobra CLI, JSON file persistence with flock

**Spec:** `docs/superpowers/specs/2026-04-04-agent-shared-log-design.md`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/context/context.go` | Modify | Add `LogEntry`, `Channel` types; add `AppendLog`, `TrimLog`, `CreateChannel`, `SendToChannel`, `TrimChannel` functions |
| `internal/context/context_test.go` | Modify | Tests for all new functions |
| `cmd/fleet/cmd_context.go` | Modify | Add `log`, `trim`, `channel-create`, `channel-send`, `channel-read`, `channel-list` subcommands; update `read` to show log |

---

### Task 1: Add LogEntry type and AppendLog function

**Files:**
- Modify: `internal/context/context.go`
- Modify: `internal/context/context_test.go`

- [ ] **Step 1: Write the failing tests for AppendLog**

Add to `internal/context/context_test.go`:

```go
func TestAppendLog(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.AppendLog(dir, "auth-agent", "found auth bug"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(ctx.Log))
	}
	if ctx.Log[0].Agent != "auth-agent" {
		t.Errorf("agent mismatch: got %q", ctx.Log[0].Agent)
	}
	if ctx.Log[0].Message != "found auth bug" {
		t.Errorf("message mismatch: got %q", ctx.Log[0].Message)
	}
	if ctx.Log[0].Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestAppendLogPreservesExistingData(t *testing.T) {
	dir := t.TempDir()

	// Write agent context first
	if err := fleetctx.WriteAgent(dir, "auth-agent", "working on auth"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Append to log — should not clobber agent data
	if err := fleetctx.AppendLog(dir, "api-agent", "endpoints ready"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "working on auth" {
		t.Errorf("agent data clobbered: got %q", ctx.Agents["auth-agent"])
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(ctx.Log))
	}
}

func TestAppendLogMultipleEntries(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.AppendLog(dir, "agent-a", "first"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}
	if err := fleetctx.AppendLog(dir, "agent-b", "second"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(ctx.Log))
	}
	if ctx.Log[0].Message != "first" {
		t.Errorf("first entry: got %q", ctx.Log[0].Message)
	}
	if ctx.Log[1].Message != "second" {
		t.Errorf("second entry: got %q", ctx.Log[1].Message)
	}
}

func TestAppendLogEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	err := fleetctx.AppendLog(dir, "agent-a", "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/context/... -run "TestAppendLog" -v`
Expected: FAIL — `AppendLog` and `LogEntry` don't exist yet.

- [ ] **Step 3: Add LogEntry type and AppendLog function**

In `internal/context/context.go`, add the `time` import, the `LogEntry` struct, update `Context`, and add `AppendLog`:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// LogEntry is a single attributed entry in the shared agent log.
type LogEntry struct {
	Agent     string    `json:"agent"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// Context is the shared context store for fleet agents.
type Context struct {
	Shared   string              `json:"shared"`
	Agents   map[string]string   `json:"agents"`
	Log      []LogEntry          `json:"log,omitempty"`
	Channels map[string]*Channel `json:"channels,omitempty"`
}

// Channel is a private named space where a fixed set of agents can communicate.
type Channel struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Members     []string   `json:"members"`
	Log         []LogEntry `json:"log,omitempty"`
}
```

Add the `AppendLog` function after `WriteShared`:

```go
// AppendLog adds an attributed entry to the shared agent log under lock.
func AppendLog(fleetDir, agentName, message string) error {
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	ctx.Log = append(ctx.Log, LogEntry{
		Agent:     agentName,
		Timestamp: time.Now().UTC(),
		Message:   message,
	})
	return saveUnlocked(fleetDir, ctx)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/context/... -run "TestAppendLog" -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/context/context.go internal/context/context_test.go
git commit -m "feat(context): add LogEntry type and AppendLog function"
```

---

### Task 2: Add TrimLog function

**Files:**
- Modify: `internal/context/context.go`
- Modify: `internal/context/context_test.go`

- [ ] **Step 1: Write the failing tests for TrimLog**

Add to `internal/context/context_test.go`:

```go
func TestTrimLog(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 10; i++ {
		if err := fleetctx.AppendLog(dir, "agent", fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("AppendLog failed: %v", err)
		}
	}

	if err := fleetctx.TrimLog(dir, 3); err != nil {
		t.Fatalf("TrimLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(ctx.Log))
	}
	// Should keep the last 3
	if ctx.Log[0].Message != "msg-7" {
		t.Errorf("first kept: got %q", ctx.Log[0].Message)
	}
	if ctx.Log[2].Message != "msg-9" {
		t.Errorf("last kept: got %q", ctx.Log[2].Message)
	}
}

func TestTrimLogNoOp(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.AppendLog(dir, "agent", "only one"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	if err := fleetctx.TrimLog(dir, 500); err != nil {
		t.Fatalf("TrimLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 entry (no-op), got %d", len(ctx.Log))
	}
}

func TestTrimLogClearAll(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 5; i++ {
		if err := fleetctx.AppendLog(dir, "agent", fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("AppendLog failed: %v", err)
		}
	}

	if err := fleetctx.TrimLog(dir, 0); err != nil {
		t.Fatalf("TrimLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(ctx.Log))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/context/... -run "TestTrimLog" -v`
Expected: FAIL — `TrimLog` doesn't exist yet.

- [ ] **Step 3: Implement TrimLog**

Add to `internal/context/context.go` after `AppendLog`:

```go
// TrimLog retains only the last `keep` entries in the shared log.
// No-op if the log already has keep or fewer entries.
// Pass keep=0 to clear the log entirely.
func TrimLog(fleetDir string, keep int) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	if len(ctx.Log) <= keep {
		return nil
	}
	ctx.Log = ctx.Log[len(ctx.Log)-keep:]
	return saveUnlocked(fleetDir, ctx)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/context/... -run "TestTrimLog" -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/context/context.go internal/context/context_test.go
git commit -m "feat(context): add TrimLog function"
```

---

### Task 3: Add `fleet context log`, `fleet context trim`, and update `fleet context read`

**Files:**
- Modify: `cmd/fleet/cmd_context.go`
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Add the `contextLogCmd` command**

Add to `cmd/fleet/cmd_context.go`:

```go
var contextLogCmd = &cobra.Command{
	Use:   "log [message]",
	Short: "Append a message to the shared agent log",
	Long:  "Adds an attributed entry to the shared agent log. Must be run from within a fleet agent session (FLEET_AGENT_NAME must be set).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if agentName == "" {
			return fmt.Errorf("must be run from within a fleet agent session (FLEET_AGENT_NAME not set)")
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		if err := fleetctx.AppendLog(f.FleetDir, agentName, args[0]); err != nil {
			return fmt.Errorf("failed to append log: %w", err)
		}
		fmt.Printf("Logged by '%s'\n", agentName)
		return nil
	},
}
```

- [ ] **Step 2: Add the `contextTrimCmd` command**

Add to `cmd/fleet/cmd_context.go`:

```go
var contextTrimCmd = &cobra.Command{
	Use:   "trim",
	Short: "Trim the shared log or a channel log to the most recent entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		keep, _ := cmd.Flags().GetInt("keep")
		channelName, _ := cmd.Flags().GetString("channel")

		if channelName != "" {
			if err := fleetctx.TrimChannel(f.FleetDir, channelName, keep); err != nil {
				return fmt.Errorf("failed to trim channel: %w", err)
			}
			ctx, err := fleetctx.Load(f.FleetDir)
			if err != nil {
				return fmt.Errorf("failed to load context: %w", err)
			}
			ch := ctx.Channels[channelName]
			fmt.Printf("Channel '%s' trimmed: %d entries remain\n", channelName, len(ch.Log))
			return nil
		}

		// Trim shared log
		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}
		before := len(ctx.Log)
		if before <= keep {
			fmt.Println("Log already within limit")
			return nil
		}

		if err := fleetctx.TrimLog(f.FleetDir, keep); err != nil {
			return fmt.Errorf("failed to trim log: %w", err)
		}
		after := keep
		if before < keep {
			after = before
		}
		fmt.Printf("Log trimmed: %d entries remain\n", after)
		return nil
	},
}
```

- [ ] **Step 3: Update `contextReadCmd` to show the log section**

In `cmd/fleet/cmd_context.go`, add a `"time"` import and append the following to the end of the "Full dump" section in `contextReadCmd.RunE`, right before the final `return nil`:

```go
		// Agent Log section
		if len(ctx.Log) > 0 {
			fmt.Println("== Agent Log ==")
			for _, entry := range ctx.Log {
				fmt.Printf("[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Agent, entry.Message)
			}
			fmt.Println()
		}
```

- [ ] **Step 4: Wire up new commands in `cmd/fleet/main.go`**

In the `init()` function, add after the existing `contextCmd.AddCommand(contextSetSharedCmd)` line:

```go
	contextCmd.AddCommand(contextLogCmd)
	contextTrimCmd.Flags().Int("keep", 500, "Number of entries to keep")
	contextTrimCmd.Flags().String("channel", "", "Trim a specific channel's log instead of the shared log")
	contextCmd.AddCommand(contextTrimCmd)
```

- [ ] **Step 5: Run all tests to make sure nothing is broken**

Run: `go test ./... 2>&1 | tail -20`
Expected: All existing tests PASS. (The `TrimChannel` function doesn't exist yet, so `go build` may fail — if so, add a stub for `TrimChannel` that returns `fmt.Errorf("not implemented")` temporarily.)

Note: If `TrimChannel` is referenced but not yet defined, add this stub to `internal/context/context.go` to let it compile:

```go
// TrimChannel retains only the last `keep` entries in the named channel's log.
func TrimChannel(fleetDir, channelName string, keep int) error {
	return fmt.Errorf("not implemented")
}
```

- [ ] **Step 6: Commit**

```bash
git add cmd/fleet/cmd_context.go cmd/fleet/main.go internal/context/context.go
git commit -m "feat(cli): add fleet context log, trim commands; show log in read"
```

---

### Task 4: Add CreateChannel function

**Files:**
- Modify: `internal/context/context.go`
- Modify: `internal/context/context_test.go`

- [ ] **Step 1: Write the failing tests for CreateChannel**

Add to `internal/context/context_test.go`:

```go
func TestCreateChannelDM(t *testing.T) {
	dir := t.TempDir()

	name, err := fleetctx.CreateChannel(dir, "ignored", "auth discussion", []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if name != "dm-[alice]-[bob]" {
		t.Errorf("expected dm-[alice]-[bob], got %q", name)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch, ok := ctx.Channels["dm-[alice]-[bob]"]
	if !ok {
		t.Fatal("channel not found in context")
	}
	if ch.Description != "auth discussion" {
		t.Errorf("description: got %q", ch.Description)
	}
	if len(ch.Members) != 2 || ch.Members[0] != "alice" || ch.Members[1] != "bob" {
		t.Errorf("members: got %v", ch.Members)
	}
}

func TestCreateChannelGroup(t *testing.T) {
	dir := t.TempDir()

	name, err := fleetctx.CreateChannel(dir, "backend-crew", "backend sync", []string{"alice", "bob", "charlie"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if name != "backend-crew" {
		t.Errorf("expected backend-crew, got %q", name)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch, ok := ctx.Channels["backend-crew"]
	if !ok {
		t.Fatal("channel not found")
	}
	if len(ch.Members) != 3 {
		t.Errorf("expected 3 members, got %d", len(ch.Members))
	}
}

func TestCreateChannelDuplicate(t *testing.T) {
	dir := t.TempDir()

	_, err := fleetctx.CreateChannel(dir, "backend-crew", "first", []string{"alice", "bob", "charlie"})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = fleetctx.CreateChannel(dir, "backend-crew", "second", []string{"alice", "bob", "charlie"})
	if err == nil {
		t.Fatal("expected error for duplicate channel, got nil")
	}
}

func TestCreateChannelTooFewMembers(t *testing.T) {
	dir := t.TempDir()
	_, err := fleetctx.CreateChannel(dir, "solo", "alone", []string{"alice"})
	if err == nil {
		t.Fatal("expected error for < 2 members, got nil")
	}
}

func TestCreateChannelEmptyMember(t *testing.T) {
	dir := t.TempDir()
	_, err := fleetctx.CreateChannel(dir, "bad", "empty member", []string{"alice", ""})
	if err == nil {
		t.Fatal("expected error for empty member name, got nil")
	}
}

func TestCreateChannelPreservesExistingData(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.WriteShared(dir, "shared stuff"); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}
	if err := fleetctx.AppendLog(dir, "agent", "log entry"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	_, err := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Shared != "shared stuff" {
		t.Errorf("shared clobbered: got %q", ctx.Shared)
	}
	if len(ctx.Log) != 1 {
		t.Errorf("log clobbered: got %d entries", len(ctx.Log))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/context/... -run "TestCreateChannel" -v`
Expected: FAIL — `CreateChannel` doesn't exist yet.

- [ ] **Step 3: Implement CreateChannel**

In `internal/context/context.go`, add:

```go
// CreateChannel creates a new named channel with fixed membership.
// For 2-member channels, the name is auto-set to dm-[member1]-[member2] and
// the provided name is ignored. Returns the resolved channel name.
func CreateChannel(fleetDir, name, description string, members []string) (string, error) {
	if len(members) < 2 {
		return "", fmt.Errorf("channel requires at least 2 members")
	}
	for _, m := range members {
		if m == "" {
			return "", fmt.Errorf("member name cannot be empty")
		}
	}

	// Auto-name DM channels
	if len(members) == 2 {
		name = fmt.Sprintf("dm-[%s]-[%s]", members[0], members[1])
	}

	lf, err := acquireLock(fleetDir)
	if err != nil {
		return "", err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return "", err
	}
	if ctx.Channels == nil {
		ctx.Channels = map[string]*Channel{}
	}
	if _, exists := ctx.Channels[name]; exists {
		return "", fmt.Errorf("channel already exists: %s", name)
	}

	ctx.Channels[name] = &Channel{
		Name:        name,
		Description: description,
		Members:     members,
	}
	if err := saveUnlocked(fleetDir, ctx); err != nil {
		return "", err
	}
	return name, nil
}
```

Also update `loadUnlocked` to initialize `Channels` if nil:

```go
	if ctx.Channels == nil {
		ctx.Channels = map[string]*Channel{}
	}
```

Add this after the existing `if ctx.Agents == nil` block.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/context/... -run "TestCreateChannel" -v`
Expected: All 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/context/context.go internal/context/context_test.go
git commit -m "feat(context): add CreateChannel with DM auto-naming"
```

---

### Task 5: Add SendToChannel function

**Files:**
- Modify: `internal/context/context.go`
- Modify: `internal/context/context_test.go`

- [ ] **Step 1: Write the failing tests for SendToChannel**

Add to `internal/context/context_test.go`:

```go
func TestSendToChannel(t *testing.T) {
	dir := t.TempDir()

	name, err := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}

	if err := fleetctx.SendToChannel(dir, name, "alice", "hey bob"); err != nil {
		t.Fatalf("SendToChannel failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch := ctx.Channels[name]
	if len(ch.Log) != 1 {
		t.Fatalf("expected 1 message, got %d", len(ch.Log))
	}
	if ch.Log[0].Agent != "alice" {
		t.Errorf("agent: got %q", ch.Log[0].Agent)
	}
	if ch.Log[0].Message != "hey bob" {
		t.Errorf("message: got %q", ch.Log[0].Message)
	}
}

func TestSendToChannelNonMember(t *testing.T) {
	dir := t.TempDir()

	name, _ := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})

	err := fleetctx.SendToChannel(dir, name, "charlie", "let me in")
	if err == nil {
		t.Fatal("expected error for non-member, got nil")
	}
}

func TestSendToChannelNotExists(t *testing.T) {
	dir := t.TempDir()
	err := fleetctx.SendToChannel(dir, "no-such-channel", "alice", "hello")
	if err == nil {
		t.Fatal("expected error for missing channel, got nil")
	}
}

func TestSendToChannelEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	name, _ := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})

	err := fleetctx.SendToChannel(dir, name, "alice", "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/context/... -run "TestSendToChannel" -v`
Expected: FAIL — `SendToChannel` doesn't exist yet.

- [ ] **Step 3: Implement SendToChannel**

Add to `internal/context/context.go`:

```go
// SendToChannel appends a message to a channel's log. The sender must be a member.
func SendToChannel(fleetDir, channelName, agentName, message string) error {
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	ch, ok := ctx.Channels[channelName]
	if !ok {
		return fmt.Errorf("channel not found: %s", channelName)
	}

	isMember := false
	for _, m := range ch.Members {
		if m == agentName {
			isMember = true
			break
		}
	}
	if !isMember {
		return fmt.Errorf("agent is not a member of this channel: %s", agentName)
	}

	ch.Log = append(ch.Log, LogEntry{
		Agent:     agentName,
		Timestamp: time.Now().UTC(),
		Message:   message,
	})
	return saveUnlocked(fleetDir, ctx)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/context/... -run "TestSendToChannel" -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/context/context.go internal/context/context_test.go
git commit -m "feat(context): add SendToChannel with membership check"
```

---

### Task 6: Implement TrimChannel (replace stub)

**Files:**
- Modify: `internal/context/context.go`
- Modify: `internal/context/context_test.go`

- [ ] **Step 1: Write the failing tests for TrimChannel**

Add to `internal/context/context_test.go`:

```go
func TestTrimChannel(t *testing.T) {
	dir := t.TempDir()

	name, _ := fleetctx.CreateChannel(dir, "ignored", "dm", []string{"alice", "bob"})
	for i := 0; i < 10; i++ {
		if err := fleetctx.SendToChannel(dir, name, "alice", fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("SendToChannel failed: %v", err)
		}
	}

	if err := fleetctx.TrimChannel(dir, name, 3); err != nil {
		t.Fatalf("TrimChannel failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ch := ctx.Channels[name]
	if len(ch.Log) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(ch.Log))
	}
	if ch.Log[0].Message != "msg-7" {
		t.Errorf("first kept: got %q", ch.Log[0].Message)
	}
}

func TestTrimChannelNotExists(t *testing.T) {
	dir := t.TempDir()
	err := fleetctx.TrimChannel(dir, "no-such-channel", 10)
	if err == nil {
		t.Fatal("expected error for missing channel, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/context/... -run "TestTrimChannel" -v`
Expected: FAIL — `TrimChannel` is a stub returning "not implemented".

- [ ] **Step 3: Replace the TrimChannel stub with the real implementation**

Replace the stub in `internal/context/context.go`:

```go
// TrimChannel retains only the last `keep` entries in the named channel's log.
func TrimChannel(fleetDir, channelName string, keep int) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	ch, ok := ctx.Channels[channelName]
	if !ok {
		return fmt.Errorf("channel not found: %s", channelName)
	}
	if len(ch.Log) <= keep {
		return nil
	}
	ch.Log = ch.Log[len(ch.Log)-keep:]
	return saveUnlocked(fleetDir, ctx)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/context/... -run "TestTrimChannel" -v`
Expected: Both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/context/context.go internal/context/context_test.go
git commit -m "feat(context): implement TrimChannel"
```

---

### Task 7: Add channel CLI commands

**Files:**
- Modify: `cmd/fleet/cmd_context.go`
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Add `channelCreateCmd`**

Add to `cmd/fleet/cmd_context.go`:

```go
var channelCreateCmd = &cobra.Command{
	Use:   "channel-create [name] [agent1] [agent2] [agent3...]",
	Short: "Create a private channel between agents",
	Long:  "Creates a named channel with fixed membership. For 2-member channels, the name is auto-set to dm-[agent1]-[agent2].",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		name := args[0]
		members := args[1:]
		description, _ := cmd.Flags().GetString("description")

		resolved, err := fleetctx.CreateChannel(f.FleetDir, name, description, members)
		if err != nil {
			return fmt.Errorf("failed to create channel: %w", err)
		}
		fmt.Printf("Channel created: %s (members: %v)\n", resolved, members)
		return nil
	},
}
```

- [ ] **Step 2: Add `channelSendCmd`**

Add to `cmd/fleet/cmd_context.go`:

```go
var channelSendCmd = &cobra.Command{
	Use:   "channel-send [channel-name] [message]",
	Short: "Send a message to a channel",
	Long:  "Appends a message to the channel's log. Must be run from within a fleet agent session and the sender must be a channel member.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if agentName == "" {
			return fmt.Errorf("must be run from within a fleet agent session (FLEET_AGENT_NAME not set)")
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		if err := fleetctx.SendToChannel(f.FleetDir, args[0], agentName, args[1]); err != nil {
			return fmt.Errorf("failed to send to channel: %w", err)
		}
		fmt.Printf("Sent to '%s' as '%s'\n", args[0], agentName)
		return nil
	},
}
```

- [ ] **Step 3: Add `channelReadCmd`**

Add to `cmd/fleet/cmd_context.go` (add `"time"` to imports if not already present):

```go
var channelReadCmd = &cobra.Command{
	Use:   "channel-read [channel-name]",
	Short: "Read a channel's messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}

		ch, ok := ctx.Channels[args[0]]
		if !ok {
			return fmt.Errorf("channel not found: %s", args[0])
		}

		fmt.Printf("Channel: %s\n", ch.Name)
		if ch.Description != "" {
			fmt.Printf("Description: %s\n", ch.Description)
		}
		fmt.Printf("Members: %v\n", ch.Members)
		fmt.Println()

		if len(ch.Log) == 0 {
			fmt.Println("(no messages)")
		} else {
			for _, entry := range ch.Log {
				fmt.Printf("[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Agent, entry.Message)
			}
		}
		return nil
	},
}
```

- [ ] **Step 4: Add `channelListCmd`**

Add to `cmd/fleet/cmd_context.go`:

```go
var channelListCmd = &cobra.Command{
	Use:   "channel-list",
	Short: "List all channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}

		if len(ctx.Channels) == 0 {
			fmt.Println("No channels")
			return nil
		}

		names := make([]string, 0, len(ctx.Channels))
		for name := range ctx.Channels {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			ch := ctx.Channels[name]
			desc := ch.Description
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Printf("%-30s  %d members  %d messages  %s\n", ch.Name, len(ch.Members), len(ch.Log), desc)
		}
		return nil
	},
}
```

- [ ] **Step 5: Wire up channel commands in `cmd/fleet/main.go`**

In the `init()` function, add after the existing context command wiring:

```go
	channelCreateCmd.Flags().String("description", "", "Channel description")
	contextCmd.AddCommand(channelCreateCmd)
	contextCmd.AddCommand(channelSendCmd)
	contextCmd.AddCommand(channelReadCmd)
	contextCmd.AddCommand(channelListCmd)
```

- [ ] **Step 6: Build and verify**

Run: `go build -o fleet ./cmd/fleet/`
Expected: Builds cleanly.

Run: `go test ./... 2>&1 | tail -20`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/fleet/cmd_context.go cmd/fleet/main.go
git commit -m "feat(cli): add channel-create, channel-send, channel-read, channel-list commands"
```

---

### Task 8: Final integration smoke test

**Files:**
- None created — manual verification only.

- [ ] **Step 1: Build the binary**

Run: `go build -o fleet ./cmd/fleet/`

- [ ] **Step 2: Run all tests**

Run: `go test ./... -v 2>&1 | tail -40`
Expected: All tests PASS.

- [ ] **Step 3: Verify CLI help text**

Run: `./fleet context --help`
Expected: Shows `log`, `trim`, `channel-create`, `channel-send`, `channel-read`, `channel-list` as subcommands alongside existing `read`, `write`, `set-shared`.

- [ ] **Step 4: Clean up binary**

Run: `rm -f fleet`

- [ ] **Step 5: Commit (if any cleanup was needed)**

Only if changes were made during smoke testing.
