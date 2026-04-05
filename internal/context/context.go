//go:build !windows

package context

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// LogEntry is a single attributed entry in the shared agent log.
type LogEntry struct {
	Agent     string    `json:"agent"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// Context is the shared context store for fleet agents.
type Context struct {
	Shared   string              `json:"shared"`
	Agents   map[string]string   `json:"agents"`
	Log      []LogEntry          `json:"log,omitempty"`
	Channels map[string]*Channel `json:"channels,omitempty"`
}

// Channel is a private named space where a fixed set of agents can communicate.
type Channel struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Members     []string   `json:"members"`
	Log         []LogEntry `json:"log,omitempty"`
}

const contextFile = "context.json"
const lockFile = "context.lock"

// Load reads .fleet/context.json from fleetDir. Returns an empty Context if
// the file does not exist.
func Load(fleetDir string) (*Context, error) {
	return loadUnlocked(fleetDir)
}

// loadUnlocked reads context.json without acquiring the lock.
// Only call from within a locked section or when no write is in progress.
func loadUnlocked(fleetDir string) (*Context, error) {
	path := filepath.Join(fleetDir, contextFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Context{Agents: map[string]string{}}, nil
		}
		return nil, fmt.Errorf("failed to read context: %w", err)
	}

	var ctx Context
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to parse context: %w", err)
	}
	if ctx.Agents == nil {
		ctx.Agents = map[string]string{}
	}
	if ctx.Channels == nil {
		ctx.Channels = map[string]*Channel{}
	}
	return &ctx, nil
}

// Save writes the context to .fleet/context.json under an exclusive flock.
// This replaces the entire file. For atomic read-modify-write of individual
// sections, use WriteAgent or WriteShared instead.
func Save(fleetDir string, ctx *Context) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)
	return saveUnlocked(fleetDir, ctx)
}

// saveUnlocked writes context.json without acquiring the lock.
// Only call from within a locked section.
func saveUnlocked(fleetDir string, ctx *Context) error {
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}
	path := filepath.Join(fleetDir, contextFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write context: %w", err)
	}
	return nil
}

func acquireLock(fleetDir string) (*os.File, error) {
	lf, err := os.OpenFile(filepath.Join(fleetDir, lockFile), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open context lock: %w", err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		lf.Close()
		return nil, fmt.Errorf("failed to acquire context lock: %w", err)
	}
	return lf, nil
}

func releaseLock(lf *os.File) {
	syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck
	lf.Close()
}

// WriteAgent updates a single agent's section under lock. It reads the current
// context from disk, updates only the named agent's entry, and writes back.
func WriteAgent(fleetDir, agentName, message string) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	ctx.Agents[agentName] = message
	return saveUnlocked(fleetDir, ctx)
}

// AppendLog adds an attributed entry to the shared agent log under lock.
func AppendLog(fleetDir, agentName, message string) error {
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	ctx.Log = append(ctx.Log, LogEntry{
		Agent:     agentName,
		Timestamp: time.Now().UTC(),
		Message:   message,
	})
	return saveUnlocked(fleetDir, ctx)
}

// TrimLog retains only the last `keep` entries in the shared log.
// No-op if the log already has keep or fewer entries.
// Pass keep=0 to clear the log entirely.
func TrimLog(fleetDir string, keep int) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	if len(ctx.Log) <= keep {
		return nil
	}
	ctx.Log = ctx.Log[len(ctx.Log)-keep:]
	return saveUnlocked(fleetDir, ctx)
}

// TrimChannel retains only the last `keep` entries in the named channel's log.
func TrimChannel(fleetDir, channelName string, keep int) error {
	return fmt.Errorf("not implemented")
}

// CreateChannel creates a new named channel with fixed membership.
// For 2-member channels, the name is auto-set to dm-[member1]-[member2] and
// the provided name is ignored. Returns the resolved channel name.
func CreateChannel(fleetDir, name, description string, members []string) (string, error) {
	if len(members) < 2 {
		return "", fmt.Errorf("channel requires at least 2 members")
	}
	for _, m := range members {
		if m == "" {
			return "", fmt.Errorf("member name cannot be empty")
		}
	}

	// Auto-name DM channels
	if len(members) == 2 {
		name = fmt.Sprintf("dm-[%s]-[%s]", members[0], members[1])
	}

	lf, err := acquireLock(fleetDir)
	if err != nil {
		return "", err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return "", err
	}
	if ctx.Channels == nil {
		ctx.Channels = map[string]*Channel{}
	}
	if _, exists := ctx.Channels[name]; exists {
		return "", fmt.Errorf("channel already exists: %s", name)
	}

	ctx.Channels[name] = &Channel{
		Name:        name,
		Description: description,
		Members:     members,
	}
	if err := saveUnlocked(fleetDir, ctx); err != nil {
		return "", err
	}
	return name, nil
}

// WriteShared updates the shared section under lock. It reads the current
// context from disk, updates the shared field, and writes back.
func WriteShared(fleetDir, message string) error {
	lf, err := acquireLock(fleetDir)
	if err != nil {
		return err
	}
	defer releaseLock(lf)

	ctx, err := loadUnlocked(fleetDir)
	if err != nil {
		return err
	}
	ctx.Shared = message
	return saveUnlocked(fleetDir, ctx)
}
