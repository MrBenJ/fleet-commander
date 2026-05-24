package ws

type Event struct {
	Type      string   `json:"type"`
	Agent     string   `json:"agent,omitempty"`
	Message   string   `json:"message,omitempty"`
	State     string   `json:"state,omitempty"`
	Squadron  string   `json:"squadron,omitempty"`
	Agents    []string `json:"agents,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`

	// Cost fields (type == "agent_cost").
	CostUSD             float64  `json:"costUSD,omitempty"`
	InputTokens         int      `json:"inputTokens,omitempty"`
	OutputTokens        int      `json:"outputTokens,omitempty"`
	CacheCreationTokens int      `json:"cacheCreationTokens,omitempty"`
	CacheReadTokens     int      `json:"cacheReadTokens,omitempty"`
	Models              []string `json:"models,omitempty"`
}
