# Agent Shared Log Design

**Date:** 2026-04-04
**Status:** Approved

## Problem

The existing `context.json` has two sections: `shared` (user/orchestrator-only) and `agents` (each agent writes its own key). There is no way for agents to communicate findings or status to *each other* in a shared space — the `shared` section is intentionally blocked when `FLEET_AGENT_NAME` is set.

## Goal

Add an append-only, attributed log that any agent can write to, visible to everyone (agents and users), shown as a distinct section in `fleet context read`.

## Data Model

Add `LogEntry` struct and `Log []LogEntry` field to `Context` in `internal/context/context.go`:

```go
type LogEntry struct {
    Agent     string    `json:"agent"`
    Timestamp time.Time `json:"timestamp"`
    Message   string    `json:"message"`
}

type Context struct {
    Shared string            `json:"shared"`
    Agents map[string]string `json:"agents"`
    Log    []LogEntry        `json:"log,omitempty"`
}
```

## New Functions (`internal/context/context.go`)

### `AppendLog(fleetDir, agentName, message string) error`
- Acquires flock, reads current context, appends one `LogEntry{Agent: agentName, Timestamp: time.Now().UTC(), Message: message}`, writes back.
- Returns error if `message` is empty.

### `TrimLog(fleetDir string, keep int) error`
- Acquires flock, reads current context, retains only the last `keep` entries (by slice index), writes back.
- No-op if `len(Log) <= keep`.
- `keep = 0` clears the log entirely.

## CLI Commands (`cmd/fleet/cmd_context.go`)

### `fleet context log <message>`
- Requires `FLEET_AGENT_NAME` to be set (agent-only). Returns error otherwise.
- Calls `AppendLog(fleetDir, agentName, message)`.

### `fleet context trim [--keep N]`
- `--keep` flag, default `500`.
- No agent/user restriction — can be run from anywhere.
- Calls `TrimLog(fleetDir, keep)`.
- Prints how many entries remain after trim (or "log already within limit" if no-op).

### `fleet context read` (updated)
- After the existing `== Shared Context ==` and `== <agent> ==` sections, prints a new `== Agent Log ==` section.
- Each entry formatted as: `[2026-04-04T12:00:00Z] [agent-name] message`
- Section omitted entirely if `Log` is empty.

## Error Handling

| Scenario | Behavior |
|---|---|
| `log` with empty message | Error: "message cannot be empty" |
| `trim` when log ≤ keep entries | No-op, no error |
| `trim --keep 0` | Clears log entirely |
| Concurrent appends | Handled by existing flock |
| `log` without `FLEET_AGENT_NAME` | Error: "must be run from within a fleet agent session" |

## Out of Scope

- Automatic trimming on append (can be added later)
- Log streaming / tail -f
- Per-agent log filtering
