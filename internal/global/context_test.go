//go:build !windows

package global

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAppendGlobalLogConcurrent(t *testing.T) {
	setupTestHome(t)

	const goroutines = 10
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := AppendGlobalLog("repo", "agent", "concurrent message")
			if err != nil {
				t.Errorf("goroutine %d: AppendGlobalLog failed: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	ctx, err := LoadGlobalContext()
	if err != nil {
		t.Fatalf("LoadGlobalContext failed: %v", err)
	}
	if len(ctx.Log) != goroutines {
		t.Errorf("expected %d log entries after concurrent writes, got %d", goroutines, len(ctx.Log))
	}
}

func TestLoadGlobalContextMalformedJSON(t *testing.T) {
	home := setupTestHome(t)

	dir := filepath.Join(home, ".fleet")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write malformed JSON
	path := filepath.Join(dir, "context.json")
	if err := os.WriteFile(path, []byte("{not valid json!!!"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadGlobalContext()
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestSaveAndLoadGlobalContext(t *testing.T) {
	home := setupTestHome(t)

	dir := filepath.Join(home, ".fleet")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	ctx := &GlobalContext{
		Shared: "test shared data",
		Log: []GlobalLogEntry{
			{Repo: "myrepo", Agent: "agent1", Message: "hello"},
		},
	}

	if err := saveGlobalContext(dir, ctx); err != nil {
		t.Fatalf("saveGlobalContext failed: %v", err)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(filepath.Join(dir, "context.json"))
	if err != nil {
		t.Fatalf("failed to read context.json: %v", err)
	}

	var loaded GlobalContext
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse saved context: %v", err)
	}

	if loaded.Shared != "test shared data" {
		t.Errorf("shared = %q, want %q", loaded.Shared, "test shared data")
	}
	if len(loaded.Log) != 1 || loaded.Log[0].Message != "hello" {
		t.Errorf("unexpected log entries: %+v", loaded.Log)
	}
}

func TestAppendGlobalLogSetsTimestamp(t *testing.T) {
	setupTestHome(t)

	if err := AppendGlobalLog("repo", "agent", "timed message"); err != nil {
		t.Fatalf("AppendGlobalLog failed: %v", err)
	}

	ctx, err := LoadGlobalContext()
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(ctx.Log))
	}
	if ctx.Log[0].Timestamp.IsZero() {
		t.Error("timestamp should be set, got zero value")
	}
	if ctx.Log[0].Repo != "repo" {
		t.Errorf("repo = %q, want %q", ctx.Log[0].Repo, "repo")
	}
	if ctx.Log[0].Agent != "agent" {
		t.Errorf("agent = %q, want %q", ctx.Log[0].Agent, "agent")
	}
}

func TestTrimGlobalLogToZero(t *testing.T) {
	setupTestHome(t)

	for i := 0; i < 5; i++ {
		if err := AppendGlobalLog("repo", "agent", "msg"); err != nil {
			t.Fatal(err)
		}
	}

	if err := TrimGlobalLog(0); err != nil {
		t.Fatalf("TrimGlobalLog(0) failed: %v", err)
	}

	ctx, err := LoadGlobalContext()
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Log) != 0 {
		t.Errorf("expected 0 entries after trimming to 0, got %d", len(ctx.Log))
	}
}

func TestTrimGlobalLogConcurrent(t *testing.T) {
	setupTestHome(t)

	// Seed with some data
	for i := 0; i < 20; i++ {
		if err := AppendGlobalLog("repo", "agent", "msg"); err != nil {
			t.Fatal(err)
		}
	}

	// Concurrently trim and append
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = TrimGlobalLog(5)
		}()
		go func() {
			defer wg.Done()
			_ = AppendGlobalLog("repo", "agent", "new msg")
		}()
	}
	wg.Wait()

	// Just verify it's in a consistent state (no corruption)
	ctx, err := LoadGlobalContext()
	if err != nil {
		t.Fatalf("LoadGlobalContext failed after concurrent ops: %v", err)
	}
	// We can't predict the exact count due to race ordering, but it should be valid
	if ctx.Log == nil {
		t.Error("log should not be nil")
	}
}
