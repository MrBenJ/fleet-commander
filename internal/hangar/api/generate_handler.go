package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// HandleGenerate handles POST /api/squadron/generate — generates an agent
// breakdown using the requested driver (default: claude-code).
func (h *Handlers) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if req.Description == "" {
		writeError(w, http.StatusBadRequest, "description is required")
		return
	}

	selectedDriver := req.Driver
	if selectedDriver == "" {
		selectedDriver = "claude-code"
	}
	if selectedDriver != "claude-code" && selectedDriver != "codex" && selectedDriver != "antigravity" {
		writeError(w, http.StatusBadRequest, "driver must be claude-code, codex, or antigravity")
		return
	}
	// isDriverBinaryAvailable returns true for drivers without a binary mapping
	// (claude-code), so this generalized check preserves claude-code behavior
	// while covering codex and antigravity from driverBinaries.
	if !isDriverBinaryAvailable(selectedDriver) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s not installed", selectedDriver))
		return
	}

	drv, err := driverGet(selectedDriver)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get driver: %v", err))
		return
	}

	metaprompt := fmt.Sprintf(`You are a task decomposition assistant for Fleet Commander, a tool that manages parallel AI coding agents.

The user wants to accomplish the following:

%s

Break this down into individual agent tasks. Each agent handles exactly one task.

Respond with ONLY a JSON array (no markdown, no explanation, no code fences) where each element has:
- "name": a short kebab-case agent name. MUST be 20 characters or fewer, contain only letters, digits, and hyphens, and start with a letter or digit. Examples: "auth-agent", "test-writer", "db-migrator". Do NOT exceed 20 characters — names like "database-migration-agent" (24 chars) or "frontend-component-builder" (26 chars) will be rejected.
- "prompt": the full detailed task prompt for that agent
- "branch": leave as empty string (will be auto-generated)
- "driver": "%s"
- "persona": leave as empty string

Example format:
[{"name":"auth-agent","prompt":"Implement OAuth2 login flow with Google and GitHub providers","branch":"","driver":"%s","persona":""}]`, req.Description, selectedDriver, selectedDriver)

	out, err := drv.PlanCommand(metaprompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("plan command failed: %v", err))
		return
	}

	jsonData := extractJSON(out)
	if jsonData == nil {
		// No array means the model replied in prose instead of the requested
		// JSON — typically a refusal (e.g. a safety over-refusal on a "security
		// audit" task) or a clarifying question. Surface its actual words so the
		// failure is actionable rather than a cryptic parse error.
		writeError(w, http.StatusUnprocessableEntity, planNoArrayMessage(out))
		return
	}

	var agents []LaunchAgentInput
	if err := json.Unmarshal(jsonData, &agents); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to parse plan output: %v", err))
		return
	}

	// The planner is instructed to populate driver, but LLM output isn't
	// guaranteed. Anchor each agent to the driver the user actually picked so
	// selecting Codex never silently produces Claude-backed agents.
	for i := range agents {
		agents[i].Driver = selectedDriver
	}

	writeJSON(w, http.StatusOK, GenerateResponse{Agents: agents})
}

// planNoArrayMessage builds an actionable error for when the planner returned no
// JSON array. The model usually replied in prose — a refusal or a clarifying
// question — so we echo its actual words (rune-truncated) instead of a generic
// "no JSON array" message.
func planNoArrayMessage(out []byte) string {
	reply := strings.TrimSpace(string(out))
	const max = 600
	if r := []rune(reply); len(r) > max {
		reply = strings.TrimSpace(string(r[:max])) + "…"
	}
	if reply == "" {
		return "the agent returned no output and no task list (no JSON array). Try again or rephrase your description."
	}
	return "the agent did not return a task list (no JSON array). It replied: " + reply
}

// extractJSON finds the JSON array carrying the planner's answer. Three driver
// quirks force us to be careful:
//
//  1. Banners can contain decorative bracket pairs that aren't JSON — e.g.
//     codex's "sandbox: workspace-write [workdir, /tmp, $TMPDIR, ...]" header.
//  2. Codex echoes the entire user prompt back in stdout *before* the
//     assistant reply, so the metaprompt's example JSON (`[{"name":"auth-agent",...}]`)
//     appears as a valid array earlier than the actual answer.
//  3. An agent object can hold nested arrays in its fields (e.g. tags), and
//     those nested arrays mustn't be mistaken for the outer answer.
//
// Strategy: scan every '[' position with a string-aware balanced match, accept
// only candidates that parse as an array of *objects* (matching the planner
// schema), and return the *last* such candidate. Both supported drivers
// (claude-code, codex) place the real answer at or near the end of stdout,
// so taking the last schema-shaped array sidesteps the prompt echo.
func extractJSON(data []byte) []byte {
	var last []byte
	for i := 0; i < len(data); i++ {
		if data[i] != '[' {
			continue
		}
		end, ok := matchJSONArray(data, i)
		if !ok {
			continue
		}
		candidate := data[i : end+1]
		var probe []map[string]json.RawMessage
		if json.Unmarshal(candidate, &probe) == nil && len(probe) > 0 {
			last = candidate
		}
	}
	return last
}

// matchJSONArray returns the index of the ']' that balances the '[' at start,
// treating quoted JSON strings as opaque so brackets inside strings don't
// throw off depth. Returns false if the bracket is never balanced.
func matchJSONArray(data []byte, start int) (int, bool) {
	if start >= len(data) || data[start] != '[' {
		return 0, false
	}
	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(data); i++ {
		c := data[i]
		if inStr {
			switch {
			case escape:
				escape = false
			case c == '\\':
				escape = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}
