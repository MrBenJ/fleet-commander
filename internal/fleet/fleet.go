package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	Name         string `json:"name"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktree_path"`
	Status       string `json:"status"`
	PID          int    `json:"pid"`
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
	
	f := &Fleet{
		RepoPath: absPath,
		FleetDir: fleetDir,
		Agents:   []*Agent{},
	}
	
	if err := f.save(); err != nil {
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

// save persists the fleet configuration
func (f *Fleet) save() error {
	configPath := filepath.Join(f.FleetDir, "config.json")
	
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	
	return nil
}

// AddAgent creates a new agent workspace
func (f *Fleet) AddAgent(name, branch string) (*Agent, error) {
	// Check for duplicate name
	for _, a := range f.Agents {
		if a.Name == name {
			return nil, fmt.Errorf("agent '%s' already exists", name)
		}
	}
	
	// Create worktree path
	worktreePath := filepath.Join(f.FleetDir, "worktrees", name)
	
	// Create git worktree
	wt := worktree.NewManager(f.RepoPath)
	if err := wt.Create(worktreePath, branch); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}
	
	agent := &Agent{
		Name:         name,
		Branch:       branch,
		WorktreePath: worktreePath,
		Status:       "ready",
		PID:          0,
	}
	
	f.Agents = append(f.Agents, agent)
	
	if err := f.save(); err != nil {
		return nil, err
	}
	
	return agent, nil
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
	for i, a := range f.Agents {
		if a.Name == name {
			f.Agents = append(f.Agents[:i], f.Agents[i+1:]...)
			return f.save()
		}
	}
	return fmt.Errorf("agent '%s' not found", name)
}

// UpdateAgent updates an agent's status and PID
func (f *Fleet) UpdateAgent(name string, status string, pid int) error {
	for _, a := range f.Agents {
		if a.Name == name {
			a.Status = status
			a.PID = pid
			return f.save()
		}
	}
	return fmt.Errorf("agent '%s' not found", name)
}
