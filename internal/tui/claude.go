package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeGeneratedItem represents a single task expanded by Claude.
type ClaudeGeneratedItem struct {
	Prompt    string `json:"prompt"`
	AgentName string `json:"agent_name"`
	Branch    string `json:"branch"`
}

// GenerateWithClaude sends the user's raw task list to `claude -p` and gets back
// structured prompts, agent names, and branch names for each item.
func GenerateWithClaude(userInput string, existingAgents []string) ([]LaunchItem, error) {
	metaPrompt := buildMetaPrompt(userInput, existingAgents)

	cmd := exec.Command("claude", "-p", metaPrompt)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude command failed: %w\noutput: %s", err, string(output))
	}

	return parseClaudeResponse(string(output))
}

func buildMetaPrompt(userInput string, existingAgents []string) string {
	var sb strings.Builder

	sb.WriteString(`You are a task planner for Fleet Commander, a tool that manages parallel Claude Code sessions.

Given the following list of tasks, generate a JSON array where each element has:
- "prompt": A detailed, actionable prompt to send to Claude Code for this task. Expand the user's brief description into a clear, specific instruction that Claude Code can act on immediately.
- "agent_name": A short kebab-case name for the agent (max 30 chars, lowercase, no special chars except hyphens)
- "branch": A git branch name in the format "fleet/<agent_name>"

`)

	if len(existingAgents) > 0 {
		sb.WriteString("These agent names are already taken, do NOT reuse them: ")
		sb.WriteString(strings.Join(existingAgents, ", "))
		sb.WriteString("\n\n")
	}

	sb.WriteString("Tasks:\n")
	sb.WriteString(userInput)
	sb.WriteString("\n\nRespond ONLY with a valid JSON array. No markdown fences, no explanation, no extra text.")

	return sb.String()
}

func parseClaudeResponse(raw string) ([]LaunchItem, error) {
	trimmed := strings.TrimSpace(raw)

	// Strip markdown code fences if present
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		// Remove first and last lines (the fences)
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
			trimmed = strings.Join(lines, "\n")
		}
	}
	trimmed = strings.TrimSpace(trimmed)

	// Find the JSON array boundaries
	start := strings.Index(trimmed, "[")
	end := strings.LastIndex(trimmed, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response:\n%s", raw)
	}
	jsonStr := trimmed[start : end+1]

	var generated []ClaudeGeneratedItem
	if err := json.Unmarshal([]byte(jsonStr), &generated); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w\nraw: %s", err, jsonStr)
	}

	if len(generated) == 0 {
		return nil, fmt.Errorf("claude returned an empty list")
	}

	items := make([]LaunchItem, len(generated))
	for i, g := range generated {
		items[i] = LaunchItem{
			Prompt:    g.Prompt,
			AgentName: g.AgentName,
			Branch:    g.Branch,
		}
	}

	return items, nil
}
