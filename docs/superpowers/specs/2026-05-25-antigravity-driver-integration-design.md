# Antigravity (`agy`) Driver Integration

> **Correction (2026-05-25, post-implementation):** Empirical `agy --help` on a
> real install (agy **1.0.2**) shows `agy` **does** ship
> `--dangerously-skip-permissions` ("Auto-approve all tool permission requests
> without prompting") plus `--sandbox`, and seeds an interactive session via `-i`
> /`--prompt-interactive` (not a bare positional arg). This reverses the
> "no bypass flag / yolo is best-effort" premise below, which was based on older
> docs and a now-stale forum thread. As built: `BuildCommand` passes
> `--dangerously-skip-permissions` in YoloMode and seeds with `-i`, and the
> pre-launch **babysitting warning was removed**. The cost-analysis info panel and
> everything else stand. Treat the struck-through framing below as historical.

## Goal

Add Google Antigravity's `agy` coding-agent CLI as a first-class driver in Fleet
Commander, on par with `claude-code` and `codex`. After this change a user can:

- Select **Antigravity** as the harness for any agent (manual add, CSV, edit).
- Pick **Antigravity** in the hangar's "AI Generate from Description" panel to
  decompose a task description into agent prompts.
- See the Antigravity brand icon on agent pills and driver chips.
- Launch Antigravity-backed agents via `fleet launch` / squadron mode, with
  YoloMode passing `--dangerously-skip-permissions` for unattended runs.
- Be informed, **before launching a squadron**, when any selected agent uses a
  harness that `ccusage` cannot report on (aider, generic, antigravity) — so the
  blank cost column is expected, not a bug.

## Background: the `agy` CLI surface

Grounded in Antigravity's published CLI docs and hands-on guides:

| Capability | Command |
|---|---|
| Interactive session | `agy` |
| Headless one-shot prompt (plain-text stdout) | `agy -p "<prompt>"` |
| Structured output | `agy -p "<prompt>" --output-format json` |
| Model select | `agy -m <model> -p "<prompt>"` |
| Inspect config (skills/plugins/hooks/MCP) | `agy inspect` |
| Autonomous, no approval pauses | `/goal <objective>` (slash command **inside** the TUI) |
| Install | `curl -fsSL https://antigravity.google/cli/install.sh \| bash` |

Key facts that shape the design:

- **`agy -p` is a clean headless mode** that emits the model's answer as plain
  text — a direct analogue of `claude -p` and `codex exec`. This is what AI
  Generate needs, and it feeds the existing `extractJSON` unchanged. We do **not**
  use `--output-format json` for planning, because that wraps the answer in a JSON
  envelope that `extractJSON` (which scans for a bare object-array) would not
  match — same reasoning the codebase already applies to `kimi --quiet`.
- ~~**There is no documented permission-bypass / yolo flag.**~~ **(Corrected — see
  the note at the top.)** `agy 1.0.2` exposes `--dangerously-skip-permissions`
  ("Auto-approve all tool permission requests without prompting"), so `BuildCommand`
  passes it in YoloMode, exactly like the codex and claude-code drivers. The
  prompt is seeded into an interactive session via `-i` /`--prompt-interactive`
  (which needs the TTY the tmux pane provides), not a bare positional argument.
- **No Claude-Code-style hook injection point.** `agy` has its own
  hooks/plugins/MCP config surfaced by `agy inspect`, but nothing equivalent to
  writing `.claude/settings.json`. So `InjectHooks`/`RemoveHooks` are no-ops and
  state is determined entirely by tmux pane-scraping — exactly like the codex and
  aider drivers.

## Architecture

Antigravity is structurally a **clone of the Codex driver**: a CLI-backed driver,
no hook system, state via pane heuristics. The integration touches the same files
any new driver touches, and registering it in the driver map is what makes it flow
automatically through the manual-add / edit / review dropdowns (which are
populated from `driver.Available()`).

### Backend (Go)

**1. `internal/driver/antigravity.go` (new) — `AntigravityDriver`**

Implements the full `driver.Driver` interface:

- `Name() string` → `"antigravity"`
- `InteractiveCommand() []string` → `[]string{"agy"}`
- `PlanCommand(prompt string) ([]byte, error)` →
  `exec.Command("agy", "-p", prompt).CombinedOutput()`
- `BuildCommand(opts LaunchOpts) string` → returns a shell script that seeds an
  interactive session with the prompt:
  ```
  #!/usr/bin/env bash
  prompt=$(cat "<PromptFile>")
  exec agy --dangerously-skip-permissions -i "$prompt"   # --dangerously... only when YoloMode
  ```
  In YoloMode it adds `--dangerously-skip-permissions`; otherwise it omits it.
  The prompt is always seeded via `-i` (`--prompt-interactive`).
- `DetectState(bottomLines []string, fullContent string) *AgentState` →
  pane-content heuristics. Empty content → `StateStarting`. **Waiting** patterns
  (checked first): approval/confirmation prompts (`[Y/n]`, `[y/N]`, `(y/n)`,
  "Approve", "requesting approval"), a lone `>` input line, and the
  trailing-question heuristic (line ending in `?`, len > 10, in last 3 lines).
  **Working** patterns: braille spinner glyphs and "esc to interrupt"-style text.
  No match → return `nil` (caller falls back), matching the codex driver's
  conservative default. A comment notes these patterns are based on the documented
  TUI and need empirical tuning against real `tmux capture-pane` output.
- `InjectHooks(worktreePath string) error` → `nil` (no-op)
- `RemoveHooks(worktreePath string) error` → `nil` (no-op)
- `CheckAvailable() error` → `exec.LookPath("agy")`; on failure returns a message
  including the install hint
  (`curl -fsSL https://antigravity.google/cli/install.sh | bash`).

**2. `internal/driver/registry.go`**

Add `"antigravity": &AntigravityDriver{}` to the `drivers` map. This single line
makes Antigravity appear everywhere `driver.Available()` is consumed — the
hangar's manual-add form, the agent-edit dropdown, and `GET /api/fleet/drivers`.

**3. `internal/hangar/api/handlers.go`**

- Add `"antigravity": "agy"` to the `driverBinaries` map (so availability checks
  resolve the right binary).
- Add `{Name: "antigravity", Available: isDriverBinaryAvailable("antigravity")}`
  to the slice returned by `HandleAvailableDrivers`.

**4. `internal/hangar/api/generate_handler.go`**

- Extend the allowed-driver guard from `claude-code`/`codex` to also accept
  `antigravity`.
- Add the binary-availability pre-check for `antigravity` (parallel to the
  existing codex check), returning a clear "agy not installed" error.

### Frontend (web)

**5. `web/src/components/icons/AntigravityIcon.tsx` (new)**

Renders the supplied brand PNG faithfully, embedded as a base64 data-URI `<img>`
(decision: faithful embed over a hand-rolled SVG approximation). Signature matches
the sibling icons: `({ size = 14 }: { size?: number })`, `aria-hidden`, square at
`size`×`size`. The PNG is downscaled to a small square (≈64×64) before encoding to
keep the inline string modest.

**6. `web/src/components/mission/AgentPill.tsx`**

Import `AntigravityIcon` and add `case "antigravity":` to the `DriverIcon` switch.
This also flows to `MultiView.tsx`, which renders `DriverIcon` from the same
component.

**7. `web/src/components/wizard/AIGeneratePanel.tsx`**

- Add an **Antigravity** `<option>` to the AI-Generate driver `<select>`.
- Generalize the current codex-only availability state into a small map/lookup so
  both `codex` and `antigravity` options can be independently disabled with a
  "not installed" hint when their binary is missing. Default selection stays
  `claude-code`.

**8. `web/src/components/wizard/review-constants.ts`**

Add an `antigravity` entry to both `driverColors` and `driverTextColors` so the
review-step driver chip and `AgentCard` pill render with a recognizable color
(blue family, matching the icon's dominant hue).

**9. ~~`ReviewStep.tsx` — pre-launch babysitting warning~~ (REMOVED — superseded)**

This item was built and then **removed** once `agy 1.0.2` was confirmed to support
`--dangerously-skip-permissions` (see the correction note at the top). With real
YOLO support, the "you must babysit this agent" caveat no longer holds, so the
orange warning banner was deleted. Only the cost-analysis info panel (item 10)
remains in the review step.

**10. `web/src/components/wizard/ReviewStep.tsx` — cost-analysis info panel**

A second, separate inline panel (distinct in purpose and tone from the
babysitting warning) that informs the user when any selected agent uses a harness
`ccusage` cannot report on. Trigger: at least one agent whose driver is in the
**cost-unsupported set** — `aider`, `generic`, `antigravity`. Copy names the
distinct unsupported harness(es) present, e.g.:

> The selected harness (antigravity) does not have cost analysis enabled due to
> tooling limitations and will display blank.

…and pluralizes when more than one is present, e.g. "The selected harnesses
(aider, antigravity) do not have cost analysis enabled…".

Tone and styling are **informational, not a warning or error**: neutral/info
palette (muted text on `var(--bg-secondary)`, e.g. `var(--text-secondary)` with a
subtle left border / ℹ️ affordance — *not* orange or red), `role="status"`. Purely
advisory; does not block or disable launch. Re-evaluates as agents change. (It was
originally designed to coexist with the item-9 babysitting warning; that warning
has since been removed, so this cost panel is now the only advisory panel.)

The cost-unsupported set lives as an exported constant
(`costUnsupportedDrivers`) in `review-constants.ts`, alongside the existing
hardcoded per-driver maps. A comment marks it as the UI mirror of
`cost.driverSource` (Go) — the authoritative source — so the two are kept in
sync by hand, consistent with how `driverColors`/`driverTextColors` already
enumerate drivers in that file.

### Cost reporting: intentionally unsupported

Fleet's per-agent cost column wraps the external `ccusage` CLI
(`internal/cost/`), mapping each driver to a ccusage *source* subcommand via
`cost.driverSource()`. `ccusage` (v20.x) has **no `antigravity`/`agy` source** —
its sources are claude, codex, opencode, amp, droid, codebuff, hermes, pi, goose,
kilo, copilot, gemini, kimi, qwen, openclaw. So Antigravity falls through
`driverSource`'s `default` branch (`("", false)`) and its cost renders as **"—"**,
exactly like `aider` and `generic` today.

**Do not add a `driverSource` mapping for antigravity:**

- There is no ccusage source to map to — the subcommand does not exist.
- Although Antigravity runs Gemini models and ccusage has a `gemini` source, that
  source reads the *Gemini CLI's* local logs, not `agy`'s. Attributing `agy` spend
  through `gemini` would be incorrect, not merely empty.

If a future ccusage release adds an `agy`/`antigravity` source, wiring it is a
one-line addition to the `driverSource` switch — but until then it would be dead
code, so it stays out of scope.

This blank-cost behavior is surfaced to the user before launch by the
cost-analysis info panel (frontend item 10), so an empty cost column reads as
expected rather than broken.

### What needs no change

- Manual-add (`ManualAddForm`), agent-edit (`AgentCard`), and the review step read
  their driver list from `getDrivers()` → `driver.Available()`. Once the driver is
  registered, Antigravity appears in all of them automatically.
- Squadron validation (`squadron.ParseAndValidate`) has **no** driver allowlist —
  it accepts any non-empty driver string — so no change is needed there.
- `extractJSON` and the metaprompt in `generate_handler.go` are driver-agnostic
  and work as-is with `agy -p` plain-text output.

## Error handling

- **`agy` not installed:** `CheckAvailable` and the API availability checks return
  a clear, install-hint-bearing error. The AI-Generate dropdown disables the
  Antigravity option; the launch path surfaces the driver error through existing
  channels.
- **Plan output unparseable:** handled by the existing `extractJSON` / unmarshal
  error paths in `generate_handler.go` — no new path needed.
- **Unknown state from pane scrape:** `DetectState` returns `nil`, and the caller
  falls back to legacy heuristics / `StateWorking`, exactly as for codex.

## Testing strategy

**Go**

- `internal/driver/antigravity_test.go` (mirror `codex_test.go`): asserts `Name`,
  `InteractiveCommand`, `BuildCommand` (prompt-file plumbing; yolo and non-yolo
  produce the documented script), a table-driven `DetectState` covering each
  waiting/working pattern and the empty-content and no-match cases, and
  `CheckAvailable` behavior.
- `internal/driver/registry_test.go`: extend the available-drivers assertion to
  include `antigravity`.
- `internal/hangar/api/handlers_test.go`: extend `HandleAvailableDrivers` coverage
  to include the `antigravity` entry (available + unavailable via the
  `execLookPath` test seam).
- `generate_handler` tests: a case proving `antigravity` is accepted and routed to
  the antigravity driver, plus the not-installed rejection path.

**Web**

- `AIGeneratePanel.test.tsx`: the Antigravity option renders, is disabled with a
  hint when unavailable, and is selectable/submittable when available.
- `AgentPill` / `DriverIcon`: a render assertion that `driver="antigravity"`
  produces the Antigravity icon (if the existing test file covers the switch).
- `ReviewStep.test.tsx`: no babysitting warning renders even when an Antigravity
  agent is present (the warning was removed once real YOLO support was confirmed).
- `ReviewStep.test.tsx`: the cost-analysis info panel appears (naming the
  harness) when an agent uses a cost-unsupported driver (aider/generic/
  antigravity), names multiple distinct harnesses when several are present, is
  absent for cost-supported-only squadrons (e.g. all claude-code/codex), and does
  not disable the Launch button.

**Manual / empirical**

- Run `agy --help` during implementation to confirm the available flags before
  finalizing `BuildCommand`/`PlanCommand`. (This is exactly what surfaced
  `--dangerously-skip-permissions` and `-i` post-implementation.)
- If `agy` is installed locally, capture a real pane via `tmux capture-pane` to
  validate `DetectState` patterns and tune them.

## Documentation

Update the driver lists and references so the new driver is discoverable:

- `CLAUDE.md` and `AGENTS.md` — add `antigravity` to the `Driver` enumeration and
  the `internal/driver/` description.
- `README.md` — driver/command reference.
- `docs/drivers/` — add an Antigravity entry consistent with the existing driver
  docs (CLI surface, install, yolo caveat, state-detection notes).

## Non-goals

- Wiring Antigravity's native hooks/plugins/MCP config (`agy inspect`) into
  Fleet's state system. Pane-scraping is sufficient and matches codex/aider.
- Model selection (`agy -m`) in the UI — out of scope for this change.
- Cost tracking for Antigravity agents — `ccusage` has no `agy` source, so cost
  renders as "—" (same as aider/generic). See "Cost reporting" above.
