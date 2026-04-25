package driver

import (
	"strings"
	"testing"
)

func TestKimiCodeName(t *testing.T) {
	d := &KimiCodeDriver{}
	if d.Name() != "kimi-code" {
		t.Errorf("expected Name() to return 'kimi-code', got %q", d.Name())
	}
}

func TestKimiCodeInteractiveCommand(t *testing.T) {
	d := &KimiCodeDriver{}
	cmd := d.InteractiveCommand()
	if len(cmd) != 1 || cmd[0] != "kimi" {
		t.Errorf("expected [\"kimi\"], got %v", cmd)
	}
}

func TestKimiCodeBuildCommand(t *testing.T) {
	d := &KimiCodeDriver{}

	t.Run("normal mode", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/prompt.txt",
			YoloMode:   false,
		})
		if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
			t.Errorf("script does not start with shebang: %q", script[:min(40, len(script))])
		}
		if !strings.Contains(script, "exec kimi") {
			t.Errorf("script does not contain 'exec kimi': %q", script)
		}
		if strings.Contains(script, "--yolo") {
			t.Errorf("normal mode should not contain --yolo: %q", script)
		}
		if !strings.Contains(script, "-p \"$prompt\"") {
			t.Errorf("script should pass prompt via -p: %q", script)
		}
		if !strings.Contains(script, "/tmp/prompt.txt") {
			t.Errorf("script does not contain prompt file path: %q", script)
		}
	})

	t.Run("yolo mode", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/prompt.txt",
			YoloMode:   true,
		})
		if !strings.Contains(script, "--yolo") {
			t.Errorf("yolo mode should contain --yolo: %q", script)
		}
		// --yolo must come before -p so the prompt isn't consumed by --yolo
		yoloIdx := strings.Index(script, "--yolo")
		promptIdx := strings.Index(script, "-p \"$prompt\"")
		if yoloIdx < 0 || promptIdx < 0 || yoloIdx > promptIdx {
			t.Errorf("--yolo should appear before -p: %q", script)
		}
	})
}

func TestKimiCodeBuildCommand_PromptFileEscaping(t *testing.T) {
	d := &KimiCodeDriver{}

	t.Run("path with spaces", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/my prompts/task file.txt",
		})
		if !strings.Contains(script, "/tmp/my prompts/task file.txt") {
			t.Errorf("script should contain the full path (quoted by fmt %%q): %q", script)
		}
	})

	t.Run("path with special chars", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: `/tmp/prompt's "file".txt`,
		})
		if !strings.Contains(script, "prompt") {
			t.Errorf("script should handle special characters: %q", script)
		}
	})
}

func TestKimiCodeDetectState_Waiting(t *testing.T) {
	d := &KimiCodeDriver{}

	cases := []struct {
		name        string
		fullContent string
	}{
		{"Y/n prompt", "Apply changes? [Y/n]"},
		{"y/N prompt", "Continue? [y/N]"},
		{"❯ prompt on own line", "some output\n❯"},
		{"question line", "What would you like me to do next?"},
		// Real kimi 1.39.0 capture: shell-approval modal.
		{"shell approval modal", "╭─ approval ─╮\nShell is requesting approval to run command:\nfind internal -type d | sort\n→ [1] Approve once\n  [2] Approve for this session\n  [3] Reject\n  [4] Reject, tell the model what to do instead"},
		// Even when approval text is the only signal, just the option label is enough.
		{"approve once option visible", "→ [1] Approve once"},
		{"reject-tell-the-model option", "[4] Reject, tell the model what to do instead"},
		// Idle at the kimi input prompt — the input-help line is the signal.
		{"idle input help line", "agent (Kimi-k2.6 ●)  ~/repo  test/kimi  ctrl-j: newline | /feedback: send feedback"},
		// Real kimi 1.39.0 capture: truly idle pane (no approval, no spinner).
		{"idle input divider", "Some prior response text\n── input ──\n\nagent (Kimi-k2.6 ●)  ~/repo  test/kimi  ctrl-x: toggle mode | shift-tab: plan mode\n                                                                                     context: 6.6% (17.3k/262.1k)"},
		{"idle ctrl-x toggle mode", "agent (Kimi-k2.6 ●)  test/kimi  ctrl-x: toggle mode | shift-tab: plan mode"},
		{"idle shift-tab plan mode", "shift-tab: plan mode"},
		// Approval modal wins even when a subagent spinner is also present.
		{"approval beats spinner", "⠏ Using Agent (subagent)\nShell is requesting approval to run command:"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bottomLines := nonEmptyLines(tc.fullContent)
			result := d.DetectState(bottomLines, tc.fullContent)
			if result == nil {
				t.Fatal("DetectState returned nil, expected waiting")
			}
			if *result != StateWaiting {
				t.Errorf("expected waiting, got %q", *result)
			}
		})
	}
}

func TestKimiCodeDetectState_Working(t *testing.T) {
	d := &KimiCodeDriver{}

	cases := []struct {
		name        string
		fullContent string
	}{
		{"spinner", "Processing ⠋"},
		{"plan-mode glyph", "📋 planning the change"},
		// Real kimi 1.39.0 capture: subagent dispatch.
		{"subagent dispatch", "⠏ Using Agent (Review Go code quality)\n  • subagent coder (a2b5f4a08)"},
		{"using agent web", "⠏ Using Agent (Review web frontend code)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bottomLines := nonEmptyLines(tc.fullContent)
			result := d.DetectState(bottomLines, tc.fullContent)
			if result == nil {
				t.Fatal("DetectState returned nil, expected working")
			}
			if *result != StateWorking {
				t.Errorf("expected working, got %q", *result)
			}
		})
	}
}

func TestKimiCodeDetectState_Unknown(t *testing.T) {
	d := &KimiCodeDriver{}

	cases := []struct {
		name        string
		fullContent string
	}{
		{"random output", "some random text that matches nothing"},
		{"finished message", "Done. All changes applied."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bottomLines := nonEmptyLines(tc.fullContent)
			result := d.DetectState(bottomLines, tc.fullContent)
			if result != nil {
				t.Errorf("expected nil for unknown content, got %q", *result)
			}
		})
	}
}

func TestKimiCodeDetectState_Empty(t *testing.T) {
	d := &KimiCodeDriver{}
	result := d.DetectState(nil, "")
	if result == nil {
		t.Fatal("DetectState returned nil for empty content, expected starting")
	}
	if *result != StateStarting {
		t.Errorf("expected starting for empty content, got %q", *result)
	}
}

func TestKimiCodeCheckAvailable(t *testing.T) {
	d := &KimiCodeDriver{}
	err := d.CheckAvailable()
	if err != nil {
		if !strings.Contains(err.Error(), "kimi.com/code/docs") {
			t.Errorf("error should contain install hint URL, got: %v", err)
		}
	}
}

func TestKimiCodeRegistered(t *testing.T) {
	d, err := Get("kimi-code")
	if err != nil {
		t.Fatalf("Get('kimi-code') returned error: %v", err)
	}
	if d.Name() != "kimi-code" {
		t.Errorf("expected 'kimi-code', got %q", d.Name())
	}
}
