//go:build !windows

package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquireLock opens .fleet/config.lock and acquires an exclusive flock.
// The caller must call releaseLock when the critical section is done.
// Requires fleetDir to already exist.
func acquireLock(fleetDir string) (*os.File, error) {
	lf, err := os.OpenFile(filepath.Join(fleetDir, "config.lock"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		lf.Close()
		return nil, fmt.Errorf("failed to acquire config lock: %w", err)
	}
	return lf, nil
}

// releaseLock releases the flock and closes the file.
func releaseLock(lf *os.File) {
	syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck
	lf.Close()
}

// readConfig reads and parses the fleet config at path without acquiring any lock.
// Only call this from within withLock where the lock is already held.
func readConfig(path string) (*Fleet, error) {
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

// writeConfig marshals f and writes it to disk without acquiring any lock.
// Only call this from within withLock where the lock is already held,
// or from Init() where no concurrent access is possible.
func (f *Fleet) writeConfig() error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	configPath := filepath.Join(f.FleetDir, "config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// withLock acquires an exclusive lock on .fleet/config.lock, re-reads the
// config from disk so fn sees the latest state, runs fn (which mutates f),
// then writes back to disk. This prevents logical write conflicts between
// concurrent fleet commands.
func (f *Fleet) withLock(fn func() error) error {
	lf, err := acquireLock(f.FleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	// Re-read from disk inside the lock so we always have the latest state.
	// Preserve FleetDir which is derived at load time, not stored in config.
	fresh, err := readConfig(filepath.Join(f.FleetDir, "config.json"))
	if err != nil {
		return fmt.Errorf("failed to re-read config: %w", err)
	}
	fleetDir := f.FleetDir
	*f = *fresh
	f.FleetDir = fleetDir

	if err := fn(); err != nil {
		return err
	}
	return f.writeConfig()
}
