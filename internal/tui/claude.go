package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
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
// If the input contains structured markdown with agent identity tables, it parses
// them directly without calling Claude.
func GenerateWithClaude(userInput string, existingAgents []string, log *LaunchLogger) ([]LaunchItem, error) {
	log.Log("GenerateWithClaude: input_len=%d existing_agents=%d", len(userInput), len(existingAgents))

	// Try structured markdown parsing first (deterministic, no LLM needed)
	if items := parseStructuredMarkdown(userInput, log); len(items) > 0 {
		log.Log("Parse method: structured markdown (no LLM call)")
		log.Log("Parsed %d agents from structured markdown", len(items))
		return items, nil
	}

	log.Log("Parse method: Claude LLM meta-prompt (structured markdown not detected)")
	metaPrompt := buildMetaPrompt(userInput, existingAgents)
	log.Log("Meta-prompt length: %d bytes", len(metaPrompt))

	cmd := exec.Command("claude", "-p", metaPrompt)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Log("ERROR: claude -p failed: %v\noutput_len=%d\nfirst_500_bytes=%s", err, len(output), truncate(string(output), 500))
		return nil, fmt.Errorf("claude command failed: %w\noutput: %s", err, string(output))
	}
	log.Log("Claude response: %d bytes", len(output))
	log.Log("Claude raw output (first 1000 chars): %s", truncate(string(output), 1000))

	items, err := parseClaudeResponse(string(output))
	if err != nil {
		log.Log("ERROR: parseClaudeResponse failed: %v", err)
		return nil, err
	}
	log.Log("Parsed %d items from Claude response", len(items))
	return items, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func buildMetaPrompt(userInput string, existingAgents []string) string {
	var sb strings.Builder

	sb.WriteString(`You are a task planner for Fleet Commander, a tool that manages parallel Claude Code sessions.

Given the following list of tasks, generate a JSON array where each element has:
- "prompt": A detailed, actionable prompt to send to Claude Code for this task. Expand the user's brief description into a clear, specific instruction that Claude Code can act on immediately.
- "agent_name": A short kebab-case name for the agent (max 30 chars, lowercase, no special chars except hyphens)
- "branch": A git branch name in the format "fleet/<agent_name>", or explicitly named in the listed prompt itself

You may see in the prompts that agent names and and git branches already provided. If this is the case, follow what the prompts are saying for agent name and git branch.
This means that the entire fleet is self coordinating

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

// Regex patterns for structured markdown parsing
var (
	// Matches ## Prompt N headers
	promptHeaderRe = regexp.MustCompile(`(?m)^##\s+Prompt\s+\d+`)
	// Extracts Agent ID from markdown table: | **Agent ID** | `value` |
	agentIDRe = regexp.MustCompile("\\|\\s*\\*{0,2}Agent ID\\*{0,2}\\s*\\|\\s*`([^`]+)`\\s*\\|")
	// Extracts Git Branch from markdown table: | **Git Branch** | `value` |
	gitBranchRe = regexp.MustCompile("\\|\\s*\\*{0,2}Git Branch\\*{0,2}\\s*\\|\\s*`([^`]+)`\\s*\\|")
)

// parseStructuredMarkdown detects and parses markdown with ## Prompt N headers
// and agent identity tables. Returns nil if the input doesn't match this format.
func parseStructuredMarkdown(input string, log *LaunchLogger) []LaunchItem {
	locs := promptHeaderRe.FindAllStringIndex(input, -1)
	if len(locs) < 2 {
		log.Log("Structured markdown check: found %d '## Prompt N' headers (need >= 2), skipping", len(locs))
		return nil
	}
	log.Log("Structured markdown check: found %d '## Prompt N' headers", len(locs))

	// Split input into sections at each ## Prompt N header
	var sections []string
	for i, loc := range locs {
		start := loc[0]
		var end int
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else {
			end = len(input)
		}
		sections = append(sections, input[start:end])
	}

	var items []LaunchItem
	for i, section := range sections {
		agentID := extractMatch(agentIDRe, section)
		branch := extractMatch(gitBranchRe, section)

		if agentID == "" || branch == "" {
			log.Log("Section %d missing agent_id=%q or branch=%q, falling back to Claude", i, agentID, branch)
			return nil
		}

		items = append(items, LaunchItem{
			Prompt:    strings.TrimSpace(section),
			AgentName: agentID,
			Branch:    branch,
		})
		log.Log("  [%d] agent=%q branch=%q prompt_len=%d", i, agentID, branch, len(section))
	}

	return items
}

func extractMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
