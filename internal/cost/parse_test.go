package cost

import (
	"os"
	"testing"
)

func TestParseReport_SumsEntriesPerProject(t *testing.T) {
	raw, err := os.ReadFile("testdata/ccusage_claude_daily.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	report, err := parseReport(raw)
	if err != nil {
		t.Fatalf("parseReport: %v", err)
	}
	if len(report) == 0 {
		t.Fatal("expected at least one project in the report")
	}
	for key, ac := range report {
		if !ac.Available {
			t.Errorf("project %q should be marked Available", key)
		}
		if ac.TotalCostUSD < 0 {
			t.Errorf("project %q has negative cost %v", key, ac.TotalCostUSD)
		}
	}
}

func TestParseReport_SkipsMalformedEntry(t *testing.T) {
	// The fixture (Task 2 Step 4) contains one malformed entry; parsing must
	// succeed and skip it rather than erroring.
	raw, _ := os.ReadFile("testdata/ccusage_claude_daily.json")
	if _, err := parseReport(raw); err != nil {
		t.Errorf("parseReport should skip malformed entries, got error: %v", err)
	}
}

func TestParseReport_EmptyProjects(t *testing.T) {
	report, err := parseReport([]byte(`{"projects":{}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report) != 0 {
		t.Errorf("expected empty report, got %d projects", len(report))
	}
}
