package worktree_test

import (
	"path/filepath"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/worktree"
)

func TestMove_RelocatesWorktree(t *testing.T) {
	repoDir := initGitRepo(t)
	m := worktree.NewManager(repoDir)

	oldPath := filepath.Join(t.TempDir(), "old-wt")
	if err := m.Create(oldPath, "feature/move"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer m.Remove(oldPath)

	newPath := filepath.Join(t.TempDir(), "new-wt")
	if err := m.Move(oldPath, newPath); err != nil {
		t.Fatalf("Move: %v", err)
	}

	if m.Exists(oldPath) {
		t.Error("worktree should not exist at old path after move")
	}
	if !m.Exists(newPath) {
		t.Error("worktree should exist at new path after move")
	}

	// Cleanup the new path location.
	t.Cleanup(func() { m.Remove(newPath) })
}

func TestMove_NonexistentSource(t *testing.T) {
	repoDir := initGitRepo(t)
	m := worktree.NewManager(repoDir)

	err := m.Move("/tmp/does-not-exist-source", "/tmp/does-not-exist-dest")
	if err == nil {
		t.Fatal("Move of nonexistent source should fail")
	}
}

func TestMove_NonRepoFails(t *testing.T) {
	// Calling Move from a directory that isn't a git repo should fail.
	dir := t.TempDir() // plain dir, not a git repo
	m := worktree.NewManager(dir)

	src := filepath.Join(t.TempDir(), "src-wt")
	dest := filepath.Join(t.TempDir(), "dest-wt")

	if err := m.Move(src, dest); err == nil {
		t.Error("Move from non-repo should fail")
	}
}
