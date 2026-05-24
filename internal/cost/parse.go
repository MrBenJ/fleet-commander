package cost

import "encoding/json"

// AgentCost is the worktree-lifetime spend for one agent (or project).
// Available is false for unsupported drivers and when ccusage is missing, so
// callers render "—" instead of a misleading $0.00.
type AgentCost struct {
	TotalCostUSD        float64  `json:"costUSD"`
	InputTokens         int      `json:"inputTokens"`
	OutputTokens        int      `json:"outputTokens"`
	CacheCreationTokens int      `json:"cacheCreationTokens"`
	CacheReadTokens     int      `json:"cacheReadTokens"`
	Models              []string `json:"models"`
	Available           bool     `json:"available"`
}

// ccusageReport mirrors `ccusage <source> daily --instances --json`. Each
// project maps to a slice of raw daily entries, decoded one at a time so a
// malformed entry (e.g. a numeric field arriving as a string) fails to decode
// into the typed ccusageEntry and is skipped, rather than aborting the whole
// parse.
type ccusageReport struct {
	Projects map[string][]json.RawMessage `json:"projects"`
}

type ccusageEntry struct {
	TotalCost           float64  `json:"totalCost"`
	InputTokens         int      `json:"inputTokens"`
	OutputTokens        int      `json:"outputTokens"`
	CacheCreationTokens int      `json:"cacheCreationTokens"`
	CacheReadTokens     int      `json:"cacheReadTokens"`
	ModelsUsed          []string `json:"modelsUsed"`
}

// parseReport parses a ccusage JSON report and sums each project's daily
// entries into a single AgentCost (worktree-lifetime total). Malformed entries
// are skipped. Returns one AgentCost per project key.
func parseReport(raw []byte) (map[string]AgentCost, error) {
	var rep ccusageReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return nil, err
	}
	out := make(map[string]AgentCost, len(rep.Projects))
	for key, entries := range rep.Projects {
		ac := AgentCost{Available: true}
		models := map[string]bool{}
		for _, rawEntry := range entries {
			var e ccusageEntry
			if err := json.Unmarshal(rawEntry, &e); err != nil {
				continue // skip malformed entry
			}
			ac.TotalCostUSD += e.TotalCost
			ac.InputTokens += e.InputTokens
			ac.OutputTokens += e.OutputTokens
			ac.CacheCreationTokens += e.CacheCreationTokens
			ac.CacheReadTokens += e.CacheReadTokens
			for _, m := range e.ModelsUsed {
				if !models[m] {
					models[m] = true
					ac.Models = append(ac.Models, m)
				}
			}
		}
		out[key] = ac
	}
	return out, nil
}
