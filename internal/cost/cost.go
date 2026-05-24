// Package cost wraps the external `ccusage` CLI to report per-agent spend.
// Each Fleet agent runs in its own git worktree (its own project directory),
// which ccusage groups separately under `--instances`, giving per-agent cost.
package cost

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
