package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	if selectedDriver != "claude-code" && selectedDriver != "codex" {
		writeError(w, http.StatusBadRequest, "driver must be claude-code or codex")
		return
	}
	if selectedDriver == "codex" && !isDriverBinaryAvailable("codex") {
		writeError(w, http.StatusBadRequest, "codex not installed")
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
		writeError(w, http.StatusInternalServerError, "no JSON array found in plan output")
		return
	}

	var agents []LaunchAgentInput
	if err := json.Unmarshal(jsonData, &agents); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to parse plan output: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, GenerateResponse{Agents: agents})
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
