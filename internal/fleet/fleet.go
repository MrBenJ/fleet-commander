package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/teknal/fleet-commander/internal/worktree"
)

// Fleet represents a managed repository with multiple agent workspaces
type Fleet struct {
	RepoPath string   `json:"repo_path"`
	FleetDir string   `json:"fleet_dir"`
	Agents   []*Agent `json:"agents"`
}

// Agent represents a single agent workspace
type Agent struct {
	Name          string `json:"name"`
	Branch        string `json:"branch"`
	WorktreePath  string `json:"worktree_path"`
	Status        string `json:"status"`
	PID           int    `json:"pid"`
	StateFilePath string `json:"state_file_path,omitempty"`
	HooksOK       bool   `json:"hooks_ok"`
}

const configFile = ".fleet/config.json"

// Init initializes a new fleet for the given repository
func Init(repoPath string) (*Fleet, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}
	
	// Verify it's a git repo
	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s is not a git repository", absPath)
	}
	
	// Create fleet directory
	fleetDir := filepath.Join(absPath, ".fleet")
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create fleet directory: %w", err)
	}
	
	// Create worktrees directory
	worktreesDir := filepath.Join(fleetDir, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create worktrees directory: %w", err)
	}
	
	// Add .fleet to .gitignore if not already there
	addToGitignore(absPath, ".fleet")
	addToGitignore(absPath, ".fleet/config.lock")

	f := &Fleet{
		RepoPath: absPath,
		FleetDir: fleetDir,
		Agents:   []*Agent{},
	}
	
	if err := f.writeConfig(); err != nil {
		return nil, fmt.Errorf("failed to save fleet config: %w", err)
	}
	
	return f, nil
}

// Load loads a fleet from the given directory (or current directory)
func Load(dir string) (*Fleet, error) {
	if dir == "" || dir == "." {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}
	
	// Walk up to find .fleet directory
	for {
		configPath := filepath.Join(dir, configFile)
		if _, err := os.Stat(configPath); err == nil {
			return loadFromPath(configPath)
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	
	return nil, fmt.Errorf("no fleet found (looked for %s)", configFile)
}

func loadFromPath(path string) (*Fleet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	
	var f Fleet
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	return &f, nil
}


// AddAgent creates a new agent workspace with a git worktree.
// If the config save fails after the worktree is created, the worktree is
// cleaned up automatically to avoid leaving orphaned directories.
func (f *Fleet) AddAgent(name, branch string) (*Agent, error) {
	// Fast-fail for duplicate before acquiring lock.
	for _, a := range f.Agents {
		if a.Name == name {
			return nil, fmt.Errorf("agent '%s' already exists", name)
		}
	}

	worktreePath := filepath.Join(f.FleetDir, "worktrees", name)
	wt := worktree.NewManager(f.RepoPath)
	if err := wt.Create(worktreePath, branch); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	var created *Agent
	err := f.withLock(func() error {
		// Re-check inside the lock: another concurrent process may have added
		// an agent with the same name between our fast-fail check and now.
		for _, a := range f.Agents {
			if a.Name == name {
				return fmt.Errorf("agent '%s' already exists", name)
			}
		}
		created = &Agent{
			Name:         name,
			Branch:       branch,
			WorktreePath: worktreePath,
			Status:       "ready",
			PID:          0,
		}
		f.Agents = append(f.Agents, created)
		return nil // withLock calls writeConfig after this returns nil
	})

	if err != nil {
		// Rollback: remove the worktree we already created so it doesn't orphan.
		if removeErr := wt.Remove(worktreePath); removeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not clean up orphaned worktree at %s: %v\n",
				worktreePath, removeErr)
		}
		return nil, err
	}
	return created, nil
}

// GetAgent returns an agent by name
func (f *Fleet) GetAgent(name string) (*Agent, error) {
	for _, a := range f.Agents {
		if a.Name == name {
			return a, nil
		}
	}
	return nil, fmt.Errorf("agent '%s' not found", name)
}

// RemoveAgent removes an agent from the fleet config
func (f *Fleet) RemoveAgent(name string) error {
	return f.withLock(func() error {
		for i, a := range f.Agents {
			if a.Name == name {
				f.Agents = append(f.Agents[:i], f.Agents[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}

// RenameAgent renames an agent, moving its worktree and state file.
// The agent must be stopped (no active tmux session) before renaming.
func (f *Fleet) RenameAgent(oldName, newName string) error {
	// Fast-fail checks before acquiring lock.
	if oldName == newName {
		return fmt.Errorf("new name is the same as the current name")
	}
	for _, a := range f.Agents {
		if a.Name == newName {
			return fmt.Errorf("agent '%s' already exists", newName)
		}
	}

	agent, err := f.GetAgent(oldName)
	if err != nil {
		return err
	}

	oldWorktreePath := agent.WorktreePath
	newWorktreePath := filepath.Join(f.FleetDir, "worktrees", newName)

	// Move the git worktree
	wt := worktree.NewManager(f.RepoPath)
	if err := wt.Move(oldWorktreePath, newWorktreePath); err != nil {
		return fmt.Errorf("failed to move worktree: %w", err)
	}

	// Rename state file if present
	var newStateFilePath string
	if agent.StateFilePath != "" {
		newStateFilePath = filepath.Join(f.FleetDir, "states", newName+".json")
		if err := os.Rename(agent.StateFilePath, newStateFilePath); err != nil {
			// Non-fatal: state file may not exist on disk
			newStateFilePath = ""
		}
	}

	err = f.withLock(func() error {
		// Re-check inside the lock.
		for _, a := range f.Agents {
			if a.Name == newName {
				return fmt.Errorf("agent '%s' already exists", newName)
			}
		}
		for _, a := range f.Agents {
			if a.Name == oldName {
				a.Name = newName
				a.WorktreePath = newWorktreePath
				if newStateFilePath != "" {
					a.StateFilePath = newStateFilePath
				} else if a.StateFilePath != "" {
					a.StateFilePath = ""
				}
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", oldName)
	})

	if err != nil {
		// Rollback: move worktree back
		if moveErr := wt.Move(newWorktreePath, oldWorktreePath); moveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not roll back worktree move: %v\n", moveErr)
		}
		// Rollback: move state file back
		if newStateFilePath != "" && agent.StateFilePath != "" {
			os.Rename(newStateFilePath, agent.StateFilePath)
		}
		return err
	}

	return nil
}

// UpdateAgent updates an agent's status and PID
func (f *Fleet) UpdateAgent(name string, status string, pid int) error {
	return f.withLock(func() error {
		for _, a := range f.Agents {
			if a.Name == name {
				a.Status = status
				a.PID = pid
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}

// UpdateAgentStateFile persists the state file path for an agent
func (f *Fleet) UpdateAgentStateFile(name, stateFilePath string) error {
	return f.withLock(func() error {
		for _, a := range f.Agents {
			if a.Name == name {
				a.StateFilePath = stateFilePath
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}

// UpdateAgentHooks records whether fleet hooks are currently injected for an agent.
func (f *Fleet) UpdateAgentHooks(name string, hooksOK bool) error {
	return f.withLock(func() error {
		for _, a := range f.Agents {
			if a.Name == name {
				a.HooksOK = hooksOK
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}

// addToGitignore adds an entry to .gitignore if it's not already present
func addToGitignore(repoPath, entry string) {
	gitignorePath := filepath.Join(repoPath, ".gitignore")

	// Read existing content
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return // can't read, skip
	}

	// Check if entry already exists
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == entry {
			return // already there
		}
	}

	// Append entry
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// Add newline before entry if file doesn't end with one
	if len(content) > 0 && content[len(content)-1] != '\n' {
		f.WriteString("\n")
	}
	f.WriteString(entry + "\n")
}
