package monitor

import (
	"strings"
	"testing"
)

func TestDetectState(t *testing.T) {
	tests := []struct {
		name        string
		fullContent string
		want        AgentState
	}{
		// Working patterns
		{
			name:        "esc to interrupt",
			fullContent: "Claude is thinking...\nesc to interrupt",
			want:        StateWorking,
		},
		{
			name:        "spinner char braille 1",
			fullContent: "Processing ⠋",
			want:        StateWorking,
		},
		{
			name:        "spinner char braille 2",
			fullContent: "Processing ⠹",
			want:        StateWorking,
		},

		// Waiting patterns
		{
			name:        "Esc to cancel",
			fullContent: "Do you want to run this?\nEsc to cancel",
			want:        StateWaiting,
		},
		{
			name:        "Do you want to proceed",
			fullContent: "Do you want to proceed",
			want:        StateWaiting,
		},
		{
			name:        "accept edits",
			fullContent: "Review the changes\naccept edits",
			want:        StateWaiting,
		},
		{
			name:        "shift+tab to cycle",
			fullContent: "Select an option\nshift+tab to cycle",
			want:        StateWaiting,
		},
		{
			name:        "numbered menu with arrow",
			fullContent: "❯ 1. Option one\n  2. Option two",
			want:        StateWaiting,
		},
		{
			name:        "bare arrow prompt",
			fullContent: "Some output\n❯",
			want:        StateWaiting,
		},
		{
			name:        "question ending in last 3 lines",
			fullContent: "line1\nline2\nline3\nline4\nShould I continue with this approach?",
			want:        StateWaiting,
		},
		{
			name:        "y/n prompt",
			fullContent: "Delete the file? (y/n)",
			want:        StateWaiting,
		},
		{
			name:        "Y/n prompt",
			fullContent: "Delete the file? [Y/n]",
			want:        StateWaiting,
		},

		// Edge cases
		{
			name:        "empty content returns StateStarting",
			fullContent: "",
			want:        StateStarting,
		},
		{
			name:        "whitespace-only content returns StateStarting",
			fullContent: "   \n\n   ",
			want:        StateStarting,
		},
		{
			name:        "no matching patterns returns StateWorking",
			fullContent: "Claude finished the task.",
			want:        StateWorking,
		},
		{
			name:        "ansi-wrapped working pattern is stripped before matching",
			fullContent: "\x1b[32mesc to interrupt\x1b[0m",
			want:        StateWorking,
		},
		{
			name:        "short question line does NOT trigger waiting",
			fullContent: "Is it?",
			want:        StateWorking, // len("Is it?") == 6, ≤ 10 → no match
		},
		{
			name: "question in 4th-from-last line does NOT trigger waiting",
			// last 3 non-empty: "line5", "line6", "line7"
			// question is at 4th from the end
			fullContent: "Should I continue with this approach?\nline5\nline6\nline7",
			want:        StateWorking,
		},
		{
			name:        "waiting pattern beats working pattern (waiting checked first)",
			fullContent: "esc to interrupt\n(y/n)",
			want:        StateWaiting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastLine := getLastNonEmptyLine(tt.fullContent)
			got := detectState(lastLine, tt.fullContent)
			if got != tt.want {
				t.Errorf("detectState(%q) = %q, want %q", tt.fullContent, got, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain string unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "reset sequence removed",
			input: "before\x1b[0mafter",
			want:  "beforeafter",
		},
		{
			name:  "color sequence removed text preserved",
			input: "\x1b[32mgreen text\x1b[0m",
			want:  "green text",
		},
		{
			name:  "multiple sequences",
			input: "\x1b[1m\x1b[32mbold green\x1b[0m",
			want:  "bold green",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetLastNonEmptyLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  []string
	}{
		{
			name:  "returns last N non-empty",
			input: "line1\nline2\nline3\nline4\nline5",
			n:     3,
			want:  []string{"line5", "line4", "line3"},
		},
		{
			name:  "skips trailing empty lines",
			input: "line1\nline2\nline3\n\n\n",
			n:     2,
			want:  []string{"line3", "line2"},
		},
		{
			name:  "returns fewer than N when not enough non-empty lines",
			input: "line1\nline2",
			n:     5,
			want:  []string{"line2", "line1"},
		},
		{
			name:  "empty input returns empty slice",
			input: "",
			n:     3,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.input, "\n")
			got := getLastNonEmptyLines(lines, tt.n)
			if len(got) != len(tt.want) {
				t.Fatalf("getLastNonEmptyLines(%q, %d) = %v (len %d), want %v (len %d)",
					tt.input, tt.n, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
