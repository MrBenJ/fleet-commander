# Shared Context System

**Date:** 2026-03-27
**Status:** Approved

## Overview

A structured shared state store that lets fleet agents publish context about their work and read context from other agents. Agents coordinate asynchronously through a shared `.fleet/context.json` file with enforced ownership boundaries.

## Goals

- Agents can share decisions, interface definitions, status updates, and requests with other agents
- Agents can tag other agents (e.g., `@api-agent merge fleet/auth`) to get their attention
- The user controls the shared section; agents control their own sections
- Concurrent writes are safe via file locking

## Data Format

File: `.fleet/context.json`

```json
{
  "shared": "",
  "agents": {
    "auth-agent": "User model finalized at internal/auth/user.go. @api-agent merge fleet/auth",
    "api-agent": "Endpoints defined. Waiting on auth model."
  }
}
```

- `shared`: free-form string, only writable from outside agent sessions (user's terminal)
- `agents`: map of agent name to free-form string, each agent can only write to its own key
- File created lazily on first write (not by `fleet init`)
- Locking via `.fleet/context.lock` using the same flock pattern as `config.json`

## CLI Commands

All under `fleet context`:

| Command | Usage | Behavior |
|---------|-------|----------|
| `fleet context read` | Read full context | Dumps all sections as formatted output |
| `fleet context read <name>` | Read one agent's section | Prints just that agent's text (mutually exclusive with `--shared`) |
| `fleet context read --shared` | Read shared section | Prints just the shared text (mutually exclusive with agent name arg) |
| `fleet context write <message>` | Agent writes to own section | Requires `FLEET_AGENT_NAME` env var. Replaces the agent's entire section. |
| `fleet context set-shared <message>` | User sets shared context | Fails if `FLEET_AGENT_NAME` is set. Replaces the shared section. |

### Output format for `fleet context read`

```
== Shared Context ==
API uses JWT. Base path is /v2.

== auth-agent ==
User model finalized. @api-agent merge fleet/auth

== api-agent ==
Endpoints defined. Waiting on auth model.
```

### Write semantics

- `write` **replaces** the agent's section entirely (not append). Agents manage their own content.
- All writes acquire flock on `.fleet/context.lock` before read-modify-write.
- If `context.json` doesn't exist yet, `write` and `set-shared` create it.
- `read` on a missing file prints nothing (no error).

## Ownership Enforcement

- `fleet context write` requires `FLEET_AGENT_NAME` environment variable to be set. This env var is already injected into tmux sessions by the fleet start flow. The agent can only write to its own named section.
- `fleet context set-shared` requires `FLEET_AGENT_NAME` to NOT be set. This ensures only the user (outside agent sessions) can modify the shared section.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `write` outside agent session (no `FLEET_AGENT_NAME`) | Error: "must be run from within a fleet agent session", exit 1 |
| `set-shared` inside agent session (`FLEET_AGENT_NAME` set) | Error: "shared context can only be set from outside agent sessions", exit 1 |
| `read <name>` for nonexistent agent | Prints nothing, exit 0 |
| Flock contention | Blocks until lock is available |
| Malformed `context.json` | Returns error, does not overwrite |

## Package Structure

### `internal/context/context.go`

- `Context` struct: `Shared string`, `Agents map[string]string`
- `Load(fleetDir string) (*Context, error)` — reads `.fleet/context.json`, returns empty Context if file doesn't exist
- `Save(fleetDir string, ctx *Context) error` — writes JSON with flock on `.fleet/context.lock`
- `WriteAgent(fleetDir, agentName, message string) error` — acquires lock, loads, updates agent section, saves
- `WriteShared(fleetDir, message string) error` — acquires lock, loads, updates shared section, saves

### `cmd/fleet/main.go`

New `contextCmd` command group with subcommands:
- `readCmd` — reads and formats context output
- `writeCmd` — calls `WriteAgent` after checking `FLEET_AGENT_NAME`
- `setSharedCmd` — calls `WriteShared` after checking `FLEET_AGENT_NAME` is not set

## Locking Pattern

Mirrors `internal/fleet/fleet.go`:
1. Open `.fleet/context.lock` (create if needed, mode 0600)
2. Acquire exclusive flock (blocks until available)
3. Read `.fleet/context.json` from disk
4. Mutate in memory
5. Write back to disk
6. Release flock (close file)

## Out of Scope

- Auto-injection of context into worktrees (e.g., rendering to `FLEET_CONTEXT.md`)
- TUI integration (showing context in the queue view)
- Append mode or history tracking
- Notifications when tagged by another agent
