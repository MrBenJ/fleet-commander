package driver

import (
	"strings"
	"testing"
)

func TestCodexName(t *testing.T) {
	d := &CodexDriver{}
	if d.Name() != "codex" {
		t.Errorf("expected Name() to return 'codex', got %q", d.Name())
	}
}

func TestCodexBuildCommand(t *testing.T) {
	d := &CodexDriver{}

	t.Run("normal mode", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/prompt.txt",
			YoloMode:   false,
		})
		if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
			t.Errorf("script does not start with shebang: %q", script[:min(40, len(script))])
		}
		if !strings.Contains(script, "exec codex") {
			t.Errorf("script does not contain 'exec codex': %q", script)
		}
		if !strings.Contains(script, "--ask-for-approval on-request") {
			t.Errorf("normal mode should use --ask-for-approval on-request: %q", script)
		}
		if !strings.Contains(script, "--sandbox workspace-write") {
			t.Errorf("normal mode should use --sandbox workspace-write: %q", script)
		}
		if strings.Contains(script, "--dangerously-bypass-approvals-and-sandbox") {
			t.Errorf("normal mode should not bypass approvals and sandbox: %q", script)
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
		if !strings.Contains(script, "--dangerously-bypass-approvals-and-sandbox") {
			t.Errorf("yolo mode should bypass approvals and sandbox: %q", script)
		}
		if strings.Contains(script, "--ask-for-approval") {
			t.Errorf("yolo mode should not configure approval prompts: %q", script)
		}
		if strings.Contains(script, "--sandbox workspace-write") {
			t.Errorf("yolo mode should not configure workspace-write sandbox: %q", script)
		}
	})
}

func TestCodexBuildCommand_PromptFileEscaping(t *testing.T) {
	d := &CodexDriver{}

	t.Run("path with spaces", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/my prompts/task file.txt",
		})
		if !strings.Contains(script, "/tmp/my prompts/task file.txt") {
			t.Errorf("script should contain the full path (quoted by fmt): %q", script)
		}
	})

	t.Run("path with special chars", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: `/tmp/prompt's "file".txt`,
		})
		// fmt.Sprintf %q will escape these
		if !strings.Contains(script, "prompt") {
			t.Errorf("script should handle special characters: %q", script)
		}
	})
}

func TestCodexDetectState_Waiting(t *testing.T) {
	d := &CodexDriver{}

	cases := []struct {
		name        string
		fullContent string
	}{
		{"Y/n prompt", "Do you want to apply? [Y/n]"},
		{"y/N prompt", "Confirm changes [y/N]"},
		{"Accept prompt", "Accept?"},
		{"sandbox prompt", "sandbox$ "},
		{"> prompt on own line", "some output\n>"},
		{"question line", "What would you like me to do next?"},
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

func TestCodexDetectState_Working(t *testing.T) {
	d := &CodexDriver{}

	cases := []struct {
		name        string
		fullContent string
	}{
		{"Running status", "Running..."},
		{"Executing status", "Executing..."},
		{"Reading status", "Reading file.go"},
		{"Writing status", "Writing output.txt"},
		{"Searching status", "Searching for patterns"},
		{"spinner", "Processing ⠋"},
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

func TestCodexDetectState_Unknown(t *testing.T) {
	d := &CodexDriver{}

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

func TestCodexDetectState_Empty(t *testing.T) {
	d := &CodexDriver{}
	result := d.DetectState(nil, "")
	if result == nil {
		t.Fatal("DetectState returned nil for empty content, expected starting")
	}
	if *result != StateStarting {
		t.Errorf("expected starting for empty content, got %q", *result)
	}
}

func TestCodexCheckAvailable(t *testing.T) {
	d := &CodexDriver{}
	err := d.CheckAvailable()
	// codex may or may not be installed — just verify no panic
	// and that a missing codex gives a helpful error
	if err != nil {
		if !strings.Contains(err.Error(), "npm i -g @openai/codex") {
			t.Errorf("error should contain install hint, got: %v", err)
		}
	}
}

func TestCodexInteractiveCommand(t *testing.T) {
	d := &CodexDriver{}
	cmd := d.InteractiveCommand()
	if len(cmd) != 1 || cmd[0] != "codex" {
		t.Errorf("expected [\"codex\"], got %v", cmd)
	}
}

func TestCodexRegistered(t *testing.T) {
	d, err := Get("codex")
	if err != nil {
		t.Fatalf("Get('codex') returned error: %v", err)
	}
	if d.Name() != "codex" {
		t.Errorf("expected 'codex', got %q", d.Name())
	}
}

// nonEmptyLines splits content into non-empty lines (simulating bottom pane lines).
func nonEmptyLines(content string) []string {
	var result []string
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}
