package main

import (
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/cost"
)

func TestRenderCostTable_UnsupportedDriverShowsDash(t *testing.T) {
	rows := []costRow{
		{Agent: "api", Driver: "claude-code", Cost: cost.AgentCost{TotalCostUSD: 1.5, Models: []string{"claude-opus-4-7"}, Available: true}},
		{Agent: "legacy", Driver: "aider", Cost: cost.AgentCost{Available: false}},
	}
	out := renderCostTable("myrepo", rows)
	if !strings.Contains(out, "$1.50") {
		t.Errorf("expected formatted cost, got:\n%s", out)
	}
	if !strings.Contains(out, "—") {
		t.Errorf("unsupported driver should render as —, got:\n%s", out)
	}
	if !strings.Contains(out, "$1.50") || !strings.Contains(strings.ToLower(out), "total") {
		t.Errorf("expected a total row, got:\n%s", out)
	}
}

func TestRenderCostJSON(t *testing.T) {
	rows := []costRow{{Agent: "api", Driver: "claude-code", Cost: cost.AgentCost{TotalCostUSD: 2.0, Available: true}}}
	out := renderCostJSON(rows)
	if !strings.Contains(out, `"agent": "api"`) && !strings.Contains(out, `"agent":"api"`) {
		t.Errorf("expected agent in JSON, got:\n%s", out)
	}
}
