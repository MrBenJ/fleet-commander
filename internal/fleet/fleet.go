package fleet

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MrBenJ/fleet-commander/internal/global"
	"github.com/MrBenJ/fleet-commander/internal/worktree"
)

// validAgentName matches alphanumeric names with hyphens and underscores.
// Periods are excluded because tmux uses them as session name separators.
var validAgentName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

//go:embed system_prompt.md
var defaultSystemPrompt string

// Fleet represents a managed repository with multiple agent workspaces
type Fleet struct {
	RepoPath  string   `json:"repo_path"`
	FleetDir  string   `json:"fleet_dir"`
	ShortName string   `json:"short_name,omitempty"`
	Agents    []*Agent `json:"agents"`
}

// DriverConfig holds configuration for the generic driver or custom agent commands.
type DriverConfig struct {
	Command         string   `json:"command,omitempty"`
	Args            []string `json:"args,omitempty"`
	YoloArgs        []string `json:"yolo_args,omitempty"`
	PromptFlag      string   `json:"prompt_flag,omitempty"`
	PromptFromFile  bool     `json:"prompt_from_file,omitempty"`
	WaitingPatterns []string `json:"waiting_patterns,omitempty"`
	WorkingPatterns []string `json:"working_patterns,omitempty"`
}

// Agent represents a single agent workspace
type Agent struct {
	Name          string        `json:"name"`
	Branch        string        `json:"branch"`
	WorktreePath  string        `json:"worktree_path"`
	Status        string        `json:"status"`
	PID           int           `json:"pid"`
	StateFilePath string        `json:"state_file_path,omitempty"`
	HooksOK       bool          `json:"hooks_ok"`
	Driver        string        `json:"driver,omitempty"`
	DriverConfig  *DriverConfig `json:"driver_config,omitempty"`
}

const fleetDirName = ".fleet"
const configFile = fleetDirName + "/config.json"

// Init initializes a new fleet for the given repository.
// If shortName is empty, it defaults to the directory basename.
func Init(repoPath, shortName string) (*Fleet, error) {
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
	fleetDir := filepath.Join(absPath, fleetDirName)
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create fleet directory: %w", err)
	}
	
	// Create worktrees directory
	worktreesDir := filepath.Join(fleetDir, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create worktrees directory: %w", err)
	}
	
	// Write default system prompt (only if it doesn't already exist)
	promptPath := filepath.Join(fleetDir, "FLEET_SYSTEM_PROMPT.md")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		if writeErr := os.WriteFile(promptPath, []byte(defaultSystemPrompt), 0644); writeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write default system prompt: %v\n", writeErr)
		}
	}

	// Add .fleet to .gitignore if not already there
	addToGitignore(absPath, fleetDirName)
	addToGitignore(absPath, fleetDirName+"/config.lock")

	// Register in global index (best-effort — don't fail init if global dir is broken)
	resolvedName, regErr := global.Register(absPath, shortName)
	if regErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not register in global index: %v\n", regErr)
		if shortName == "" {
			resolvedName = filepath.Base(absPath)
		} else {
			resolvedName = shortName
		}
	}

	f := &Fleet{
		RepoPath:  absPath,
		FleetDir:  fleetDir,
		ShortName: resolvedName,
		Agents:    []*Agent{},
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

	startDir := dir

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

	return nil, fmt.Errorf("no fleet found (searched from %s up to /)", startDir)
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
	if !validAgentName.MatchString(name) {
		return nil, fmt.Errorf("invalid agent name '%s': must be alphanumeric with hyphens/underscores, starting with a letter or number", name)
	}

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
	if !validAgentName.MatchString(newName) {
		return fmt.Errorf("invalid agent name '%s': must be alphanumeric with hyphens/underscores, starting with a letter or number", newName)
	}

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
			if renameErr := os.Rename(newStateFilePath, agent.StateFilePath); renameErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not roll back state file rename: %v\n", renameErr)
			}
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

// UpdateAgentDriver sets the driver for an agent.
func (f *Fleet) UpdateAgentDriver(name, driverName string) error {
	return f.withLock(func() error {
		for _, a := range f.Agents {
			if a.Name == name {
				a.Driver = driverName
				return nil
			}
		}
		return fmt.Errorf("agent '%s' not found", name)
	})
}

// CurrentBranch returns the currently checked-out branch of the fleet's root repo.
func (f *Fleet) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// TmuxPrefix returns the tmux session prefix for this fleet.
func (f *Fleet) TmuxPrefix() string {
	if f.ShortName != "" {
		return "fleet-" + f.ShortName
	}
	return "fleet"
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

// LoadSystemPrompt reads the fleet system prompt from .fleet/FLEET_SYSTEM_PROMPT.md.
// Returns empty string if the file is missing or empty.
func LoadSystemPrompt(fleetDir string) (string, error) {
	path := filepath.Join(fleetDir, "FLEET_SYSTEM_PROMPT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read system prompt: %w", err)
	}
	return string(data), nil
}
