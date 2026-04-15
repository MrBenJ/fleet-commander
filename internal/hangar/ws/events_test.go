package ws

import (
	"encoding/json"
	"testing"
)

func TestEventJSON_ContextMessage(t *testing.T) {
	ev := Event{
		Type:      "context_message",
		Agent:     "alpha",
		Message:   "hello world",
		Timestamp: "2025-01-15T10:30:00Z",
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "context_message" {
		t.Errorf("Type = %q", decoded.Type)
	}
	if decoded.Agent != "alpha" {
		t.Errorf("Agent = %q", decoded.Agent)
	}
	if decoded.Message != "hello world" {
		t.Errorf("Message = %q", decoded.Message)
	}
}

func TestEventJSON_AgentState(t *testing.T) {
	ev := Event{
		Type:      "agent_state",
		Agent:     "bravo",
		State:     "working",
		Timestamp: "2025-01-15T10:30:00Z",
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Event
	json.Unmarshal(data, &decoded)

	if decoded.State != "working" {
		t.Errorf("State = %q", decoded.State)
	}
}

func TestEventJSON_OmitsEmptyFields(t *testing.T) {
	ev := Event{Type: "agent_stopped", Agent: "charlie"}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	// omitempty fields should be absent
	for _, field := range []string{"message", "state", "squadron", "agents", "timestamp"} {
		if _, ok := raw[field]; ok {
			t.Errorf("expected field %q to be omitted, but it was present", field)
		}
	}
}

func TestEventJSON_SquadronEvent(t *testing.T) {
	ev := Event{
		Type:     "squadron_launched",
		Squadron: "alpha-squad",
		Agents:   []string{"a", "b", "c"},
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Event
	json.Unmarshal(data, &decoded)

	if decoded.Squadron != "alpha-squad" {
		t.Errorf("Squadron = %q", decoded.Squadron)
	}
	if len(decoded.Agents) != 3 {
		t.Errorf("Agents length = %d, want 3", len(decoded.Agents))
	}
}
