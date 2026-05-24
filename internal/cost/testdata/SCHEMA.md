# ccusage report schema notes

**Fixture status: REAL.** Captured from `ccusage 20.0.4` via
`ccusage claude daily --instances --json --offline` on 2026-05-23, then
hand-trimmed to two real projects plus one deliberately malformed entry.
The numbers in `ccusage_claude_daily.json` are genuine ccusage output.

## The three empirical unknowns (resolved)

### 1. Project-key format

ccusage keys each project by the **absolute worktree path** with path separators
and `.`/`_` characters replaced by `-`. The leading `/` of an absolute path
produces a **leading `-`** which ccusage keeps. Existing `-` characters are
preserved as single dashes.

Verbatim real pair (used by the matcher test in `match_test.go`):

| project key | worktree path |
|-------------|---------------|
| `-Users-bjunya-code-fleet-commander--fleet-worktrees-cost-viewer` | `/Users/bjunya/code/fleet-commander/.fleet/worktrees/cost-viewer` |

Note the `--fleet` doubling: it comes from `/.fleet` — the `/` becomes `-` and
the `.` becomes `-`, giving `--`.

**Consequence for `sanitizePath` (Task 4):** the transform is
`strings.NewReplacer("/", "-", "\\", "-", ".", "-", "_", "-")` with **no**
trailing `strings.Trim(..., "-")` — trimming would strip the leading `-` that
ccusage keeps and break the exact match. The plan's hypothesis included a
`Trim`; we drop it here, reconciled against the real captured key above.

### 2. Entry field names (per daily entry, inside each project array)

Confirmed exactly as the plan hypothesized:

- `totalCost` (float, USD)
- `inputTokens` (int)
- `outputTokens` (int)
- `cacheCreationTokens` (int)
- `cacheReadTokens` (int)
- `modelsUsed` (`[]string`)

(Each entry also carries `date`, `totalTokens`, `project`, and a
`modelBreakdowns` array that we ignore — the top-level per-entry fields above
are sufficient.)

### 3. kimi source name

Confirmed: the subcommand is **`kimi`** (`ccusage --help` lists `kimi  Show
Kimi usage commands`). `claude` and `codex` likewise confirmed. The Task 1
`driverSource` mapping is correct as written.

## Malformed entry

The second project (`-Users-bjunya-code-ableton-live-mcp-server`) has two real
daily entries plus a third entry whose `totalCost` is the string
`"MALFORMED_NOT_A_NUMBER"`. It fails to decode into the typed `ccusageEntry`
(float64) and must be **skipped** by `parseReport`, not crash the parse.
