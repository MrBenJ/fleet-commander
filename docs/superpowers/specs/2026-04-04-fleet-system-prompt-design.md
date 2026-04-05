# Fleet System Prompt Injection Design

**Date:** 2026-04-04
**Status:** Approved

## Problem

Fleet agents launch with no awareness that they're part of a fleet. They don't know about the shared context system, channels, other active agents, or their own identity within the fleet. Users have no way to inject fleet-wide instructions into agent prompts.

## Goal

Prepend a system prompt and active agent roster to every agent's prompt at launch time. Provide a default `FLEET_SYSTEM_PROMPT.md` that documents fleet capabilities, and allow users to customize it.

---

## Architecture

### File Layout

- `internal/fleet/system_prompt.md` — Default system prompt content, embedded into the binary via `go:embed`
- `.fleet/FLEET_SYSTEM_PROMPT.md` — User-editable copy in the fleet directory (gitignored). Created by `fleet init`.

### Injection Point

`launchCurrent()` in `internal/tui/launch.go` prepends the system prompt and agent roster to the task prompt before passing it to `claude`.

### Prompt Assembly Order

For each agent, the final prompt is:

```
[contents of .fleet/FLEET_SYSTEM_PROMPT.md]

## Active Fleet Agents

You are: <agent-name> (branch: <branch>)

| Agent | Branch | Task |
|-------|--------|------|
| auth-agent | fleet/auth-agent | Fix the login validation bug |
| api-agent | fleet/api-agent | Add OAuth2 support |

---

<original user task prompt>
```

The agent roster is built from all `LaunchItem`s in the current launch batch (not from fleet config).

If `.fleet/FLEET_SYSTEM_PROMPT.md` doesn't exist, the roster and task prompt are still assembled — just without the system prompt preamble. The roster is always injected.

---

## Changes

### `internal/fleet/system_prompt.md` (new file)

Default content embedded via `go:embed`:

```markdown
# Fleet Commander — Agent System Prompt

You are an AI coding agent managed by Fleet Commander. You are one of several
agents working in parallel on the same repository, each in your own git worktree.

## Your Identity

Your agent name is available in the `FLEET_AGENT_NAME` environment variable.

## Communicating With Other Agents

### Shared Context (per-agent sections)
- `fleet context write "<message>"` — Update your status so others know what you're doing
- `fleet context read` — Read everyone's context, including the shared log
- `fleet context read <agent-name>` — Read a specific agent's context

### Shared Log (bulletin board)
- `fleet context log "<message>"` — Post a finding or status update visible to all agents
- Keep posts concise and actionable

### Private Channels (DMs)
- `fleet context channel-create <name> <agent1> <agent2>` — Create a private channel
- `fleet context channel-send <channel> "<message>"` — Send a message to a channel
- `fleet context channel-read <channel>` — Read channel messages
- `fleet context channel-list` — List all channels

## Best Practices

- Update your context regularly so other agents know your progress
- Check shared context before starting work to avoid duplicating effort
- Post to the shared log when you discover something other agents should know
- Keep your worktree branch clean — commit frequently
```

### `internal/fleet/fleet.go` (modify)

- Add `go:embed system_prompt.md` to embed the default content
- `Init()` writes the embedded content to `.fleet/FLEET_SYSTEM_PROMPT.md` (only if the file doesn't already exist — re-init won't overwrite)
- New exported function `LoadSystemPrompt(fleetDir string) (string, error)` — reads `.fleet/FLEET_SYSTEM_PROMPT.md`, returns content or empty string if file missing

### `internal/tui/launch.go` (modify)

- New function `buildFullPrompt(systemPrompt string, allItems []LaunchItem, currentItem LaunchItem) string` — assembles the final prompt
- `launchCurrent()` calls `buildFullPrompt` and uses the result as the prompt passed to `claude`
- System prompt is loaded once and stored on `LaunchModel` as `systemPrompt string`. Loaded lazily on first call to `launchCurrent()` (covers both YOLO and review flows). Warning to stderr on error, stored as empty string.

---

## Error Handling

| Scenario | Behavior |
|---|---|
| `.fleet/FLEET_SYSTEM_PROMPT.md` missing at launch | No error. Roster still injected, no system prompt preamble |
| `.fleet/FLEET_SYSTEM_PROMPT.md` missing at init | Always created for new fleets |
| Re-running `fleet init` on existing fleet | Does NOT overwrite existing file |
| File read error (permissions) | Warning to stderr, proceed without system prompt |
| Single agent launch | Roster still shown with just the one agent |
| Empty system prompt file | Treated same as missing — roster and task still assembled |

## Out of Scope

- Per-agent system prompts (all agents get the same one)
- Runtime reload of system prompt (reads once at launch)
- Injecting system prompt for `fleet start` / `fleet attach` (only `fleet launch`)
