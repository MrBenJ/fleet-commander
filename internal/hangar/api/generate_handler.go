package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/MrBenJ/fleet-commander/internal/driver"
)

// HandleGenerate handles POST /api/squadron/generate — uses claude-code to generate an agent breakdown.
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

	drv, err := driver.Get("claude-code")
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
- "driver": "claude-code"
- "persona": leave as empty string

Example format:
[{"name":"auth-agent","prompt":"Implement OAuth2 login flow with Google and GitHub providers","branch":"","driver":"claude-code","persona":""}]`, req.Description)

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

// extractJSON finds the first JSON array in data (e.g., claude output that may
// include prose before/after the JSON).
func extractJSON(data []byte) []byte {
	start := bytes.IndexByte(data, '[')
	if start == -1 {
		return nil
	}
	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return data[start : i+1]
			}
		}
	}
	return nil
}
