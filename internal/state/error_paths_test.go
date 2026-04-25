package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/state"
)

// TestWriteFailsWhenParentIsAFile verifies that Write returns an error
// when the directory cannot be created because its parent is a regular file.
// This exercises the os.MkdirAll error path.
func TestWriteFailsWhenParentIsAFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file where Write expects a directory ancestor.
	parentAsFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(parentAsFile, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to write a state file under that "directory" — MkdirAll should fail.
	target := filepath.Join(parentAsFile, "subdir", "state.json")
	if err := state.Write(target, "agent", "waiting"); err == nil {
		t.Fatal("expected error when parent is a file, got nil")
	}
}

// TestWriteFailsOnReadOnlyDir verifies that Write returns an error when
// the target directory is read-only and the temp file cannot be created.
// Skipped when running as root since root bypasses permissions.
func TestWriteFailsOnReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — read-only dir restrictions don't apply")
	}

	tmpDir := t.TempDir()
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(roDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(roDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0755) })

	target := filepath.Join(roDir, "state.json")
	if err := state.Write(target, "agent", "waiting"); err == nil {
		t.Error("expected error writing to read-only dir, got nil")
	}
}
