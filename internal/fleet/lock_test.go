//go:build !windows

package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func setupLockTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		t.Fatalf("failed to create fleet dir: %v", err)
	}
	return fleetDir
}

func TestAcquireReleaseLock(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	lf, err := acquireLock(fleetDir)
	if err != nil {
		t.Fatalf("acquireLock failed: %v", err)
	}
	if lf == nil {
		t.Fatal("expected non-nil lock file")
	}

	// Lock file should exist on disk
	lockPath := filepath.Join(fleetDir, "config.lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file not created on disk")
	}

	releaseLock(lf)
}

func TestAcquireLockExclusive(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	// Acquire the first lock
	lf1, err := acquireLock(fleetDir)
	if err != nil {
		t.Fatalf("first acquireLock failed: %v", err)
	}
	defer releaseLock(lf1)

	// Try to acquire a second lock — should timeout since first is held
	// Use a goroutine with a short timeout to avoid blocking the test
	done := make(chan error, 1)
	go func() {
		// Override timeout by just trying once with a short deadline
		lf2, err := acquireLock(fleetDir)
		if err != nil {
			done <- err
			return
		}
		releaseLock(lf2)
		done <- nil
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("second lock should have timed out while first is held")
		}
		// Expected: timeout error
	case <-time.After(10 * time.Second):
		t.Fatal("test timed out waiting for second lock attempt")
	}
}

func TestAcquireLockSucceedsAfterRelease(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	lf1, err := acquireLock(fleetDir)
	if err != nil {
		t.Fatalf("first acquireLock failed: %v", err)
	}
	releaseLock(lf1)

	// Should succeed now that first lock is released
	lf2, err := acquireLock(fleetDir)
	if err != nil {
		t.Fatalf("second acquireLock should succeed after release: %v", err)
	}
	releaseLock(lf2)
}

func TestForceUnlock(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	lockPath := filepath.Join(fleetDir, "config.lock")

	// Create a lock file manually
	if err := os.WriteFile(lockPath, []byte(""), 0600); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	if err := ForceUnlock(fleetDir); err != nil {
		t.Fatalf("ForceUnlock failed: %v", err)
	}

	// Lock file should be gone
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file still exists after ForceUnlock")
	}
}

func TestForceUnlockNoFileIsNoOp(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	// No lock file exists — should not error
	if err := ForceUnlock(fleetDir); err != nil {
		t.Fatalf("ForceUnlock on nonexistent file should not error: %v", err)
	}
}

func TestReadConfig(t *testing.T) {
	fleetDir := setupLockTestDir(t)
	configPath := filepath.Join(fleetDir, "config.json")

	expected := &Fleet{
		RepoPath: "/tmp/repo",
		FleetDir: fleetDir,
		Agents: []*Agent{
			{Name: "alpha", Branch: "fleet/alpha", Status: "ready"},
		},
	}
	data, _ := json.MarshalIndent(expected, "", "  ")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	got, err := readConfig(configPath)
	if err != nil {
		t.Fatalf("readConfig failed: %v", err)
	}
	if got.RepoPath != expected.RepoPath {
		t.Errorf("RepoPath = %q, want %q", got.RepoPath, expected.RepoPath)
	}
	if len(got.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(got.Agents))
	}
	if got.Agents[0].Name != "alpha" {
		t.Errorf("agent name = %q, want %q", got.Agents[0].Name, "alpha")
	}
}

func TestReadConfigMissingFile(t *testing.T) {
	_, err := readConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestReadConfigInvalidJSON(t *testing.T) {
	fleetDir := setupLockTestDir(t)
	configPath := filepath.Join(fleetDir, "config.json")

	if err := os.WriteFile(configPath, []byte("{invalid json!"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := readConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestWriteConfig(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	f := &Fleet{
		RepoPath: "/tmp/repo",
		FleetDir: fleetDir,
		Agents: []*Agent{
			{Name: "bravo", Branch: "fleet/bravo", Status: "running"},
		},
	}

	if err := f.writeConfig(); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	// Read it back
	configPath := filepath.Join(fleetDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var got Fleet
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to parse written config: %v", err)
	}
	if len(got.Agents) != 1 || got.Agents[0].Name != "bravo" {
		t.Errorf("written config doesn't match: %+v", got)
	}
}

func TestWithLockReReadsFromDisk(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	// Write initial config with one agent
	initial := &Fleet{
		RepoPath: "/tmp/repo",
		FleetDir: fleetDir,
		Agents: []*Agent{
			{Name: "existing", Branch: "fleet/existing", Status: "ready"},
		},
	}
	if err := initial.writeConfig(); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	// Create a Fleet struct with stale state (no agents)
	stale := &Fleet{
		RepoPath: "/tmp/repo",
		FleetDir: fleetDir,
		Agents:   []*Agent{},
	}

	// withLock should re-read from disk, so fn should see the existing agent
	var agentCountInsideLock int
	err := stale.withLock(func() error {
		agentCountInsideLock = len(stale.Agents)
		stale.Agents = append(stale.Agents, &Agent{
			Name:   "new-agent",
			Branch: "fleet/new-agent",
			Status: "ready",
		})
		return nil
	})
	if err != nil {
		t.Fatalf("withLock failed: %v", err)
	}

	if agentCountInsideLock != 1 {
		t.Errorf("expected 1 agent inside lock (re-read from disk), got %d", agentCountInsideLock)
	}

	// Verify the written config has both agents
	configPath := filepath.Join(fleetDir, "config.json")
	got, err := readConfig(configPath)
	if err != nil {
		t.Fatalf("readConfig failed: %v", err)
	}
	if len(got.Agents) != 2 {
		t.Errorf("expected 2 agents after withLock, got %d", len(got.Agents))
	}
}

func TestWithLockPreservesFleetDir(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	f := &Fleet{
		RepoPath: "/tmp/repo",
		FleetDir: fleetDir,
		Agents:   []*Agent{},
	}
	if err := f.writeConfig(); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	err := f.withLock(func() error {
		// FleetDir should be preserved even though it's re-read from disk
		if f.FleetDir != fleetDir {
			return fmt.Errorf("FleetDir changed inside lock: got %q, want %q", f.FleetDir, fleetDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withLock failed: %v", err)
	}
}

func TestWithLockConcurrentAccess(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	f := &Fleet{
		RepoPath: "/tmp/repo",
		FleetDir: fleetDir,
		Agents:   []*Agent{},
	}
	if err := f.writeConfig(); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	// Run 10 goroutines that each add an agent under withLock.
	// If locking works correctly, we should end up with exactly 10 agents.
	const goroutines = 10
	var wg sync.WaitGroup
	var errCount atomic.Int32

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			localFleet := &Fleet{
				RepoPath: "/tmp/repo",
				FleetDir: fleetDir,
				Agents:   []*Agent{},
			}
			err := localFleet.withLock(func() error {
				name := fmt.Sprintf("agent-%d", idx)
				localFleet.Agents = append(localFleet.Agents, &Agent{
					Name:   name,
					Branch: "fleet/" + name,
					Status: "ready",
				})
				return nil
			})
			if err != nil {
				errCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	if errCount.Load() > 0 {
		t.Errorf("%d goroutines hit errors during concurrent withLock", errCount.Load())
	}

	// Read final state — should have exactly 10 agents
	configPath := filepath.Join(fleetDir, "config.json")
	got, err := readConfig(configPath)
	if err != nil {
		t.Fatalf("readConfig failed: %v", err)
	}
	if len(got.Agents) != goroutines {
		t.Errorf("expected %d agents after concurrent writes, got %d", goroutines, len(got.Agents))
	}
}

func TestWithLockRollbackOnError(t *testing.T) {
	fleetDir := setupLockTestDir(t)

	f := &Fleet{
		RepoPath: "/tmp/repo",
		FleetDir: fleetDir,
		Agents: []*Agent{
			{Name: "keeper", Branch: "fleet/keeper", Status: "ready"},
		},
	}
	if err := f.writeConfig(); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	// withLock fn returns error — config should NOT be updated
	err := f.withLock(func() error {
		f.Agents = append(f.Agents, &Agent{Name: "should-not-persist"})
		return fmt.Errorf("intentional error")
	})
	if err == nil {
		t.Fatal("expected error from withLock")
	}

	// Config on disk should still have just the original agent
	configPath := filepath.Join(fleetDir, "config.json")
	got, err := readConfig(configPath)
	if err != nil {
		t.Fatalf("readConfig failed: %v", err)
	}
	if len(got.Agents) != 1 || got.Agents[0].Name != "keeper" {
		t.Errorf("config should be unchanged after error, got %d agents", len(got.Agents))
	}
}
