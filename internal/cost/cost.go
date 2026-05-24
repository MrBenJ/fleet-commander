// Package cost wraps the external `ccusage` CLI to report per-agent spend.
// Each Fleet agent runs in its own git worktree (its own project directory),
// which ccusage groups separately under `--instances`, giving per-agent cost.
package cost

import (
	"os/exec"
	"sync"
	"time"
)

// driverSource maps a Fleet driver name to the ccusage source subcommand.
// The second return is false for drivers ccusage cannot report (rendered as "—").
func driverSource(driver string) (string, bool) {
	switch driver {
	case "claude-code":
		return "claude", true
	case "codex":
		return "codex", true
	case "kimi-code":
		return "kimi", true
	default:
		return "", false
	}
}

// Seams overridable in tests.
var (
	lookPath   = exec.LookPath
	nowFunc    = time.Now
	cacheTTL   = 10 * time.Second
	runCcusage = func(source string) ([]byte, error) {
		cmd := exec.Command("ccusage", source, "daily", "--instances", "--json", "--offline")
		return cmd.Output()
	}
)

type cacheEntry struct {
	at     time.Time
	report map[string]AgentCost
	err    error
}

var (
	cacheMu     sync.Mutex
	reportCache = map[string]cacheEntry{}
)

func clearCache() {
	cacheMu.Lock()
	reportCache = map[string]cacheEntry{}
	cacheMu.Unlock()
}

// Available reports whether the ccusage CLI is on PATH. Mirrors the gh-missing
// pattern used to gate Auto PR.
func Available() bool {
	_, err := lookPath("ccusage")
	return err == nil
}

// Report runs ccusage for one source and returns per-project costs, caching the
// parsed result for cacheTTL so the hangar poll and CLI don't spawn a
// subprocess more often than necessary.
func Report(source string) (map[string]AgentCost, error) {
	cacheMu.Lock()
	if e, ok := reportCache[source]; ok && nowFunc().Sub(e.at) < cacheTTL {
		cacheMu.Unlock()
		return e.report, e.err
	}
	cacheMu.Unlock()

	raw, err := runCcusage(source)
	var report map[string]AgentCost
	if err == nil {
		report, err = parseReport(raw)
	}

	cacheMu.Lock()
	reportCache[source] = cacheEntry{at: nowFunc(), report: report, err: err}
	cacheMu.Unlock()
	return report, err
}
