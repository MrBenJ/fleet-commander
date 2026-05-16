package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MrBenJ/fleet-commander/internal/execx"
)

// Manager handles git worktree operations
type Manager struct {
	RepoPath string
	runner   execx.Runner
}

// NewManager creates a new worktree manager for the given repo
func NewManager(repoPath string) *Manager {
	return NewManagerWithRunner(repoPath, execx.DefaultRunner())
}

func NewManagerWithRunner(repoPath string, runner execx.Runner) *Manager {
	if runner == nil {
		runner = execx.DefaultRunner()
	}
	return &Manager{RepoPath: repoPath, runner: runner}
}

// Create creates a new worktree with the given branch
func (m *Manager) Create(worktreePath, branch string) error {
	// Ensure parent directory exists
	parent := filepath.Dir(worktreePath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Check if repo has any commits (HEAD exists)
	ctx := context.Background()
	if err := m.runner.Run(ctx, execx.Options{
		Name: "git",
		Args: []string{"rev-parse", "--verify", "HEAD"},
		Dir:  m.RepoPath,
	}); err != nil {
		// No commits yet - create an empty commit first
		if err := m.runner.Run(ctx, execx.Options{
			Name:   "git",
			Args:   []string{"commit", "--allow-empty", "-m", "Initial commit"},
			Dir:    m.RepoPath,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}); err != nil {
			return fmt.Errorf("failed to create initial commit: %w", err)
		}
	}

	// Create worktree with new branch
	if err := m.runner.Run(ctx, execx.Options{
		Name:   "git",
		Args:   []string{"worktree", "add", "-b", branch, worktreePath},
		Dir:    m.RepoPath,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	return nil
}

// CreateFromExisting creates a worktree from an existing branch
func (m *Manager) CreateFromExisting(worktreePath, branch string) error {
	// Ensure parent directory exists
	parent := filepath.Dir(worktreePath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create worktree from existing branch (no -b flag)
	if err := m.runner.Run(context.Background(), execx.Options{
		Name:   "git",
		Args:   []string{"worktree", "add", worktreePath, branch},
		Dir:    m.RepoPath,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	return nil
}

// Remove removes a worktree. It escalates through three strategies:
// normal remove → force remove → os.RemoveAll (for leftover directories
// that git no longer recognizes as worktrees).
func (m *Manager) Remove(worktreePath string) error {
	ctx := context.Background()
	if err := m.runner.Run(ctx, execx.Options{
		Name: "git",
		Args: []string{"worktree", "remove", worktreePath},
		Dir:  m.RepoPath,
	}); err == nil {
		return nil
	}

	if err := m.runner.Run(ctx, execx.Options{
		Name: "git",
		Args: []string{"worktree", "remove", "--force", worktreePath},
		Dir:  m.RepoPath,
	}); err == nil {
		return nil
	}

	// Git doesn't recognize it as a worktree — just remove the leftover directory
	if err := os.RemoveAll(worktreePath); err != nil {
		return fmt.Errorf("failed to remove worktree directory: %w", err)
	}
	return nil
}

// List returns all worktree paths for the repo.
// The first entry is always the main worktree (the repo root itself).
// Callers who want only fleet-managed worktrees must filter it out.
func (m *Manager) List() ([]string, error) {
	output, err := m.runner.Output(context.Background(), execx.Options{
		Name: "git",
		Args: []string{"worktree", "list", "--porcelain"},
		Dir:  m.RepoPath,
	})
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

// Move moves a worktree to a new path using git worktree move
func (m *Manager) Move(oldPath, newPath string) error {
	if out, err := m.runner.CombinedOutput(context.Background(), execx.Options{
		Name: "git",
		Args: []string{"worktree", "move", oldPath, newPath},
		Dir:  m.RepoPath,
	}); err != nil {
		return fmt.Errorf("failed to move worktree: %s", strings.TrimSpace(string(out)))
	}

	return nil
}

// Exists checks if a worktree exists at the given path
func (m *Manager) Exists(worktreePath string) bool {
	_, err := os.Stat(filepath.Join(worktreePath, ".git"))
	return !os.IsNotExist(err)
}
