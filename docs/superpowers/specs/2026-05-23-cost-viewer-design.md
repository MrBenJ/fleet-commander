# Cost Viewer — Design Spec

**Date:** 2026-05-23
**Status:** Approved design, ready for implementation plan
**Scope:** v1 — live passive cost readout in the hangar and CLI, powered by `ccusage`.

## Goal

Give the operator live visibility into what each agent (and each squadron) is
costing, so running many agents in parallel stops being a blind spend. v1 is
**passive readout only** — show the numbers, the human decides. No budget caps,
no auto-stop.

## Why ccusage

[ccusage](https://github.com/ryoppippi/ccusage) already solves the hard parts:

- It reads the JSONL transcripts coding agents write locally and applies an
  up-to-date, externally-maintained price table.
- `--instances` groups usage **by project**, and each Fleet agent runs in its
  own git worktree — i.e. its own project directory — so per-agent granularity
  falls out naturally.
- It is **multi-source** (`ccusage claude`, `ccusage codex`, `ccusage kimi`,
  `ccusage gemini`, …), so we get cost for non-Claude drivers for free instead
  of being Claude-only.

The alternative (a self-contained Go parser + hardcoded price table) was
rejected: more code, a price table to keep current, and Claude-only. Leaning on
ccusage means a thin wrapper, maintained pricing, and multi-driver support. The
trade is one new runtime dependency, handled by graceful degradation (below).

## Decisions (locked)

| Decision | Choice |
|----------|--------|
| Core job | Live passive readout (no caps in v1) |
| Surfaces | Hangar **and** CLI (`fleet cost`) |
| Engine | Wrapper that shells out to `ccusage` |
| Invocation | Require `ccusage` on `PATH`; degrade gracefully if absent. **No** auto-`npx`. |
| Per-agent number | **Worktree-lifetime total** (all sessions in that worktree) |

`ccusage`'s date filters are day-granular, so a precise sub-day "since this
squadron launched" is not expressible; worktree-lifetime is both the natural
"what has this agent cost me" figure and the clean fit for the tool.

## Architecture & data flow

```
agent.WorktreePath ──► driver→source map ──► `ccusage <source> daily --instances --json --offline`
                                                          │
                                              parse { projects: { <key>: [...] } }
                                                          │
                                          match project key ──► agent
                                                          │
                                  internal/cost.AgentCost{ $, in/out/cache tokens, model }
                                         │                          │
                                    `fleet cost`            ws.Hub (slow poll) ──► agent_cost event
                                                                                        │
                                                              AgentPill badge + AgentTooltip + squadron total
```

## Components

### `internal/cost/` — ccusage wrapper

- **`Available() bool`** — is `ccusage` on `PATH`? Mirrors the existing
  `gh`-missing pattern used to disable Auto PR.
- **`driverSource(driver string) (source string, ok bool)`** — maps Fleet
  drivers to ccusage sources:
  - `claude-code` → `claude`
  - `codex` → `codex`
  - `kimi-code` → `kimi` *(verify exact ccusage source name during build)*
  - `aider`, `generic` → unsupported → rendered as `—`
- **`Report(source string) (map[projectKey]AgentCost, error)`** — runs
  `ccusage <source> daily --instances --json --offline`, parses the `projects`
  map. `--offline` uses cached pricing (no network on the hot path).
- **Agent → project-key mapping.** ccusage keys the `projects` map by a project
  identifier derived from the project directory. The exact string format is the
  one detail not fully nailed from docs — it is resolved **empirically** during
  implementation: run ccusage once against real fleet data, inspect the keys,
  write the matcher, and add a test asserting against those real keys. Because
  each worktree is a distinct project dir, the entry is guaranteed to exist; only
  the key string needs confirming.
- **Caching.** Cache the parsed report for a short TTL (e.g. 10s) keyed by
  source, so the hangar poll and CLI don't spawn a subprocess more often than
  necessary.

`AgentCost` carries: `TotalCostUSD float64`, `InputTokens`, `OutputTokens`,
`CacheCreationTokens`, `CacheReadTokens int`, `Models []string`, and an
`Available bool` / status so unsupported drivers and missing-ccusage render
honestly (`—`) instead of a misleading `$0.00`.

### CLI — `fleet cost`

- `fleet cost` — table for the current repo: agent · model(s) · tokens · `$`,
  with a total row. Unsupported drivers show `—`.
- `fleet cost --all` — across all registered repos (mirrors `fleet list --all`).
- `fleet cost --json` — machine-readable.
- If `ccusage` is missing: print a single clear line with the install hint
  (`npm i -g ccusage`) and exit non-zero, no stack trace.

### Hangar

- Extend `ws.Event` (`internal/hangar/ws/events.go`) with cost fields:
  `CostUSD float64`, token counts, and reuse existing `Agent`/`Squadron`.
- Hub emits a new **`agent_cost`** event type. It polls cost on a **slower
  cadence than agent state** (e.g. ~10–15s vs the 2s state poll) because each
  refresh spawns a subprocess; only broadcast when an agent's cost changes.
- Frontend (`web/src/`):
  - `useWebSocket`/squadron hook keeps a `costByAgent` map (same pattern as
    `agentStates`).
  - `AgentPill` shows a small `$X.XX` badge; `—` when unavailable.
  - `AgentTooltip` shows the token breakdown + model(s).
  - `MissionControl` header shows the **squadron total** (sum over its agents).
- If `ccusage` is missing, the UI shows an unobtrusive "cost unavailable —
  install ccusage" note rather than zeros.

## Testing

**Go**
- `driverSource` mapping table test.
- JSON parse test against a captured `ccusage … --json` fixture, including a
  malformed entry to prove we skip rather than crash.
- Agent → project-key matcher test asserting against **real** captured keys.
- `Available()` false path → CLI prints hint, exits non-zero; hub broadcasts no
  cost events.

**Web (Vitest)**
- `AgentPill` renders `$` badge and `—` for unavailable.
- `MissionControl` squadron-total rollup sums member costs.

## Graceful degradation

`ccusage` absent is a first-class state, not an error:
- CLI: one-line install hint, non-zero exit.
- Hangar: "cost unavailable — install ccusage" note; no `agent_cost` events.
- Unsupported drivers (`aider`, `generic`): `—` everywhere.

## Out of scope (v1)

- Budget caps / auto-stop / pause-on-threshold.
- Historical charts / time-series.
- Sub-day "since this squadron launched" windows (ccusage is day-granular).
- Bundling/installing ccusage for the user; auto-`npx`.

## Known risks

1. **ccusage project-key format** — the agent→project mapping depends on
   ccusage's key scheme. Mitigated by resolving it empirically with a test
   against real keys, and by the fact that each worktree is a distinct project.
2. **ccusage JSON schema drift** — we are coupled to its output shape. Mitigated
   by a small, isolated parser with fixture tests.
3. **Subprocess cost on the hot path** — mitigated by short-TTL caching, a slow
   poll cadence, and `--offline`.
4. **ccusage source-name accuracy** for `kimi-code` (and any future driver) —
   verify exact source names during the build.
