package tui

import (
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"exactly max", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "hello..."},
		{"empty string", "", 10, ""},
		{"zero max", "hello", 0, "..."},
		{"one char max", "hello", 1, "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestBuildMetaPrompt_NoExistingAgents(t *testing.T) {
	result := buildMetaPrompt("Fix the login bug", nil)

	if !strings.Contains(result, "Fix the login bug") {
		t.Error("meta prompt should contain user input")
	}
	if !strings.Contains(result, "JSON array") {
		t.Error("meta prompt should request JSON array")
	}
	if strings.Contains(result, "already taken") {
		t.Error("should not mention existing agents when none provided")
	}
}

func TestBuildMetaPrompt_WithExistingAgents(t *testing.T) {
	result := buildMetaPrompt("Add OAuth", []string{"fix-login", "refactor-db"})

	if !strings.Contains(result, "Add OAuth") {
		t.Error("meta prompt should contain user input")
	}
	if !strings.Contains(result, "fix-login") {
		t.Error("should mention existing agent fix-login")
	}
	if !strings.Contains(result, "refactor-db") {
		t.Error("should mention existing agent refactor-db")
	}
	if !strings.Contains(result, "already taken") {
		t.Error("should warn about existing agent names")
	}
}

func TestBuildMetaPrompt_ContainsTaskPlannerInstructions(t *testing.T) {
	result := buildMetaPrompt("test", nil)

	if !strings.Contains(result, "task planner") {
		t.Error("meta prompt should identify as task planner")
	}
	if !strings.Contains(result, `"prompt"`) {
		t.Error("meta prompt should describe expected JSON fields")
	}
	if !strings.Contains(result, `"agent_name"`) {
		t.Error("meta prompt should describe agent_name field")
	}
	if !strings.Contains(result, `"branch"`) {
		t.Error("meta prompt should describe branch field")
	}
}

func TestExtractMatch_Found(t *testing.T) {
	result := extractMatch(agentIDRe, "| **Agent ID** | `my-agent` |")
	if result != "my-agent" {
		t.Errorf("extractMatch returned %q, want %q", result, "my-agent")
	}
}

func TestExtractMatch_NotFound(t *testing.T) {
	result := extractMatch(agentIDRe, "no agent table here")
	if result != "" {
		t.Errorf("expected empty string for no match, got %q", result)
	}
}

func TestExtractMatch_GitBranch(t *testing.T) {
	result := extractMatch(gitBranchRe, "| **Git Branch** | `feature/cool-stuff` |")
	if result != "feature/cool-stuff" {
		t.Errorf("extractMatch returned %q, want %q", result, "feature/cool-stuff")
	}
}

func TestExtractMatch_WithoutBoldMarkers(t *testing.T) {
	// The regex supports optional bold markers
	result := extractMatch(agentIDRe, "| Agent ID | `plain-agent` |")
	if result != "plain-agent" {
		t.Errorf("extractMatch without bold returned %q, want %q", result, "plain-agent")
	}
}

func TestParseStructuredMarkdown_SingleSection(t *testing.T) {
	// Need >= 2 sections to trigger parsing
	input := `## Prompt 1
| **Agent ID** | ` + "`solo`" + ` |
| **Git Branch** | ` + "`feature/solo`" + ` |
Do the work.`

	log := &LaunchLogger{}
	items := parseStructuredMarkdown(input, log)
	if items != nil {
		t.Error("expected nil for single section (need >= 2)")
	}
}

func TestParseStructuredMarkdown_MissingAgentID(t *testing.T) {
	input := `## Prompt 1
| **Git Branch** | ` + "`feature/a`" + ` |
Do A.
## Prompt 2
| **Agent ID** | ` + "`b`" + ` |
| **Git Branch** | ` + "`feature/b`" + ` |
Do B.`

	log := &LaunchLogger{}
	items := parseStructuredMarkdown(input, log)
	if items != nil {
		t.Error("expected nil when a section is missing Agent ID")
	}
}

func TestParseStructuredMarkdown_MissingBranch(t *testing.T) {
	input := `## Prompt 1
| **Agent ID** | ` + "`a`" + ` |
Do A.
## Prompt 2
| **Agent ID** | ` + "`b`" + ` |
| **Git Branch** | ` + "`feature/b`" + ` |
Do B.`

	log := &LaunchLogger{}
	items := parseStructuredMarkdown(input, log)
	if items != nil {
		t.Error("expected nil when a section is missing Git Branch")
	}
}

func TestParseClaudeResponse_InvalidJSON(t *testing.T) {
	raw := `[{"prompt": "oops", "agent_name": bad json}]`
	_, err := parseClaudeResponse(raw)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseClaudeResponse_NestedInText(t *testing.T) {
	raw := `Some preamble text here.
And more text.
[{"prompt":"Do stuff","agent_name":"stuff","branch":"fleet/stuff"}]
And some trailing text.`
	items, err := parseClaudeResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].AgentName != "stuff" {
		t.Errorf("agent_name = %q, want %q", items[0].AgentName, "stuff")
	}
}
