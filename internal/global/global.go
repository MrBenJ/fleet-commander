//go:build !windows

package global

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
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

const indexLockFile = "repos.lock"

func acquireIndexLock() (*os.File, error) {
	dir, err := ensureGlobalDir()
	if err != nil {
		return nil, err
	}
	lf, err := os.OpenFile(filepath.Join(dir, indexLockFile), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open index lock: %w", err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		lf.Close()
		return nil, fmt.Errorf("failed to acquire index lock: %w", err)
	}
	return lf, nil
}

func releaseIndexLock(lf *os.File) {
	syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck
	lf.Close()
}

// loadIndex reads ~/.fleet/repos.json without locking.
func loadIndex() (*Index, error) {
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

// saveIndex writes the index to ~/.fleet/repos.json without locking.
func saveIndex(idx *Index) error {
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

	lf, err := acquireIndexLock()
	if err != nil {
		return "", err
	}
	defer releaseIndexLock(lf)

	idx, err := loadIndex()
	if err != nil {
		return "", err
	}

	for i, r := range idx.Repos {
		if r.Path == absPath {
			idx.Repos[i].ShortName = shortName
			return shortName, saveIndex(idx)
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
	return shortName, saveIndex(idx)
}

// Unregister removes a repo from the global index by path or short name.
func Unregister(pathOrName string) error {
	absPath, _ := filepath.Abs(pathOrName)

	lf, err := acquireIndexLock()
	if err != nil {
		return err
	}
	defer releaseIndexLock(lf)

	idx, err := loadIndex()
	if err != nil {
		return err
	}

	for i, r := range idx.Repos {
		if r.Path == absPath || r.ShortName == pathOrName {
			idx.Repos = append(idx.Repos[:i], idx.Repos[i+1:]...)
			return saveIndex(idx)
		}
	}
	return fmt.Errorf("repo not found in global index: %s", pathOrName)
}

// List returns all registered repos sorted by short name.
func List() ([]RepoEntry, error) {
	idx, err := loadIndex()
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

	idx, err := loadIndex()
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
