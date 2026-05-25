package driver

import (
	"strings"
	"testing"
)

func TestAntigravityName(t *testing.T) {
	d := &AntigravityDriver{}
	if d.Name() != "antigravity" {
		t.Errorf("expected Name() to return 'antigravity', got %q", d.Name())
	}
}

func TestAntigravityInteractiveCommand(t *testing.T) {
	d := &AntigravityDriver{}
	cmd := d.InteractiveCommand()
	if len(cmd) != 1 || cmd[0] != "agy" {
		t.Errorf("expected [\"agy\"], got %v", cmd)
	}
}

func TestAntigravityBuildCommand(t *testing.T) {
	d := &AntigravityDriver{}

	t.Run("normal mode seeds interactive agy with the prompt", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt", YoloMode: false})
		if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
			t.Errorf("script does not start with shebang: %q", script)
		}
		if !strings.Contains(script, "exec agy") {
			t.Errorf("script does not exec agy: %q", script)
		}
		if !strings.Contains(script, "/tmp/prompt.txt") {
			t.Errorf("script does not contain prompt file path: %q", script)
		}
	})

	// Intentional non-YOLO limitation (see BuildCommand): agy has no
	// permission-bypass flag, so yolo must NOT emit one. Emitting a
	// nonexistent flag like --dangerously-skip-permissions would make agy
	// reject it and fail to launch, so this guard is a real safety check, not
	// a stale assumption. Revisit only if Antigravity ships a documented flag.
	t.Run("yolo mode emits no bypass flag (agy has none)", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt", YoloMode: true})
		if !strings.Contains(script, "exec agy") {
			t.Errorf("yolo script does not exec agy: %q", script)
		}
		if strings.Contains(script, "--dangerously") || strings.Contains(script, "--yolo") {
			t.Errorf("agy has no documented bypass flag; none should be emitted: %q", script)
		}
	})
}

func TestAntigravityDetectState_Waiting(t *testing.T) {
	d := &AntigravityDriver{}
	cases := []struct {
		name        string
		fullContent string
	}{
		{"Y/n prompt", "Apply these changes? [Y/n]"},
		{"y/N prompt", "Run command [y/N]"},
		{"(y/n) prompt", "Proceed (y/n)"},
		{"requesting approval", "agy is requesting approval to run a command"},
		{"Approve option", "Approve once"},
		{"> prompt on own line", "some output\n>"},
		{"question line", "What would you like me to do next?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := d.DetectState(nonEmptyLines(tc.fullContent), tc.fullContent)
			if result == nil {
				t.Fatal("DetectState returned nil, expected waiting")
			}
			if *result != StateWaiting {
				t.Errorf("expected waiting, got %q", *result)
			}
		})
	}
}

func TestAntigravityDetectState_Working(t *testing.T) {
	d := &AntigravityDriver{}
	cases := []struct {
		name        string
		fullContent string
	}{
		{"esc to interrupt", "Thinking... (esc to interrupt)"},
		{"spinner", "Working ⠋"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := d.DetectState(nonEmptyLines(tc.fullContent), tc.fullContent)
			if result == nil {
				t.Fatal("DetectState returned nil, expected working")
			}
			if *result != StateWorking {
				t.Errorf("expected working, got %q", *result)
			}
		})
	}
}

func TestAntigravityDetectState_Unknown(t *testing.T) {
	d := &AntigravityDriver{}
	cases := []struct {
		name        string
		fullContent string
	}{
		{"random output", "some random text that matches nothing"},
		{"finished message", "Done. All changes applied."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := d.DetectState(nonEmptyLines(tc.fullContent), tc.fullContent)
			if result != nil {
				t.Errorf("expected nil for unknown content, got %q", *result)
			}
		})
	}
}

func TestAntigravityDetectState_Empty(t *testing.T) {
	d := &AntigravityDriver{}
	result := d.DetectState(nil, "")
	if result == nil {
		t.Fatal("DetectState returned nil for empty content, expected starting")
	}
	if *result != StateStarting {
		t.Errorf("expected starting for empty content, got %q", *result)
	}
}

func TestAntigravityHooksAreNoOps(t *testing.T) {
	d := &AntigravityDriver{}
	if err := d.InjectHooks("/tmp/whatever"); err != nil {
		t.Errorf("InjectHooks should be a no-op, got %v", err)
	}
	if err := d.RemoveHooks("/tmp/whatever"); err != nil {
		t.Errorf("RemoveHooks should be a no-op, got %v", err)
	}
}

func TestAntigravityCheckAvailable(t *testing.T) {
	d := &AntigravityDriver{}
	err := d.CheckAvailable()
	// agy may or may not be installed — just verify a missing agy gives a
	// helpful install hint and nothing panics.
	if err != nil {
		if !strings.Contains(err.Error(), "antigravity.google/cli/install.sh") {
			t.Errorf("error should contain install hint, got: %v", err)
		}
	}
}

func TestAntigravityRegistered(t *testing.T) {
	d, err := Get("antigravity")
	if err != nil {
		t.Fatalf("Get('antigravity') returned error: %v", err)
	}
	if d.Name() != "antigravity" {
		t.Errorf("expected 'antigravity', got %q", d.Name())
	}
}

func TestAntigravityInAvailable(t *testing.T) {
	found := false
	for _, name := range Available() {
		if name == "antigravity" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'antigravity' in Available(), got %v", Available())
	}
}
