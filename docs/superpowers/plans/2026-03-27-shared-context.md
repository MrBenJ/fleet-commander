# Shared Context System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a shared context store (`.fleet/context.json`) with CLI commands so fleet agents can publish and read context from each other, with enforced ownership and file locking.

**Architecture:** New `internal/context/` package handles the data model, JSON persistence, and flock-based locking (mirroring `internal/fleet/lock.go`). CLI subcommands under `fleet context` in `cmd/fleet/main.go` handle user interaction and ownership enforcement via `FLEET_AGENT_NAME` env var.

**Tech Stack:** Go, cobra CLI, syscall.Flock

---

### Task 1: Context Package — Data Model and Load/Save

**Files:**
- Create: `internal/context/context.go`
- Create: `internal/context/context_test.go`

- [ ] **Step 1: Write the failing test for Load on missing file**

```go
//go:build !windows

package context_test

import (
	"path/filepath"
	"testing"

	fleetctx "github.com/teknal/fleet-commander/internal/context"
)

func TestLoadMissingFileReturnsEmptyContext(t *testing.T) {
	dir := t.TempDir()
	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Shared != "" {
		t.Errorf("expected empty shared, got %q", ctx.Shared)
	}
	if len(ctx.Agents) != 0 {
		t.Errorf("expected empty agents map, got %v", ctx.Agents)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/context/ -run TestLoadMissingFileReturnsEmptyContext -v`
Expected: FAIL — package does not exist yet

- [ ] **Step 3: Write the Context struct and Load function**

Create `internal/context/context.go`:

```go
//go:build !windows

package context

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Context is the shared context store for fleet agents.
type Context struct {
	Shared string            `json:"shared"`
	Agents map[string]string `json:"agents"`
}

const contextFile = "context.json"
const lockFile = "context.lock"

// Load reads .fleet/context.json from fleetDir. Returns an empty Context if
// the file does not exist.
func Load(fleetDir string) (*Context, error) {
	path := filepath.Join(fleetDir, contextFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Context{Agents: map[string]string{}}, nil
		}
		return nil, fmt.Errorf("failed to read context: %w", err)
	}

	var ctx Context
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to parse context: %w", err)
	}
	if ctx.Agents == nil {
		ctx.Agents = map[string]string{}
	}
	return &ctx, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/context/ -run TestLoadMissingFileReturnsEmptyContext -v`
Expected: PASS

- [ ] **Step 5: Write the failing test for Save and round-trip**

Add to `internal/context/context_test.go`:

```go
func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	ctx := &fleetctx.Context{
		Shared: "use JWT auth",
		Agents: map[string]string{
			"auth-agent": "User model done",
		},
	}
	if err := fleetctx.Save(dir, ctx); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Shared != "use JWT auth" {
		t.Errorf("shared mismatch: got %q", loaded.Shared)
	}
	if loaded.Agents["auth-agent"] != "User model done" {
		t.Errorf("agent mismatch: got %q", loaded.Agents["auth-agent"])
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/context/ -run TestSaveAndLoad -v`
Expected: FAIL — Save not defined

- [ ] **Step 7: Write Save function with flock**

Add to `internal/context/context.go`:

```go
// Save writes the context to .fleet/context.json under an exclusive flock.
func Save(fleetDir string, ctx *Context) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}
	path := filepath.Join(fleetDir, contextFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write context: %w", err)
	}
	return nil
}

func acquireLock(fleetDir string) (*os.File, error) {
	lf, err := os.OpenFile(filepath.Join(fleetDir, lockFile), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open context lock: %w", err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		lf.Close()
		return nil, fmt.Errorf("failed to acquire context lock: %w", err)
	}
	return lf, nil
}

func releaseLock(lf *os.File) {
	syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck
	lf.Close()
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/context/ -run TestSaveAndLoad -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/context/context.go internal/context/context_test.go
git commit -m "feat: add internal/context package with Load/Save and flock locking"
```

---

### Task 2: Context Package — WriteAgent and WriteShared

**Files:**
- Modify: `internal/context/context.go`
- Modify: `internal/context/context_test.go`

- [ ] **Step 1: Write the failing test for WriteAgent**

Add to `internal/context/context_test.go`:

```go
func TestWriteAgent(t *testing.T) {
	dir := t.TempDir()

	// First agent writes
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model done"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Second agent writes — should not clobber first
	if err := fleetctx.WriteAgent(dir, "api-agent", "Endpoints defined"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "User model done" {
		t.Errorf("auth-agent mismatch: got %q", ctx.Agents["auth-agent"])
	}
	if ctx.Agents["api-agent"] != "Endpoints defined" {
		t.Errorf("api-agent mismatch: got %q", ctx.Agents["api-agent"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/context/ -run TestWriteAgent -v`
Expected: FAIL — WriteAgent not defined

- [ ] **Step 3: Write WriteAgent function**

Add to `internal/context/context.go`:

```go
// WriteAgent updates a single agent's section under lock. It reads the current
// context from disk, updates only the named agent's entry, and writes back.
func WriteAgent(fleetDir, agentName, message string) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	ctx.Agents[agentName] = message
	return saveUnlocked(fleetDir, ctx)
}
```

Also refactor Load/Save internals to separate locked vs unlocked variants:

```go
// loadUnlocked reads context.json without acquiring the lock.
// Only call from within a locked section.
func loadUnlocked(fleetDir string) (*Context, error) {
	path := filepath.Join(fleetDir, contextFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Context{Agents: map[string]string{}}, nil
		}
		return nil, fmt.Errorf("failed to read context: %w", err)
	}

	var ctx Context
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to parse context: %w", err)
	}
	if ctx.Agents == nil {
		ctx.Agents = map[string]string{}
	}
	return &ctx, nil
}

// saveUnlocked writes context.json without acquiring the lock.
// Only call from within a locked section.
func saveUnlocked(fleetDir string, ctx *Context) error {
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}
	path := filepath.Join(fleetDir, contextFile)
	return os.WriteFile(path, data, 0644)
}
```

Update `Load` to call `loadUnlocked` and `Save` to call `acquireLock` + `saveUnlocked`:

```go
func Load(fleetDir string) (*Context, error) {
	return loadUnlocked(fleetDir)
}

func Save(fleetDir string, ctx *Context) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)
	return saveUnlocked(fleetDir, ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/context/ -run TestWriteAgent -v`
Expected: PASS

- [ ] **Step 5: Write the failing test for WriteShared**

Add to `internal/context/context_test.go`:

```go
func TestWriteShared(t *testing.T) {
	dir := t.TempDir()

	// Write an agent first
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model done"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Write shared — should not clobber agent
	if err := fleetctx.WriteShared(dir, "API uses JWT"); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Shared != "API uses JWT" {
		t.Errorf("shared mismatch: got %q", ctx.Shared)
	}
	if ctx.Agents["auth-agent"] != "User model done" {
		t.Errorf("auth-agent was clobbered: got %q", ctx.Agents["auth-agent"])
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/context/ -run TestWriteShared -v`
Expected: FAIL — WriteShared not defined

- [ ] **Step 7: Write WriteShared function**

Add to `internal/context/context.go`:

```go
// WriteShared updates the shared section under lock. It reads the current
// context from disk, updates the shared field, and writes back.
func WriteShared(fleetDir, message string) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	ctx.Shared = message
	return saveUnlocked(fleetDir, ctx)
}
```

- [ ] **Step 8: Run all context tests**

Run: `go test ./internal/context/ -v`
Expected: All PASS

- [ ] **Step 9: Write failing test for Load on malformed JSON**

Add to `internal/context/context_test.go`:

```go
func TestLoadMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "context.json")
	os.WriteFile(path, []byte("{invalid json"), 0644)

	_, err := fleetctx.Load(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
```

- [ ] **Step 10: Run test to verify it passes (Load already returns parse errors)**

Run: `go test ./internal/context/ -run TestLoadMalformedJSON -v`
Expected: PASS

- [ ] **Step 11: Commit**

```bash
git add internal/context/context.go internal/context/context_test.go
git commit -m "feat: add WriteAgent and WriteShared with lock-protected read-modify-write"
```

---

### Task 3: CLI — `fleet context read`

**Files:**
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Add the context command group and read subcommand**

Add to `cmd/fleet/main.go` imports:

```go
fleetctx "github.com/teknal/fleet-commander/internal/context"
```

Add the commands:

```go
var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Read and write shared context between agents",
}

var contextReadCmd = &cobra.Command{
	Use:   "read [agent-name]",
	Short: "Read shared context (optionally for a specific agent)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		ctx, err := fleetctx.Load(f.FleetDir)
		if err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}

		sharedOnly, _ := cmd.Flags().GetBool("shared")

		// Specific agent requested
		if len(args) == 1 {
			if sharedOnly {
				return fmt.Errorf("cannot use --shared with an agent name")
			}
			fmt.Print(ctx.Agents[args[0]])
			if ctx.Agents[args[0]] != "" {
				fmt.Println()
			}
			return nil
		}

		// Shared only
		if sharedOnly {
			fmt.Print(ctx.Shared)
			if ctx.Shared != "" {
				fmt.Println()
			}
			return nil
		}

		// Full dump
		if ctx.Shared != "" {
			fmt.Println("== Shared Context ==")
			fmt.Println(ctx.Shared)
			fmt.Println()
		}
		for name, text := range ctx.Agents {
			fmt.Printf("== %s ==\n", name)
			fmt.Println(text)
			fmt.Println()
		}
		return nil
	},
}
```

- [ ] **Step 2: Register commands in init()**

Add to `init()`:

```go
contextReadCmd.Flags().Bool("shared", false, "Read only the shared context section")
contextCmd.AddCommand(contextReadCmd)
rootCmd.AddCommand(contextCmd)
```

- [ ] **Step 3: Build and smoke test**

Run: `go build -o fleet ./cmd/fleet/ && ./fleet context read`
Expected: No output, exit 0 (no context.json yet)

- [ ] **Step 4: Commit**

```bash
git add cmd/fleet/main.go
git commit -m "feat: add 'fleet context read' command"
```

---

### Task 4: CLI — `fleet context write`

**Files:**
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Add the write subcommand**

Add to `cmd/fleet/main.go`:

```go
var contextWriteCmd = &cobra.Command{
	Use:   "write [message]",
	Short: "Write to your agent's context section",
	Long:  "Updates this agent's section in shared context. Must be run from within a fleet agent session (FLEET_AGENT_NAME must be set).",
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

		if err := fleetctx.WriteAgent(f.FleetDir, agentName, args[0]); err != nil {
			return fmt.Errorf("failed to write context: %w", err)
		}
		fmt.Printf("Updated context for agent '%s'\n", agentName)
		return nil
	},
}
```

- [ ] **Step 2: Register in init()**

Add to `init()`:

```go
contextCmd.AddCommand(contextWriteCmd)
```

- [ ] **Step 3: Build and smoke test**

Run: `go build -o fleet ./cmd/fleet/ && ./fleet context write "test"`
Expected: Error: "must be run from within a fleet agent session"

Run: `FLEET_AGENT_NAME=test-agent ./fleet context write "hello world" && ./fleet context read`
Expected: Shows `== test-agent ==` section with "hello world"

- [ ] **Step 4: Commit**

```bash
git add cmd/fleet/main.go
git commit -m "feat: add 'fleet context write' command with agent ownership enforcement"
```

---

### Task 5: CLI — `fleet context set-shared`

**Files:**
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Add the set-shared subcommand**

Add to `cmd/fleet/main.go`:

```go
var contextSetSharedCmd = &cobra.Command{
	Use:   "set-shared [message]",
	Short: "Set the shared context section",
	Long:  "Updates the shared context section. Cannot be run from within a fleet agent session.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := os.Getenv("FLEET_AGENT_NAME")
		if agentName != "" {
			return fmt.Errorf("shared context can only be set from outside agent sessions (FLEET_AGENT_NAME is set to '%s')", agentName)
		}

		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		if err := fleetctx.WriteShared(f.FleetDir, args[0]); err != nil {
			return fmt.Errorf("failed to write shared context: %w", err)
		}
		fmt.Println("Updated shared context")
		return nil
	},
}
```

- [ ] **Step 2: Register in init()**

Add to `init()`:

```go
contextCmd.AddCommand(contextSetSharedCmd)
```

- [ ] **Step 3: Build and smoke test**

Run: `go build -o fleet ./cmd/fleet/ && ./fleet context set-shared "API uses JWT. Base path /v2." && ./fleet context read`
Expected: Shows shared context section

Run: `FLEET_AGENT_NAME=test-agent ./fleet context set-shared "nope"`
Expected: Error: "shared context can only be set from outside agent sessions"

- [ ] **Step 4: Commit**

```bash
git add cmd/fleet/main.go
git commit -m "feat: add 'fleet context set-shared' command with ownership enforcement"
```

---

### Task 6: Full Integration Test

**Files:**
- Modify: `internal/context/context_test.go`

- [ ] **Step 1: Write an end-to-end test simulating multiple agents**

Add to `internal/context/context_test.go`:

```go
func TestMultiAgentWorkflow(t *testing.T) {
	dir := t.TempDir()

	// User sets shared context
	if err := fleetctx.WriteShared(dir, "API uses JWT. Base path /v2."); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}

	// auth-agent writes its section
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model at internal/auth/user.go. @api-agent merge fleet/auth"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// api-agent writes its section
	if err := fleetctx.WriteAgent(dir, "api-agent", "Endpoints defined. Waiting on auth model."); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Read full context
	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if ctx.Shared != "API uses JWT. Base path /v2." {
		t.Errorf("shared: got %q", ctx.Shared)
	}
	if ctx.Agents["auth-agent"] != "User model at internal/auth/user.go. @api-agent merge fleet/auth" {
		t.Errorf("auth-agent: got %q", ctx.Agents["auth-agent"])
	}
	if ctx.Agents["api-agent"] != "Endpoints defined. Waiting on auth model." {
		t.Errorf("api-agent: got %q", ctx.Agents["api-agent"])
	}

	// auth-agent overwrites its section
	if err := fleetctx.WriteAgent(dir, "auth-agent", "Auth complete. All tests passing."); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	ctx, err = fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "Auth complete. All tests passing." {
		t.Errorf("auth-agent after overwrite: got %q", ctx.Agents["auth-agent"])
	}
	// api-agent should be untouched
	if ctx.Agents["api-agent"] != "Endpoints defined. Waiting on auth model." {
		t.Errorf("api-agent was clobbered: got %q", ctx.Agents["api-agent"])
	}
}
```

- [ ] **Step 2: Run all context tests**

Run: `go test ./internal/context/ -v`
Expected: All PASS

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/context/context_test.go
git commit -m "test: add multi-agent workflow integration test for shared context"
```

---

### Task 7: Final Verification

- [ ] **Step 1: Build the binary**

Run: `go build -o fleet ./cmd/fleet/`
Expected: Clean build, no errors

- [ ] **Step 2: Verify all commands show in help**

Run: `./fleet context --help`
Expected: Shows `read`, `write`, `set-shared` subcommands

- [ ] **Step 3: Run full test suite one more time**

Run: `go test ./...`
Expected: All PASS
