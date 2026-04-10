package driver

import (
	"strings"
	"testing"
)

func TestClaudeCodeBuildCommand(t *testing.T) {
	d := &ClaudeCodeDriver{}

	t.Run("normal mode", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/prompt.txt",
			YoloMode:   false,
		})
		if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
			t.Errorf("script does not start with shebang, got: %q", script[:min(40, len(script))])
		}
		if !strings.Contains(script, "exec claude") {
			t.Errorf("script does not contain 'exec claude': %q", script)
		}
		if strings.Contains(script, "--dangerously-skip-permissions") {
			t.Errorf("normal mode script should not contain --dangerously-skip-permissions")
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
		if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
			t.Errorf("script does not start with shebang, got: %q", script[:min(40, len(script))])
		}
		if !strings.Contains(script, "--dangerously-skip-permissions") {
			t.Errorf("yolo mode script should contain --dangerously-skip-permissions")
		}
	})
}

func TestClaudeCodeDetectState(t *testing.T) {
	d := &ClaudeCodeDriver{}

	cases := []struct {
		name        string
		fullContent string
		expected    AgentState
	}{
		{
			name:        "esc to interrupt",
			fullContent: "Claude is thinking...\nesc to interrupt",
			expected:    StateWorking,
		},
		{
			name:        "spinner char",
			fullContent: "Processing ⠋",
			expected:    StateWorking,
		},
		{
			name:        "Esc to cancel",
			fullContent: "Do you want to run this?\nEsc to cancel",
			expected:    StateWaiting,
		},
		{
			name:        "Do you want to proceed",
			fullContent: "Do you want to proceed",
			expected:    StateWaiting,
		},
		{
			name:        "accept edits",
			fullContent: "Review the changes\naccept edits",
			expected:    StateWaiting,
		},
		{
			name:        "shift+tab to cycle",
			fullContent: "Select an option\nshift+tab to cycle",
			expected:    StateWaiting,
		},
		{
			name:        "bare arrow prompt",
			fullContent: "Some output\n❯",
			expected:    StateWaiting,
		},
		{
			name:        "question in last 3 lines",
			fullContent: "line1\nline2\nShould I continue with this approach?",
			expected:    StateWaiting,
		},
		{
			name:        "y/n prompt",
			fullContent: "Delete the file? (y/n)",
			expected:    StateWaiting,
		},
		{
			name:        "Y/n prompt",
			fullContent: "Delete the file? [Y/n]",
			expected:    StateWaiting,
		},
		{
			name:        "empty content",
			fullContent: "",
			expected:    StateStarting,
		},
		{
			name:        "whitespace only",
			fullContent: "   \n\n   ",
			expected:    StateStarting,
		},
		{
			name:        "no matching patterns",
			fullContent: "Claude finished the task.",
			expected:    StateWorking,
		},
		{
			name:        "waiting beats working",
			fullContent: "esc to interrupt\n(y/n)",
			expected:    StateWaiting,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Split into lines, collect non-empty lines (bottom 15)
			allLines := strings.Split(tc.fullContent, "\n")
			var nonEmpty []string
			for _, line := range allLines {
				if strings.TrimSpace(line) != "" {
					nonEmpty = append(nonEmpty, line)
				}
			}
			bottomLines := lastN(nonEmpty, 15)

			result := d.DetectState(bottomLines, tc.fullContent)
			if result == nil {
				t.Fatal("DetectState returned nil")
			}
			if *result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, *result)
			}
		})
	}
}

func TestClaudeCodeInteractiveCommand(t *testing.T) {
	d := &ClaudeCodeDriver{}
	cmd := d.InteractiveCommand()
	if len(cmd) != 1 || cmd[0] != "claude" {
		t.Errorf("expected [\"claude\"], got %v", cmd)
	}
}

func TestClaudeCodeCheckAvailable(t *testing.T) {
	d := &ClaudeCodeDriver{}
	// Just verify it doesn't panic; claude may or may not be installed
	_ = d.CheckAvailable()
}

func TestClaudeCodeName(t *testing.T) {
	d := &ClaudeCodeDriver{}
	if d.Name() != "claude-code" {
		t.Errorf("expected Name() to return 'claude-code', got %q", d.Name())
	}
}

func TestGetDriver(t *testing.T) {
	t.Run("empty string returns claude-code", func(t *testing.T) {
		d, err := Get("")
		if err != nil {
			t.Fatalf("Get('') returned error: %v", err)
		}
		if d.Name() != "claude-code" {
			t.Errorf("expected 'claude-code', got %q", d.Name())
		}
	})

	t.Run("explicit claude-code", func(t *testing.T) {
		d, err := Get("claude-code")
		if err != nil {
			t.Fatalf("Get('claude-code') returned error: %v", err)
		}
		if d.Name() != "claude-code" {
			t.Errorf("expected 'claude-code', got %q", d.Name())
		}
	})

	t.Run("nonexistent returns error", func(t *testing.T) {
		_, err := Get("nonexistent")
		if err == nil {
			t.Error("Get('nonexistent') should return an error")
		}
	})
}

func TestAvailable(t *testing.T) {
	names := Available()
	found := false
	for _, name := range names {
		if name == "claude-code" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Available() should include 'claude-code', got: %v", names)
	}
}
