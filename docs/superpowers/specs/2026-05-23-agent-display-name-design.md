# Agent Display Name — Design Spec

**Date:** 2026-05-23
**Status:** Approved design, ready for implementation plan
**Scope:** Add a discreet, cosmetic display name to squadron agents and rework persona prompts so an agent knows it is *named* (e.g. "Alex") while *wearing* a persona (e.g. Peter Molyneux) — never claiming to *be* the persona.

## Goal

Today a squadron agent with a persona introduces itself as the persona. An agent
named Alex wearing the Peter Molyneux persona says "I am Peter Molyneux." That is
wrong: Alex is the agent; Peter Molyneux is a costume.

Give each agent an optional **display name** — its discreet identity — that is
distinct from both the technical slug and the persona. Make it configurable in
the hangar wizard, expose it through the `fleet launch squadron --data` JSON, and
rework the internal prompts so the agent's real name always wins over the
persona's name.

## The bug, precisely

`ApplyPersona` prepends the persona preamble to the **very top** of the assembled
prompt (`internal/squadron/personas.go`):

```go
func ApplyPersona(p Persona, prompt string) string {
    return fmt.Sprintf("%s\n\n---\n\n%s", p.Preamble, prompt)
}
```

Every preamble opens with an identity claim — `"You are Peter Molyneux"`,
`"You are the Overconfident Engineer"`, etc. That claim sits **above** the
`"You are: <name>"` line emitted by `buildHeadlessPrompt` (`internal/squadron/headless.go:194`),
so the persona's name occupies the highest-priority position in the prompt and
overrides the agent's real identity.

## Decisions (locked)

| Decision | Choice |
|----------|--------|
| Name model | New **cosmetic** display name. The slug `Name` stays the sole key for tmux, branches, channels, and consensus tokens. |
| Field name | `DisplayName` (Go) / `displayName` (JSON + TypeScript) |
| Fallback | Empty `displayName` → use the slug `Name` |
| Validation | Optional; trimmed; max 50 chars; spaces allowed (it is cosmetic, never a key) |
| Persona fix | **Approach A** — identity-framing wrapper in `ApplyPersona` **plus** lightly de-claiming each preamble's opening line |
| Fight-mode label | Unchanged — keeps using the persona's `DisplayName` (it correctly references the persona, not the agent) |
| Out of scope | Single-agent `fleet add`; shared `system_prompt.md` identity section; AI-generate and CSV-import display names |

## Why "cosmetic" and not a coordination identity

The slug `Name` (`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`, max 30, no spaces) is load-bearing:

- tmux session name `fleet-<name>`
- default branch `squadron/<squadron>/<name>`
- channel membership and the `squadron-<name>` channel
- consensus review tokens: `APPROVED: <agent-name>`, `CHANGES_REQUESTED: <agent-name>`

Making the display name drive any of those would force reconciling spaces and
uniqueness against git/tmux constraints, and would risk desyncing the review
protocol (a reviewer posting `APPROVED: Alex` when the merge logic keys on the
slug). The display name is therefore **voice only**: it shapes how the agent
refers to itself in prose and in channel chatter. The slug remains the handle the
machinery and the consensus protocol key on.

## Architecture & data flow

```
hangar wizard / --data JSON
        │  displayName (optional, free text)
        ▼
SquadronAgent.DisplayName ──► resolveDisplayName(agent) ──► "Alex" (or slug fallback)
        │                                                        │
        │                                                        ├─► buildHeadlessPrompt:
        │                                                        │     "You are: Alex (coordination handle: alex, branch: ...)"
        │                                                        │
        └─► RunHeadless ──► ApplyPersona(persona, "Alex", prompt)
                                    │
                                    ▼
                  ## Your Identity  (names Alex, frames persona as a costume)
                  <persona preamble, opening line de-claimed>
                  ---
                  <full prompt>
```

The slug continues to populate the agent table column and every consensus/merge
suffix, so coordination is untouched.

## Components & changes

### 1. Data model — `internal/squadron/data.go`

Add `DisplayName` to `SquadronAgent`:

```go
type SquadronAgent struct {
    Name        string `json:"name"`
    DisplayName string `json:"displayName,omitempty"`
    Branch      string `json:"branch"`
    Prompt      string `json:"prompt"`
    Driver      string `json:"driver,omitempty"`
    Persona     string `json:"persona,omitempty"`
    FightMode   bool   `json:"fightMode,omitempty"`
}
```

`ParseAndValidate` already uses `DisallowUnknownFields`, so the field must be
added to the parsed struct (`SquadronAgent` is reused directly in
`rawSquadronData.Agents`, so no second struct edit is needed). Add validation:

- `displayName` is optional.
- If present, `len(strings.TrimSpace(a.DisplayName)) > 50` → error
  `agents[%d].displayName %q is too long (%d chars, max 50)`.
- No character-class restriction (spaces allowed).

Add a small helper (in `data.go` or `personas.go`):

```go
// resolveDisplayName returns the agent's display name, falling back to the slug
// Name when no display name is set.
func resolveDisplayName(a SquadronAgent) string {
    if dn := strings.TrimSpace(a.DisplayName); dn != "" {
        return dn
    }
    return a.Name
}
```

### 2. Prompt assembly — `internal/squadron/headless.go`

**`buildHeadlessPrompt`** — make the display name the headline identity while
keeping the slug visible as the coordination handle:

```go
b.WriteString(fmt.Sprintf("You are: %s (coordination handle: %s, branch: %s)\n\n",
    resolveDisplayName(current), current.Name, current.Branch))
```

The agent table (the `| Agent | Branch | Task |` rows) keeps using the **slug**
`a.Name`, because that is the handle other agents address in channels and the
consensus protocol references.

**`RunHeadless`** — pass the resolved display name into `ApplyPersona`:

```go
if a.Persona != "" {
    if p, ok := LookupPersona(a.Persona); ok {
        fullPrompt = ApplyPersona(p, resolveDisplayName(a), fullPrompt)
    }
}
```

### 3. Persona framing — `internal/squadron/personas.go`

**New `ApplyPersona` signature** (Approach A):

```go
// ApplyPersona wraps the prompt with an identity-framing block that names the
// agent, then the persona preamble (the costume), then the original prompt.
// displayName is the agent's discreet name; the persona is explicitly framed as
// a role the agent plays, never the agent's identity.
func ApplyPersona(p Persona, displayName, prompt string) string {
    identity := fmt.Sprintf(`## Your Identity

Your name is %s. That is who you are on this squadron — in commit messages, in
channel chatter, and whenever you refer to yourself.

You are playing a character: the **%s** persona described below. This is a
costume, not your identity. Adopt its voice, attitude, and quirks fully, but you
remain %s. Never say your name is %s. If asked who you are, you are %s wearing
the %s persona.`,
        displayName, p.DisplayName, displayName, p.DisplayName, displayName, p.DisplayName)

    return fmt.Sprintf("%s\n\n---\n\n%s\n\n---\n\n%s", identity, p.Preamble, prompt)
}
```

**De-claim each preamble's opening line** so no preamble asserts the persona name
as the agent's identity. Reword only the leading identity sentence of each of the
five preamble constants; leave the voice/behavior guidance intact. Examples:

| Persona | Before | After |
|---------|--------|-------|
| `peter-molyneux` | `You are Peter Molyneux. Yes, that Peter Molyneux.` | `You are playing Peter Molyneux. Yes, that Peter Molyneux.` |
| `overconfident-engineer` | `You are the Overconfident Engineer.` | `You are playing the Overconfident Engineer.` |
| `zen-master` | `You are the Zen Master.` | `You are playing the Zen Master.` |
| `paranoid-perfectionist` | `You are the Paranoid Perfectionist...` | `You are playing the Paranoid Perfectionist...` |
| `raging-jerk` | `You are the Raging Jerk.` | `You are playing the Raging Jerk.` |

The framing block is the authoritative identity signal; the de-claiming removes
the last contradicting "You are <persona>" text so nothing competes with it.

**Fight mode** (`internal/squadron/squadron.go`, `BuildFightModeSuffix`) is
unchanged. Its `%s` label is the persona `DisplayName` (or the slug when no
persona) and the phrasing — "make fun of them as your persona of '%s'" — already
treats the persona as a costume, which is correct.

### 4. Headless API surface — `internal/hangar/api/`

- `types.go`: add `DisplayName string \`json:"displayName,omitempty"\`` to
  `LaunchAgentInput`. (Optionally to `SquadronAgentInfo` for read-back symmetry —
  nice-to-have, not required.)
- `handlers.go` (squadron launch handler): map `LaunchAgentInput.DisplayName` →
  `SquadronAgent.DisplayName` wherever the other fields (Persona, FightMode) are
  copied across.

### 5. Hangar UI — `web/`

- `web/src/types.ts`: add `displayName?: string` to `SquadronAgent`.
- `web/src/api.ts`: thread `displayName` through the launch payload mapping
  alongside `persona` / `fightMode`.
- `web/src/components/wizard/ManualAddForm.tsx`: add a **Display Name** text input
  (optional). Placeholder shows the slug as the fallback hint
  (e.g. `Defaults to <name>`). Include it in the `onAgentAdded` object. A
  `HelpTooltip`: "The agent's discreet name (e.g. 'Alex'). Distinct from the
  persona it wears. Defaults to the agent name if left blank."
- `web/src/components/wizard/AgentCard.tsx`: add the **Display Name** input to the
  edit view (threaded through `editDraft` / `onDraftChange`), and surface it
  read-only on the summary row when set (e.g. a small `“Alex”` chip near the
  persona chip).
- AI-generate (`AIGeneratePanel`) and CSV import (`csvParser`) leave `displayName`
  empty — explicitly out of scope. Agents from those paths fall back to the slug.

## Error handling

- Display name longer than 50 chars after trimming → validation error from
  `ParseAndValidate`, surfaced the same way as existing agent-field errors (the
  `--data` flow prints them; the hangar API returns them in `ErrorResponse.Details`).
- Empty / whitespace-only display name → silently treated as unset (falls back to
  slug). Not an error.
- A persona that is unknown still errors exactly as today; display-name handling
  does not change persona validation.

## Testing

Go (`internal/squadron/`):

- Update `TestApplyPersona_PrependsPreamble` for the new three-arg signature.
  Assert: the `## Your Identity` block appears **before** the preamble; the block
  contains the supplied display name; the original prompt is preserved and last.
- New `personas_test.go` case: `ApplyPersona` output contains the
  "Never say your name is <persona>" framing and names the agent, not the persona.
- New `headless_test.go` case: build a squadron with an agent
  `{Name: "alex-slug", DisplayName: "Alex", Persona: "peter-molyneux"}`; assert the
  written prompt's `You are:` line is `Alex` with `coordination handle: alex-slug`,
  the agent table row still uses the slug, and no `"You are Peter Molyneux"`
  (un-de-claimed) string appears above the identity block.
- New `data_test.go` cases: JSON round-trip with `displayName` set; validation
  error when `displayName` exceeds 50 chars; `resolveDisplayName` falls back to
  the slug when empty/whitespace.
- Confirm `TestRunHeadless_FightModeUsesPersonaDisplayName` still passes (fight
  label intentionally unchanged).

Web (`cd web && npm run test`):

- `ManualAddForm.test.tsx`: entering a Display Name includes it in the added
  agent; leaving it blank omits it (or sends empty).
- `AgentCard.test.tsx`: editing exposes the Display Name field and round-trips it;
  the summary shows the display name when set.
- `api.test.ts`: launch payload carries `displayName`.

## Implementation order

1. Data model + `resolveDisplayName` + validation (`data.go`, tests).
2. `ApplyPersona` signature + identity block + preamble de-claiming (`personas.go`, tests).
3. `buildHeadlessPrompt` + `RunHeadless` wiring (`headless.go`, tests).
4. Headless API types + handler mapping.
5. Web types + `api.ts` + `ManualAddForm` + `AgentCard` (+ tests).
6. Full `go test ./...` and `cd web && npm run test`.

## Open questions

None. Design is locked per the decisions table.
