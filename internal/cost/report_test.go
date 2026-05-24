package cost

import (
	"errors"
	"testing"
	"time"
)

func TestAvailable(t *testing.T) {
	orig := lookPath
	defer func() { lookPath = orig }()

	lookPath = func(string) (string, error) { return "/usr/bin/ccusage", nil }
	if !Available() {
		t.Error("Available() should be true when ccusage is on PATH")
	}
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	if Available() {
		t.Error("Available() should be false when ccusage is missing")
	}
}

func TestReport_CachesWithinTTL(t *testing.T) {
	origRun, origNow := runCcusage, nowFunc
	defer func() { runCcusage, nowFunc = origRun, origNow; clearCache() }()
	clearCache()

	calls := 0
	runCcusage = func(source string) ([]byte, error) {
		calls++
		return []byte(`{"projects":{"p":[{"totalCost":0.5,"modelsUsed":["m"]}]}}`), nil
	}
	base := time.Now()
	nowFunc = func() time.Time { return base }

	if _, err := Report("claude"); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if _, err := Report("claude"); err != nil { // within TTL → cached
		t.Fatalf("Report: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 ccusage invocation within TTL, got %d", calls)
	}

	nowFunc = func() time.Time { return base.Add(cacheTTL + time.Second) }
	if _, err := Report("claude"); err != nil { // TTL expired → re-run
		t.Fatalf("Report: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected re-invocation after TTL, got %d calls", calls)
	}
}
