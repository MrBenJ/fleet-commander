package squadron_test

import (
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestParseAndValidate_HappyPath(t *testing.T) {
	payload := []byte(`{
		"name": "alpha",
		"consensus": "review_master",
		"reviewMaster": "api",
		"baseBranch": "main",
		"autoMerge": true,
		"mergeMaster": "api",
		"agents": [
			{"name": "api", "branch": "squadron/alpha/api", "prompt": "Refactor the api"},
			{"name": "db",  "branch": "squadron/alpha/db",  "prompt": "Migrate the db", "driver": "claude-code", "persona": "zen-master"}
		]
	}`)

	data, errs := squadron.ParseAndValidate(payload)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if data.Name != "alpha" {
		t.Errorf("Name = %q, want alpha", data.Name)
	}
	if data.Consensus != "review_master" {
		t.Errorf("Consensus = %q", data.Consensus)
	}
	if data.ReviewMaster != "api" {
		t.Errorf("ReviewMaster = %q", data.ReviewMaster)
	}
	if !data.AutoMerge {
		t.Error("AutoMerge should be true")
	}
	if len(data.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(data.Agents))
	}
	if data.Agents[1].Persona != "zen-master" {
		t.Errorf("agent[1].Persona = %q", data.Agents[1].Persona)
	}
}

func TestParseAndValidate_DefaultsAutoMergeTrue(t *testing.T) {
	payload := []byte(`{
		"name": "beta",
		"consensus": "none",
		"agents": [
			{"name": "a", "branch": "squadron/beta/a", "prompt": "do a"}
		]
	}`)
	data, errs := squadron.ParseAndValidate(payload)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !data.AutoMerge {
		t.Error("AutoMerge should default to true when unset")
	}
}
