package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LaunchLogger writes structured debug logs to .fleet/launch.log
// so failures during fleet launch can be diagnosed after the TUI exits.
type LaunchLogger struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// NewLaunchLogger creates a logger that writes to .fleet/launch.log.
// If fleetDir is empty or the file can't be opened, logging is silently skipped.
func NewLaunchLogger(fleetDir string) *LaunchLogger {
	if fleetDir == "" {
		return &LaunchLogger{}
	}

	logPath := filepath.Join(fleetDir, "launch.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return &LaunchLogger{}
	}

	l := &LaunchLogger{file: f, path: logPath}
	l.write("=== Fleet Launch started at %s ===", time.Now().Format(time.RFC3339))
	return l
}

// Log writes a timestamped message to the log file.
func (l *LaunchLogger) Log(format string, args ...interface{}) {
	l.write(format, args...)
}

func (l *LaunchLogger) write(format string, args ...interface{}) {
	if l == nil || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "[%s] %s\n", ts, msg)
}

// Close flushes and closes the log file.
func (l *LaunchLogger) Close() {
	if l == nil || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.file.Close()
}

// Path returns the log file path (for display to user).
func (l *LaunchLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
