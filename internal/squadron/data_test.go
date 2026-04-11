package squadron_test

import (
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func errContains(errs []error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), substr) {
			return true
		}
	}
	return false
}

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

func TestParseAndValidate_MissingName(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"consensus": "none",
		"agents": [{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "name") {
		t.Errorf("expected 'name' error, got: %v", errs)
	}
}

func TestParseAndValidate_InvalidSquadronName(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name": "-bad name!",
		"consensus": "none",
		"agents": [{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "name") {
		t.Errorf("expected name-format error, got: %v", errs)
	}
}

func TestParseAndValidate_InvalidConsensus(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"bogus",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "consensus") {
		t.Errorf("expected consensus error, got: %v", errs)
	}
}

func TestParseAndValidate_ReviewMasterRequiredWhenMode(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"review_master",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "reviewMaster") {
		t.Errorf("expected reviewMaster error, got: %v", errs)
	}
}

func TestParseAndValidate_ReviewMasterForbiddenWhenNotMode(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none","reviewMaster":"a",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "reviewMaster") {
		t.Errorf("expected reviewMaster-forbidden error, got: %v", errs)
	}
}

func TestParseAndValidate_ReviewMasterNotAnAgent(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"review_master","reviewMaster":"ghost",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "ghost") {
		t.Errorf("expected reviewMaster-not-found error, got: %v", errs)
	}
}

func TestParseAndValidate_MergeMasterNotAnAgent(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none","mergeMaster":"ghost",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "mergeMaster") {
		t.Errorf("expected mergeMaster error, got: %v", errs)
	}
}

func TestParseAndValidate_DuplicateAgentName(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none",
		"agents":[
			{"name":"a","branch":"b1","prompt":"p"},
			{"name":"a","branch":"b2","prompt":"q"}
		]
	}`))
	if !errContains(errs, "duplicate") {
		t.Errorf("expected duplicate error, got: %v", errs)
	}
}

func TestParseAndValidate_EmptyAgentFields(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none",
		"agents":[{"name":"","branch":"","prompt":""}]
	}`))
	if len(errs) < 3 {
		t.Errorf("expected >=3 errors for empty name/branch/prompt, got: %v", errs)
	}
}

func TestParseAndValidate_UnknownPersona(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none",
		"agents":[{"name":"a","branch":"b","prompt":"p","persona":"ghost"}]
	}`))
	if !errContains(errs, "persona") {
		t.Errorf("expected persona error, got: %v", errs)
	}
}

func TestParseAndValidate_EmptyAgentsArray(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none","agents":[]
	}`))
	if !errContains(errs, "agent") {
		t.Errorf("expected empty-agents error, got: %v", errs)
	}
}

func TestParseAndValidate_UnknownTopLevelField(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none","extra":"nope",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if len(errs) == 0 {
		t.Error("expected unknown-field error")
	}
}

func TestParseAndValidate_MultipleErrorsReported(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"!!","consensus":"bogus","agents":[]
	}`))
	if len(errs) < 3 {
		t.Errorf("expected all errors reported at once, got %d: %v", len(errs), errs)
	}
}
