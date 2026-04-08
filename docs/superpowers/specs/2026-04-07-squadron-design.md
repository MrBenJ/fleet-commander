# Squadron Mode — Design Spec

## Overview

Squadron mode adds a consensus mechanism to `fleet launch`. Agents launched as a squadron coordinate through fleet context channels to review and approve each other's work before considering their tasks complete.

Invoked via `fleet launch squadron`. Consensus mode and squadron name are collected interactively before entering the standard launch flow.

## CLI Interface

```
fleet launch squadron [--use-jump-sh]
```

`squadron` is a Cobra subcommand of `launch`. It does **not** expose `--ultra-dangerous-yolo-mode` or `--no-auto-merge` flags — these are implicit (squadron always runs yolo + no-auto-merge).

The `--use-jump-sh` flag is inherited from `launch` and works the same way.

## TUI Flow

Two new screens are prepended to the existing launch TUI flow:

### Screen 1: Consensus Mode Selector

Full-screen takeover. Three options navigated with arrow keys, selected with Enter. The highlighted option's description is displayed prominently below the list.

| Mode | Label | Description |
|------|-------|-------------|
| `universal` | UNIVERSAL CONSENSUS | All agents must review and approve every other agent's work. Work is not complete until unanimous approval. |
| `review_master` | REVIEW MASTER | One randomly designated agent reviews all other agents' work after they finish. |
| `none` | NONE | No review required. Agents finish independently. |

Esc/Ctrl+C aborts the entire flow.

### Screen 2: Squadron Name Input

Single text input for the squadron name. Validated with the same rules as agent names: alphanumeric with hyphens/underscores, starting with a letter or number (`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`). Max 30 characters.

Placeholder text: `alpha`

Enter confirms, Esc goes back to the consensus selector.

### Screen 3+: Standard Launch Flow

After name entry, the standard `fleet launch` flow runs:
1. Prompt input textarea (Ctrl+D to submit)
2. Claude generation spinner
3. Per-prompt review (launch / edit / skip / abort)

This is identical to the existing launch flow with `yoloMode=true` and `noAutoMerge=true`.

## Consensus Prompt Injection

After `buildFullPrompt()` assembles the system prompt + agent roster + task, a consensus suffix is appended to each agent's prompt in `launchCurrent()`. This happens right before the prompt is written to disk — the same insertion point where yolo auto-merge instructions are appended today.

### UNIVERSAL CONSENSUS Template

Appended to every agent's prompt:

```
---

## Squadron Consensus Protocol (UNIVERSAL)

You are a member of squadron "<squadronName>". Your squadron channel is `squadron-<squadronName>`.

After completing your primary task, you MUST participate in the squadron review process:

1. Announce completion:
   fleet context channel-send squadron-<squadronName> "COMPLETED: <one-line summary of what you did>"

2. Poll for other agents' status (every 30 seconds):
   fleet context channel-read squadron-<squadronName>

3. Once ALL squadron members have posted COMPLETED, review each agent's work:
   - Check out their branch: git diff main...<their-branch>
   - Evaluate: does their work meet the requirements described in their prompt?

4. Post your review for each agent:
   fleet context channel-send squadron-<squadronName> "APPROVED: <agent-name>"
   OR
   fleet context channel-send squadron-<squadronName> "CHANGES_REQUESTED: <agent-name> - <reason>"

5. If changes are requested on YOUR work, address them and re-announce:
   fleet context channel-send squadron-<squadronName> "REVISED: <summary of changes>"

6. Your work is ONLY complete when:
   - You have approved ALL other squadron members
   - ALL other squadron members have approved you

Squadron members: <comma-separated list of all agent names>
```

### REVIEW MASTER Template — Reviewer

Appended to the designated reviewer's prompt:

```
---

## Squadron Consensus Protocol (REVIEW MASTER)

You are the REVIEW MASTER for squadron "<squadronName>". Your squadron channel is `squadron-<squadronName>`.

After completing your own primary task:

1. Announce your own completion:
   fleet context channel-send squadron-<squadronName> "COMPLETED: <one-line summary>"

2. Poll for other agents' status (every 30 seconds):
   fleet context channel-read squadron-<squadronName>

3. Once ALL squadron members have posted COMPLETED, review each agent's work:
   - Check out their branch: git diff main...<their-branch>
   - Evaluate: does their work meet the requirements described in their prompt?

4. Post your review for each agent:
   fleet context channel-send squadron-<squadronName> "APPROVED: <agent-name>"
   OR
   fleet context channel-send squadron-<squadronName> "CHANGES_REQUESTED: <agent-name> - <reason>"

5. If you requested changes, wait for their REVISED message, then re-review.

6. Once all agents are approved, post:
   fleet context channel-send squadron-<squadronName> "ALL_APPROVED: Squadron review complete"

Squadron members: <comma-separated list of all agent names>
```

### REVIEW MASTER Template — Non-Reviewer

Appended to every other agent's prompt:

```
---

## Squadron Consensus Protocol (REVIEW MASTER)

You are a member of squadron "<squadronName>". Your squadron channel is `squadron-<squadronName>`.
Agent "<reviewerName>" is the designated review master.

After completing your primary task:

1. Announce completion:
   fleet context channel-send squadron-<squadronName> "COMPLETED: <one-line summary of what you did>"

2. Poll for the review master's feedback (every 30 seconds):
   fleet context channel-read squadron-<squadronName>

3. If changes are requested on your work, address them and re-announce:
   fleet context channel-send squadron-<squadronName> "REVISED: <summary of changes>"

4. Your work is complete when the review master posts APPROVED for you.

Squadron members: <comma-separated list of all agent names>
Review master: <reviewerName>
```

### NONE

No suffix appended. Agents operate independently.

## Channel Auto-Creation

Before the first agent in the squadron launches, a fleet context channel is created:

- **Name:** `squadron-<squadronName>`
- **Members:** all agent names from the generated launch items
- **Description:** `Squadron <squadronName> (<consensusMode>)`

This happens in `launchCurrent()` on the first call (guarded by a "channel created" flag on the model). If the channel already exists (e.g., re-launching a squadron), the creation error is ignored.

For REVIEW MASTER, the reviewer is selected randomly from the agent list at this point (before any agents launch) using `math/rand`.

## LaunchModel Changes

New fields on `LaunchModel`:

| Field | Type | Purpose |
|-------|------|---------|
| `squadronMode` | `bool` | `true` when entered via `fleet launch squadron` |
| `squadronName` | `string` | Collected from the name input screen |
| `consensusType` | `string` | `"universal"`, `"review_master"`, or `"none"` |
| `reviewMaster` | `string` | Agent name of the designated reviewer (review_master mode only) |
| `squadronChannelCreated` | `bool` | Guards one-time channel creation |

New `launchMode` constants:

| Constant | Value | Screen |
|----------|-------|--------|
| `launchModeSquadronConsensus` | (after existing modes) | Consensus selector |
| `launchModeSquadronName` | (after consensus) | Squadron name input |

## File Changes

| File | Change |
|------|--------|
| `cmd/fleet/cmd_launch.go` | Add `squadronCmd` as subcommand of `launchCmd`. Pass `squadronMode=true` to `tui.RunLaunch`. |
| `internal/tui/launch.go` | Add squadron fields to `LaunchModel`. Add `newSquadronLaunchModel()` constructor. Update `launchCurrent()` to create channel and append consensus suffix. Add `RunSquadronLaunch()` entry point. |
| `internal/tui/launch_modes.go` | Add `updateSquadronConsensus()` and `updateSquadronName()` mode handlers. |
| `internal/tui/launch_views.go` | Add `viewSquadronConsensus()` and `viewSquadronName()` view functions. |
| `internal/tui/squadron.go` | New file. Consensus prompt templates as string constants. `buildConsensusSuffix(consensusType, squadronName, agents []string, reviewMaster string) string` function. |

## Testing

- **Unit tests for `buildConsensusSuffix()`**: verify each consensus type produces correct prompt text with agent names and channel references substituted.
- **Unit tests for squadron name validation**: same regex as agent names.
- **Integration test**: verify that `RunSquadronLaunch` creates the fleet context channel with correct members.
