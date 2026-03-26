package tui

import (
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
	}{
		{
			name:  "simple prompt",
			input: "Fix the login bug",
			want:  "fix-login-bug",
		},
		{
			name:  "removes stop words",
			input: "Add a new feature to the user authentication system",
			want:  "add-new-feature-user-authentic",
		},
		{
			name:  "handles special characters",
			input: "Fix bug #123 in auth/login!",
			want:  "fix-bug-123-auth-login",
		},
		{
			name:  "truncates long slugs",
			input: "Refactor database connection pooling management system for better performance",
			want:  "refactor-database-connection-p",
		},
		{
			name:  "single word",
			input: "refactor",
			want:  "refactor",
		},
		{
			name:  "all stop words",
			input: "the a an to for",
			want:  "",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "numbers preserved",
			input: "Add OAuth2 support",
			want:  "add-oauth2-support",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePrompts(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCount  int
		wantFirst  string
		wantSecond string
	}{
		{
			name:       "numbered list with dots",
			input:      "1. Fix login bug\n2. Add OAuth support\n3. Refactor DB layer",
			wantCount:  3,
			wantFirst:  "Fix login bug",
			wantSecond: "Add OAuth support",
		},
		{
			name:       "numbered list with parens",
			input:      "1) Fix login bug\n2) Add OAuth support",
			wantCount:  2,
			wantFirst:  "Fix login bug",
			wantSecond: "Add OAuth support",
		},
		{
			name:       "bullet list with dashes",
			input:      "- Fix login bug\n- Add OAuth support\n- Refactor DB",
			wantCount:  3,
			wantFirst:  "Fix login bug",
			wantSecond: "Add OAuth support",
		},
		{
			name:       "bullet list with asterisks",
			input:      "* Fix login bug\n* Add OAuth support",
			wantCount:  2,
			wantFirst:  "Fix login bug",
			wantSecond: "Add OAuth support",
		},
		{
			name:       "plain newlines",
			input:      "Fix login bug\nAdd OAuth support\nRefactor DB",
			wantCount:  3,
			wantFirst:  "Fix login bug",
			wantSecond: "Add OAuth support",
		},
		{
			name:      "single prompt",
			input:     "Fix login bug",
			wantCount: 1,
			wantFirst: "Fix login bug",
		},
		{
			name:       "empty lines skipped",
			input:      "Fix login bug\n\n\nAdd OAuth support\n\n",
			wantCount:  2,
			wantFirst:  "Fix login bug",
			wantSecond: "Add OAuth support",
		},
		{
			name:      "empty input",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "whitespace only",
			input:     "  \n  \n  ",
			wantCount: 0,
		},
		{
			name:       "mixed markers",
			input:      "1. Fix login\n- Add auth\n* Refactor DB",
			wantCount:  3,
			wantFirst:  "Fix login",
			wantSecond: "Add auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := ParsePrompts(tt.input)
			if len(items) != tt.wantCount {
				t.Fatalf("ParsePrompts() returned %d items, want %d", len(items), tt.wantCount)
			}
			if tt.wantCount > 0 && items[0].Prompt != tt.wantFirst {
				t.Errorf("first prompt = %q, want %q", items[0].Prompt, tt.wantFirst)
			}
			if tt.wantCount > 1 && items[1].Prompt != tt.wantSecond {
				t.Errorf("second prompt = %q, want %q", items[1].Prompt, tt.wantSecond)
			}
		})
	}
}

func TestParsePromptsGeneratesNames(t *testing.T) {
	items := ParsePrompts("1. Fix login bug\n2. Add OAuth support")

	if items[0].AgentName == "" {
		t.Error("first item has empty agent name")
	}
	if items[0].Branch == "" {
		t.Error("first item has empty branch")
	}
	if items[0].Branch != "fleet/"+items[0].AgentName {
		t.Errorf("branch %q doesn't match expected fleet/%s", items[0].Branch, items[0].AgentName)
	}
}

func TestGenerateNamesDeduplication(t *testing.T) {
	existing := []string{"fix-login-bug"}

	name, branch := GenerateNames("Fix login bug", existing)
	if name != "fix-login-bug-2" {
		t.Errorf("expected deduplicated name fix-login-bug-2, got %q", name)
	}
	if branch != "fleet/fix-login-bug-2" {
		t.Errorf("expected branch fleet/fix-login-bug-2, got %q", branch)
	}

	// Triple dedup
	existing = append(existing, "fix-login-bug-2")
	name, _ = GenerateNames("Fix login bug", existing)
	if name != "fix-login-bug-3" {
		t.Errorf("expected fix-login-bug-3, got %q", name)
	}
}

func TestGenerateNamesEmptySlug(t *testing.T) {
	name, branch := GenerateNames("the a an", nil)
	if name != "agent" {
		t.Errorf("expected fallback name 'agent', got %q", name)
	}
	if branch != "fleet/agent" {
		t.Errorf("expected branch 'fleet/agent', got %q", branch)
	}
}
