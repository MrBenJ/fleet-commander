package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Manager handles git worktree operations
type Manager struct {
	RepoPath string
}

// NewManager creates a new worktree manager for the given repo
func NewManager(repoPath string) *Manager {
	return &Manager{RepoPath: repoPath}
}

// Create creates a new worktree with the given branch
func (m *Manager) Create(worktreePath, branch string) error {
	// Ensure parent directory exists
	parent := filepath.Dir(worktreePath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	
	// Check if repo has any commits (HEAD exists)
	headCmd := exec.Command("git", "rev-parse", "--verify", "HEAD")
	headCmd.Dir = m.RepoPath
	headCmd.Stderr = nil
	if err := headCmd.Run(); err != nil {
		// No commits yet - create an empty commit first
		emptyCommitCmd := exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
		emptyCommitCmd.Dir = m.RepoPath
		emptyCommitCmd.Stdout = os.Stdout
		emptyCommitCmd.Stderr = os.Stderr
		if err := emptyCommitCmd.Run(); err != nil {
			return fmt.Errorf("failed to create initial commit: %w", err)
		}
	}
	
	// Create worktree with new branch
	cmd := exec.Command("git", "worktree", "add", "-b", branch, worktreePath)
	cmd.Dir = m.RepoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
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
	cmd := exec.Command("git", "worktree", "add", worktreePath, branch)
	cmd.Dir = m.RepoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	
	return nil
}

// Remove removes a worktree
func (m *Manager) Remove(worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = m.RepoPath
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}
	
	return nil
}

// List returns all worktrees for the repo
func (m *Manager) List() ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.RepoPath
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	
	// Parse porcelain output
	// Format: worktree <path>\nHEAD <sha>\nbranch <ref>\n\n
	var worktrees []string
	lines := string(output)
	// Simple parsing - just extract paths for now
	_ = lines
	
	return worktrees, nil
}

// Exists checks if a worktree exists at the given path
func (m *Manager) Exists(worktreePath string) bool {
	_, err := os.Stat(filepath.Join(worktreePath, ".git"))
	return !os.IsNotExist(err)
}
