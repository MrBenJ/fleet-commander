package cost

import "testing"

func TestMatchProjectKey_RealKey(t *testing.T) {
	const realKey = `-Users-bjunya-code-fleet-commander--fleet-worktrees-cost-viewer`
	const worktreePath = `/Users/bjunya/code/fleet-commander/.fleet/worktrees/cost-viewer`

	report := map[string]AgentCost{
		realKey: {TotalCostUSD: 1.23, Available: true},
	}
	got, ok := MatchProjectKey(worktreePath, report)
	if !ok {
		t.Fatalf("expected to match worktree %q to key %q", worktreePath, realKey)
	}
	if got.TotalCostUSD != 1.23 {
		t.Errorf("matched the wrong entry: %+v", got)
	}
}

func TestMatchProjectKey_NoMatch(t *testing.T) {
	report := map[string]AgentCost{"some-other-project": {Available: true}}
	if _, ok := MatchProjectKey("/nope/not/here", report); ok {
		t.Error("expected no match for an unknown worktree")
	}
}
