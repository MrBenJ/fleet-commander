package execx

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCommandRunnerOutput(t *testing.T) {
	runner := NewRunner(time.Second)

	out, err := runner.Output(context.Background(), Options{
		Name: "go",
		Args: []string{"env", "GOVERSION"},
	})
	if err != nil {
		t.Fatalf("Output() error: %v", err)
	}
	if !strings.HasPrefix(string(out), "go") {
		t.Fatalf("expected go version output, got %q", out)
	}
}

func TestCommandRunnerTimeout(t *testing.T) {
	runner := NewRunner(10 * time.Millisecond)

	err := runner.Run(context.Background(), Options{
		Name: "sleep",
		Args: []string{"1"},
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestCommandRunnerMissingName(t *testing.T) {
	runner := NewRunner(time.Second)

	err := runner.Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "command name is required") {
		t.Fatalf("expected missing name error, got %v", err)
	}
}
