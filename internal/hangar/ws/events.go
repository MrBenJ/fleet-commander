package ws

type Event struct {
	Type      string   `json:"type"`
	Agent     string   `json:"agent,omitempty"`
	Message   string   `json:"message,omitempty"`
	State     string   `json:"state,omitempty"`
	Squadron  string   `json:"squadron,omitempty"`
	Agents    []string `json:"agents,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
}
