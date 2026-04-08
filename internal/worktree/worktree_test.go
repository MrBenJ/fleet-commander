package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/worktree"
)

func TestListReturnsMainWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

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
	absRepo, _ := filepath.Abs(repoDir)
	if len(paths) != 1 {
		t.Errorf("expected 1 worktree, got %d: %v", len(paths), paths)
	}
	if paths[0] != absRepo {
		if _, err := os.Stat(paths[0]); err != nil {
			t.Errorf("worktree path %q does not exist: %v", paths[0], err)
		}
	}
}
