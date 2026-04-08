//go:build !windows

package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const lockTimeout = 5 * time.Second

// acquireLock opens .fleet/config.lock and acquires an exclusive flock.
// It retries with LOCK_NB for up to lockTimeout before giving up, so a
// crashed process holding a stale lock won't hang fleet indefinitely.
// The caller must call releaseLock when the critical section is done.
func acquireLock(fleetDir string) (*os.File, error) {
	lf, err := os.OpenFile(filepath.Join(fleetDir, "config.lock"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	deadline := time.Now().Add(lockTimeout)
	for {
		err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return lf, nil
		}
		if time.Now().After(deadline) {
			lf.Close()
			return nil, fmt.Errorf("timed out waiting for config lock (another fleet command may be stuck — run 'fleet unlock' to force-release)")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ForceUnlock removes the config lock file. Use only as an escape hatch
// when a crashed process left a stale lock.
func ForceUnlock(fleetDir string) error {
	lockPath := filepath.Join(fleetDir, "config.lock")
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}
	return nil
}

// releaseLock releases the flock and closes the file.
func releaseLock(lf *os.File) {
	// Error ignored intentionally: unlock is best-effort in a defer path.
	// If unlock fails, the OS releases the flock when the process exits.
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
