package cost

import (
	"path/filepath"
	"strings"
)

// MatchProjectKey finds the AgentCost for a worktree within a parsed report.
// ccusage keys projects by a sanitized form of the project directory path;
// because each Fleet agent has a distinct worktree, the match is exact when the
// sanitization matches. Falls back to a basename comparison.
func MatchProjectKey(worktreePath string, report map[string]AgentCost) (AgentCost, bool) {
	abs, err := filepath.Abs(worktreePath)
	if err != nil {
		abs = worktreePath
	}
	sanitized := sanitizePath(abs)
	if ac, ok := report[sanitized]; ok {
		return ac, true
	}
	base := filepath.Base(abs)
	for key, ac := range report {
		if key == sanitized || filepath.Base(key) == base || strings.HasSuffix(key, sanitizePath(base)) {
			return ac, true
		}
	}
	return AgentCost{}, false
}

// sanitizePath mirrors ccusage's project-key derivation (confirmed empirically
// in testdata/SCHEMA.md): replace path separators, dots, and underscores with
// dashes. ccusage keeps the leading dash produced by an absolute path's leading
// "/", so — unlike the original plan hypothesis — we do NOT trim dashes here;
// trimming would strip that leading dash and break the exact-key match.
func sanitizePath(p string) string {
	r := strings.NewReplacer("/", "-", "\\", "-", ".", "-", "_", "-")
	return r.Replace(p)
}
