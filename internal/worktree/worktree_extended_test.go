package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/worktree"
)

func initGitRepo(t *testing.T) string {
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

func TestCreate_And_Remove(t *testing.T) {
	repoDir := initGitRepo(t)
	m := worktree.NewManager(repoDir)

	wtPath := filepath.Join(t.TempDir(), "wt-test")
	if err := m.Create(wtPath, "feature/test"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Worktree should exist
	if !m.Exists(wtPath) {
		t.Error("Exists() returned false for just-created worktree")
	}

	// .git file should exist in worktree
	if _, err := os.Stat(filepath.Join(wtPath, ".git")); err != nil {
		t.Errorf("worktree .git not found: %v", err)
	}

	// List should show more than just the main worktree
	paths, err := m.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(paths) < 2 {
		t.Errorf("expected at least 2 worktrees (main + new), got %d: %v", len(paths), paths)
	}

	// Remove it
	if err := m.Remove(wtPath); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Should no longer exist
	if m.Exists(wtPath) {
		t.Error("Exists() returned true after removal")
	}
}

func TestCreateFromExisting(t *testing.T) {
	repoDir := initGitRepo(t)

	// Create a branch first
	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch failed: %s", out)
	}

	m := worktree.NewManager(repoDir)
	wtPath := filepath.Join(t.TempDir(), "wt-existing")
	if err := m.CreateFromExisting(wtPath, "existing-branch"); err != nil {
		t.Fatalf("CreateFromExisting failed: %v", err)
	}

	if !m.Exists(wtPath) {
		t.Error("worktree should exist after CreateFromExisting")
	}

	// Cleanup
	m.Remove(wtPath)
}

func TestExists_NoWorktree(t *testing.T) {
	m := worktree.NewManager("/tmp/fake")
	if m.Exists("/tmp/definitely-not-a-worktree-ever") {
		t.Error("Exists() should return false for non-existent path")
	}
}

func TestRemove_NonexistentPath(t *testing.T) {
	repoDir := initGitRepo(t)
	m := worktree.NewManager(repoDir)

	// Removing a non-existent path — should not error because os.RemoveAll
	// on a non-existent path is a no-op
	err := m.Remove("/tmp/does-not-exist-at-all-ever-" + t.Name())
	if err != nil {
		t.Errorf("Remove of non-existent path should not error, got: %v", err)
	}
}

func TestCreate_NestedParentDirs(t *testing.T) {
	repoDir := initGitRepo(t)
	m := worktree.NewManager(repoDir)

	wtPath := filepath.Join(t.TempDir(), "deep", "nested", "wt")
	if err := m.Create(wtPath, "feature/nested"); err != nil {
		t.Fatalf("Create with nested dirs failed: %v", err)
	}
	if !m.Exists(wtPath) {
		t.Error("worktree should exist after Create with nested dirs")
	}
	m.Remove(wtPath)
}
