package global

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// RepoEntry is a single registered repository in the global index.
type RepoEntry struct {
	Path      string    `json:"path"`
	ShortName string    `json:"short_name"`
	AddedAt   time.Time `json:"added_at"`
}

// Index is the global fleet index stored at ~/.fleet/repos.json.
type Index struct {
	Repos []RepoEntry `json:"repos"`
}

// GlobalDir returns the path to the global fleet directory (~/.fleet).
func GlobalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".fleet"), nil
}

// ensureGlobalDir creates ~/.fleet if it doesn't exist.
func ensureGlobalDir() (string, error) {
	dir, err := GlobalDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create global fleet directory: %w", err)
	}
	return dir, nil
}

// LoadIndex reads ~/.fleet/repos.json. Returns an empty Index if the file
// does not exist.
func LoadIndex() (*Index, error) {
	dir, err := GlobalDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "repos.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Index{}, nil
		}
		return nil, fmt.Errorf("failed to read global index: %w", err)
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse global index: %w", err)
	}
	return &idx, nil
}

// SaveIndex writes the index to ~/.fleet/repos.json.
func SaveIndex(idx *Index) error {
	dir, err := ensureGlobalDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal global index: %w", err)
	}
	path := filepath.Join(dir, "repos.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write global index: %w", err)
	}
	return nil
}

// Register adds a repo to the global index. If already registered, it updates
// the short name. Returns the resolved short name.
func Register(repoPath, shortName string) (string, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	if shortName == "" {
		shortName = filepath.Base(absPath)
	}

	idx, err := LoadIndex()
	if err != nil {
		return "", err
	}

	// Check for duplicate short name pointing to a different repo
	for i, r := range idx.Repos {
		if r.Path == absPath {
			idx.Repos[i].ShortName = shortName
			return shortName, SaveIndex(idx)
		}
		if r.ShortName == shortName && r.Path != absPath {
			return "", fmt.Errorf("short name '%s' already used by %s", shortName, r.Path)
		}
	}

	idx.Repos = append(idx.Repos, RepoEntry{
		Path:      absPath,
		ShortName: shortName,
		AddedAt:   time.Now().UTC(),
	})
	return shortName, SaveIndex(idx)
}

// Unregister removes a repo from the global index by path or short name.
func Unregister(pathOrName string) error {
	absPath, _ := filepath.Abs(pathOrName)

	idx, err := LoadIndex()
	if err != nil {
		return err
	}

	for i, r := range idx.Repos {
		if r.Path == absPath || r.ShortName == pathOrName {
			idx.Repos = append(idx.Repos[:i], idx.Repos[i+1:]...)
			return SaveIndex(idx)
		}
	}
	return fmt.Errorf("repo not found in global index: %s", pathOrName)
}

// List returns all registered repos sorted by short name.
func List() ([]RepoEntry, error) {
	idx, err := LoadIndex()
	if err != nil {
		return nil, err
	}
	sort.Slice(idx.Repos, func(i, j int) bool {
		return idx.Repos[i].ShortName < idx.Repos[j].ShortName
	})
	return idx.Repos, nil
}

// Lookup finds a repo entry by path or short name.
func Lookup(pathOrName string) (*RepoEntry, error) {
	absPath, _ := filepath.Abs(pathOrName)

	idx, err := LoadIndex()
	if err != nil {
		return nil, err
	}
	for _, r := range idx.Repos {
		if r.Path == absPath || r.ShortName == pathOrName {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("repo not found in global index: %s", pathOrName)
}

// LookupByPath finds a repo entry by its absolute path.
func LookupByPath(repoPath string) (*RepoEntry, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	idx, err := LoadIndex()
	if err != nil {
		return nil, err
	}
	for _, r := range idx.Repos {
		if r.Path == absPath {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("repo not registered in global index: %s", absPath)
}
