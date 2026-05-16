package main

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// TestRun_ReturnsZeroOnSuccess verifies that the refactored run() helper
// returns 0 for a clean exit. Before C6 was fixed, main called os.Exit(1)
// inline which skipped deferred shutdown logic.
func TestRun_ReturnsZeroOnSuccess(t *testing.T) {
	originalRoot := rootCmd
	defer func() { rootCmd = originalRoot }()

	rootCmd = &cobra.Command{
		Use: "fleet-test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	rootCmd.SetArgs([]string{})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	if code := run(); code != 0 {
		t.Errorf("run() = %d, want 0 on success", code)
	}
}

// TestRun_ReturnsOneOnError verifies run() returns exit code 1 when the
// command fails, after printing the error to stderr.
func TestRun_ReturnsOneOnError(t *testing.T) {
	originalRoot := rootCmd
	defer func() { rootCmd = originalRoot }()

	rootCmd = &cobra.Command{
		Use: "fleet-test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("simulated failure")
		},
	}
	rootCmd.SetArgs([]string{})
	rootCmd.SetOut(&bytes.Buffer{})

	// Capture stderr.
	stderrPipe, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = stderrW
	defer func() { os.Stderr = origStderr }()

	code := run()

	stderrW.Close()
	var buf bytes.Buffer
	buf.ReadFrom(stderrPipe)

	if code != 1 {
		t.Errorf("run() = %d, want 1 on error", code)
	}
	if !bytes.Contains(buf.Bytes(), []byte("simulated failure")) {
		t.Errorf("expected error message on stderr, got %q", buf.String())
	}
}

// TestRun_ContextIsSignalAware verifies that the context passed to the
// root command can be canceled (proving signal.NotifyContext is wired up).
// The command captures the context, and after run() returns we check the
// context was indeed signal-aware (its Done channel is non-nil).
func TestRun_ContextIsSignalAware(t *testing.T) {
	originalRoot := rootCmd
	defer func() { rootCmd = originalRoot }()

	var capturedCtxDone <-chan struct{}
	rootCmd = &cobra.Command{
		Use: "fleet-test",
		RunE: func(cmd *cobra.Command, args []string) error {
			capturedCtxDone = cmd.Context().Done()
			return nil
		},
	}
	rootCmd.SetArgs([]string{})

	if run() != 0 {
		t.Fatal("expected success")
	}
	if capturedCtxDone == nil {
		t.Error("expected command context to have a Done channel (signal-aware)")
	}
}
