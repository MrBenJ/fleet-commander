//go:build !windows

package global

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestHome creates a temporary directory, sets HOME to it, and returns
// a cleanup function that restores the original HOME.
func setupTestHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmp)
	_ = origHome
	return tmp
}

// --- Index / Register tests ---

func TestRegisterDefaultsShortName(t *testing.T) {
	home := setupTestHome(t)
	repoPath := filepath.Join(home, "projects", "my-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	name, err := Register(repoPath, "")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if name != "my-repo" {
		t.Errorf("expected short name 'my-repo', got %q", name)
	}

	entry, err := Lookup(repoPath)
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if entry.ShortName != "my-repo" {
		t.Errorf("expected short name 'my-repo', got %q", entry.ShortName)
	}
}

func TestRegisterCustomShortName(t *testing.T) {
	home := setupTestHome(t)
	repoPath := filepath.Join(home, "projects", "my-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	name, err := Register(repoPath, "custom")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if name != "custom" {
		t.Errorf("expected 'custom', got %q", name)
	}

	entry, err := Lookup("custom")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if entry.ShortName != "custom" {
		t.Errorf("expected short name 'custom', got %q", entry.ShortName)
	}
}

func TestRegisterDuplicatePathUpdatesShortName(t *testing.T) {
	home := setupTestHome(t)
	repoPath := filepath.Join(home, "projects", "my-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := Register(repoPath, "first"); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	name, err := Register(repoPath, "second")
	if err != nil {
		t.Fatalf("second Register failed: %v", err)
	}
	if name != "second" {
		t.Errorf("expected 'second', got %q", name)
	}

	entries, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ShortName != "second" {
		t.Errorf("expected short name 'second', got %q", entries[0].ShortName)
	}
}

func TestRegisterDuplicateShortNameDifferentPathErrors(t *testing.T) {
	home := setupTestHome(t)
	repo1 := filepath.Join(home, "projects", "repo1")
	repo2 := filepath.Join(home, "projects", "repo2")
	if err := os.MkdirAll(repo1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repo2, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := Register(repo1, "samename"); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	_, err := Register(repo2, "samename")
	if err == nil {
		t.Fatal("expected error for duplicate short name, got nil")
	}
}

// --- Unregister tests ---

func TestUnregisterByPath(t *testing.T) {
	home := setupTestHome(t)
	repoPath := filepath.Join(home, "projects", "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := Register(repoPath, "repo"); err != nil {
		t.Fatal(err)
	}
	if err := Unregister(repoPath); err != nil {
		t.Fatalf("Unregister by path failed: %v", err)
	}
	entries, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after unregister, got %d", len(entries))
	}
}

func TestUnregisterByShortName(t *testing.T) {
	home := setupTestHome(t)
	repoPath := filepath.Join(home, "projects", "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := Register(repoPath, "myrepo"); err != nil {
		t.Fatal(err)
	}
	if err := Unregister("myrepo"); err != nil {
		t.Fatalf("Unregister by short name failed: %v", err)
	}
	entries, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after unregister, got %d", len(entries))
	}
}

func TestUnregisterNonExistentErrors(t *testing.T) {
	setupTestHome(t)

	err := Unregister("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent repo, got nil")
	}
}

// --- List tests ---

func TestListSortedByShortName(t *testing.T) {
	home := setupTestHome(t)
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		p := filepath.Join(home, "projects", name)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := Register(p, name); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	expected := []string{"alpha", "bravo", "charlie"}
	for i, e := range entries {
		if e.ShortName != expected[i] {
			t.Errorf("index %d: expected %q, got %q", i, expected[i], e.ShortName)
		}
	}
}

func TestListEmptyIndex(t *testing.T) {
	setupTestHome(t)

	entries, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// --- Lookup tests ---

func TestLookupByPath(t *testing.T) {
	home := setupTestHome(t)
	repoPath := filepath.Join(home, "projects", "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Register(repoPath, "repo"); err != nil {
		t.Fatal(err)
	}

	entry, err := Lookup(repoPath)
	if err != nil {
		t.Fatalf("Lookup by path failed: %v", err)
	}
	if entry.Path != repoPath {
		t.Errorf("expected path %q, got %q", repoPath, entry.Path)
	}
}

func TestLookupByShortName(t *testing.T) {
	home := setupTestHome(t)
	repoPath := filepath.Join(home, "projects", "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Register(repoPath, "myrepo"); err != nil {
		t.Fatal(err)
	}

	entry, err := Lookup("myrepo")
	if err != nil {
		t.Fatalf("Lookup by short name failed: %v", err)
	}
	if entry.ShortName != "myrepo" {
		t.Errorf("expected short name 'myrepo', got %q", entry.ShortName)
	}
}

func TestLookupNotFoundErrors(t *testing.T) {
	setupTestHome(t)

	_, err := Lookup("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent lookup, got nil")
	}
}

// --- Context tests ---

func TestAppendGlobalLogAddsEntries(t *testing.T) {
	setupTestHome(t)

	if err := AppendGlobalLog("repo1", "agent1", "first message"); err != nil {
		t.Fatalf("AppendGlobalLog failed: %v", err)
	}
	if err := AppendGlobalLog("repo2", "agent2", "second message"); err != nil {
		t.Fatalf("AppendGlobalLog failed: %v", err)
	}

	ctx, err := LoadGlobalContext()
	if err != nil {
		t.Fatalf("LoadGlobalContext failed: %v", err)
	}
	if len(ctx.Log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(ctx.Log))
	}
	if ctx.Log[0].Repo != "repo1" || ctx.Log[0].Message != "first message" {
		t.Errorf("unexpected first entry: %+v", ctx.Log[0])
	}
	if ctx.Log[1].Repo != "repo2" || ctx.Log[1].Message != "second message" {
		t.Errorf("unexpected second entry: %+v", ctx.Log[1])
	}
}

func TestAppendGlobalLogEmptyMessageErrors(t *testing.T) {
	setupTestHome(t)

	err := AppendGlobalLog("repo", "agent", "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}

func TestLoadGlobalContextEmptyWhenMissing(t *testing.T) {
	setupTestHome(t)

	ctx, err := LoadGlobalContext()
	if err != nil {
		t.Fatalf("LoadGlobalContext failed: %v", err)
	}
	if len(ctx.Log) != 0 {
		t.Errorf("expected 0 log entries, got %d", len(ctx.Log))
	}
	if ctx.Shared != "" {
		t.Errorf("expected empty shared, got %q", ctx.Shared)
	}
}

func TestTrimGlobalLogKeepsLastN(t *testing.T) {
	setupTestHome(t)

	for i := 0; i < 10; i++ {
		if err := AppendGlobalLog("repo", "agent", "msg"); err != nil {
			t.Fatal(err)
		}
	}

	if err := TrimGlobalLog(3); err != nil {
		t.Fatalf("TrimGlobalLog failed: %v", err)
	}

	ctx, err := LoadGlobalContext()
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Log) != 3 {
		t.Errorf("expected 3 entries after trim, got %d", len(ctx.Log))
	}
}

func TestTrimGlobalLogNoOpWhenWithinLimit(t *testing.T) {
	setupTestHome(t)

	for i := 0; i < 3; i++ {
		if err := AppendGlobalLog("repo", "agent", "msg"); err != nil {
			t.Fatal(err)
		}
	}

	if err := TrimGlobalLog(10); err != nil {
		t.Fatalf("TrimGlobalLog failed: %v", err)
	}

	ctx, err := LoadGlobalContext()
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Log) != 3 {
		t.Errorf("expected 3 entries (unchanged), got %d", len(ctx.Log))
	}
}
