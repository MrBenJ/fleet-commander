//go:build !windows

package global

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// GlobalLogEntry is a log entry in the global context, including repo info.
type GlobalLogEntry struct {
	Repo      string    `json:"repo"`
	Agent     string    `json:"agent"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// GlobalContext is the cross-repo shared context stored at ~/.fleet/context.json.
type GlobalContext struct {
	Shared string           `json:"shared"`
	Log    []GlobalLogEntry `json:"log,omitempty"`
}

const contextFile = "context.json"
const lockFile = "context.lock"

// LoadGlobalContext reads ~/.fleet/context.json. Returns empty context if missing.
func LoadGlobalContext() (*GlobalContext, error) {
	dir, err := GlobalDir()
	if err != nil {
		return nil, err
	}
	return loadGlobalContextFrom(dir)
}

func loadGlobalContextFrom(dir string) (*GlobalContext, error) {
	path := filepath.Join(dir, contextFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &GlobalContext{}, nil
		}
		return nil, fmt.Errorf("failed to read global context: %w", err)
	}
	var ctx GlobalContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to parse global context: %w", err)
	}
	return &ctx, nil
}

func acquireGlobalLock() (*os.File, error) {
	dir, err := ensureGlobalDir()
	if err != nil {
		return nil, err
	}
	lf, err := os.OpenFile(filepath.Join(dir, lockFile), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open global context lock: %w", err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		lf.Close()
		return nil, fmt.Errorf("failed to acquire global context lock: %w", err)
	}
	return lf, nil
}

func releaseGlobalLock(lf *os.File) {
	// Error ignored intentionally: unlock is best-effort in a defer path.
	syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck
	lf.Close()
}

func saveGlobalContext(dir string, ctx *GlobalContext) error {
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal global context: %w", err)
	}
	path := filepath.Join(dir, contextFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write global context: %w", err)
	}
	return nil
}

// AppendGlobalLog adds a cross-repo log entry to ~/.fleet/context.json.
func AppendGlobalLog(repoShortName, agentName, message string) error {
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	lf, err := acquireGlobalLock()
	if err != nil {
		return err
	}
	defer releaseGlobalLock(lf)

	dir, err := GlobalDir()
	if err != nil {
		return err
	}
	ctx, err := loadGlobalContextFrom(dir)
	if err != nil {
		return err
	}
	ctx.Log = append(ctx.Log, GlobalLogEntry{
		Repo:      repoShortName,
		Agent:     agentName,
		Timestamp: time.Now().UTC(),
		Message:   message,
	})
	return saveGlobalContext(dir, ctx)
}

// TrimGlobalLog keeps only the last N entries.
func TrimGlobalLog(keep int) error {
	lf, err := acquireGlobalLock()
	if err != nil {
		return err
	}
	defer releaseGlobalLock(lf)

	dir, err := GlobalDir()
	if err != nil {
		return err
	}
	ctx, err := loadGlobalContextFrom(dir)
	if err != nil {
		return err
	}
	if len(ctx.Log) <= keep {
		return nil
	}
	ctx.Log = ctx.Log[len(ctx.Log)-keep:]
	return saveGlobalContext(dir, ctx)
}
