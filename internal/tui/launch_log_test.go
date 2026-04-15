package tui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewLaunchLogger_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		t.Fatal(err)
	}

	l := NewLaunchLogger(fleetDir)
	defer l.Close()

	if l.Path() == "" {
		t.Error("expected non-empty log path")
	}

	logPath := filepath.Join(fleetDir, "launch.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file not created on disk")
	}

	// Should have the startup header
	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "Fleet Launch started") {
		t.Error("missing startup header in log file")
	}
}

func TestNewLaunchLogger_EmptyFleetDir(t *testing.T) {
	l := NewLaunchLogger("")
	defer l.Close()

	if l.Path() != "" {
		t.Errorf("expected empty path for empty fleetDir, got %q", l.Path())
	}

	// Should not panic when logging to a nil-file logger
	l.Log("this should be a no-op: %d", 42)
}

func TestNewLaunchLogger_InvalidDir(t *testing.T) {
	l := NewLaunchLogger("/nonexistent/path/that/doesnt/exist")
	defer l.Close()

	if l.Path() != "" {
		t.Errorf("expected empty path for invalid dir, got %q", l.Path())
	}
}

func TestLaunchLogger_Log(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	os.MkdirAll(fleetDir, 0755)

	l := NewLaunchLogger(fleetDir)
	l.Log("test message: %s %d", "hello", 42)
	l.Close()

	data, _ := os.ReadFile(l.Path())
	content := string(data)

	if !strings.Contains(content, "test message: hello 42") {
		t.Errorf("log missing expected message, got:\n%s", content)
	}

	// Should have timestamps in HH:MM:SS.mmm format
	if !strings.Contains(content, "[") || !strings.Contains(content, "]") {
		t.Error("log entries should have timestamps in brackets")
	}
}

func TestLaunchLogger_NilSafety(t *testing.T) {
	var l *LaunchLogger

	// None of these should panic
	l.Log("should not panic")
	l.Close()

	if l.Path() != "" {
		t.Error("nil logger Path() should return empty string")
	}
}

func TestLaunchLogger_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	os.MkdirAll(fleetDir, 0755)

	l := NewLaunchLogger(fleetDir)
	defer l.Close()

	const goroutines = 20
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			l.Log("message from goroutine %d", idx)
		}(i)
	}

	wg.Wait()
	l.Close()

	data, _ := os.ReadFile(l.Path())
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	// startup header + goroutines messages
	// At minimum we should have all goroutine messages (some may interleave)
	messageCount := 0
	for _, line := range lines {
		if strings.Contains(line, "message from goroutine") {
			messageCount++
		}
	}
	if messageCount != goroutines {
		t.Errorf("expected %d goroutine messages, found %d", goroutines, messageCount)
	}
}

func TestLaunchLogger_CloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	os.MkdirAll(fleetDir, 0755)

	l := NewLaunchLogger(fleetDir)

	// Closing multiple times should not panic
	l.Close()
	l.Close()
}
