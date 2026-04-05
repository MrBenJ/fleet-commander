# Agent Shared Log & Channel Design

**Date:** 2026-04-04
**Status:** Approved

## Problem

The existing `context.json` has two sections: `shared` (user/orchestrator-only) and `agents` (each agent writes its own key). There is no way for agents to communicate findings or status to *each other* in a shared space — the `shared` section is intentionally blocked when `FLEET_AGENT_NAME` is set. There is also no way for a subset of agents to have a private conversation.

## Goals

1. Add an append-only, attributed shared log that any agent can write to, visible to everyone (agents and users).
2. Add private channels — named spaces with fixed membership — where agents can communicate with each other. Channels are readable by anyone (agents and orchestrator) but writable only by members.

---

## Part 1: Shared Log

### Data Model

Add `LogEntry` struct and `Log []LogEntry` field to `Context` in `internal/context/context.go`:

```go
type LogEntry struct {
    Agent     string    `json:"agent"`
    Timestamp time.Time `json:"timestamp"`
    Message   string    `json:"message"`
}

type Context struct {
    Shared   string             `json:"shared"`
    Agents   map[string]string  `json:"agents"`
    Log      []LogEntry         `json:"log,omitempty"`
    Channels map[string]*Channel `json:"channels,omitempty"`
}
```

### New Functions (`internal/context/context.go`)

#### `AppendLog(fleetDir, agentName, message string) error`
- Acquires flock, reads current context, appends one `LogEntry{Agent: agentName, Timestamp: time.Now().UTC(), Message: message}`, writes back.
- Returns error if `message` is empty.

#### `TrimLog(fleetDir string, keep int) error`
- Acquires flock, reads current context, retains only the last `keep` entries (by slice index), writes back.
- No-op if `len(Log) <= keep`.
- `keep = 0` clears the log entirely.

### CLI Commands (`cmd/fleet/cmd_context.go`)

#### `fleet context log <message>`
- Requires `FLEET_AGENT_NAME` to be set (agent-only). Returns error otherwise.
- Calls `AppendLog(fleetDir, agentName, message)`.

#### `fleet context trim [--keep N]`
- `--keep` flag, default `500`.
- No agent/user restriction — can be run from anywhere.
- With no `--channel` flag, trims the shared log.
- Prints how many entries remain after trim (or "log already within limit" if no-op).

#### `fleet context read` (updated)
- After the existing `== Shared Context ==` and `== <agent> ==` sections, prints a new `== Agent Log ==` section.
- Each entry formatted as: `[2026-04-04T12:00:00Z] [agent-name] message`
- Section omitted entirely if `Log` is empty.

### Error Handling

| Scenario | Behavior |
|---|---|
| `log` with empty message | Error: "message cannot be empty" |
| `trim` when log ≤ keep entries | No-op, no error |
| `trim --keep 0` | Clears log entirely |
| Concurrent appends | Handled by existing flock |
| `log` without `FLEET_AGENT_NAME` | Error: "must be run from within a fleet agent session" |

---

## Part 2: Private Channels

### Data Model

```go
type Channel struct {
    Name        string     `json:"name"`
    Description string     `json:"description"`
    Members     []string   `json:"members"`
    Log         []LogEntry `json:"log,omitempty"`
}
```

Channels are stored in `Context.Channels` keyed by channel name. Membership is fixed at creation. Members are stored as-provided (agent names).

**Naming rules:**
- 2-member channels: name is always auto-set to `dm-[agent1]-[agent2]` regardless of the `name` argument. The brackets are literal (e.g., `dm-[alice]-[bob]`).
- 3+ member channels: name is whatever was passed to `channel-create`.

### New Functions (`internal/context/context.go`)

#### `CreateChannel(fleetDir, name, description string, members []string) error`
- Requires at least 2 members. Returns error if fewer.
- For 2-member channels, ignores `name` and sets it to `dm-[member1]-[member2]`.
- Returns error if a channel with the resolved name already exists.
- Returns error if any member string is empty.
- Membership is immutable after creation.

#### `SendToChannel(fleetDir, channelName, agentName, message string) error`
- Returns error if channel does not exist.
- Returns error if `agentName` is not in `Members`.
- Returns error if `message` is empty.
- Appends `LogEntry` with `time.Now().UTC()` under flock.

#### `TrimChannel(fleetDir, channelName string, keep int) error`
- Same semantics as `TrimLog`, scoped to the named channel's log.
- Returns error if channel does not exist.

### CLI Commands

#### `fleet context channel-create <name> <agent1> <agent2> [agent3...] [--description "..."]`
- For 2-member channels, `name` is ignored; prints the resolved name on success.
- For 3+ member channels, uses the provided name.
- `--description` flag, optional, defaults to empty string.
- No agent/user restriction — channels can be created from anywhere.

#### `fleet context channel-send <channel-name> <message>`
- Requires `FLEET_AGENT_NAME`. Returns error otherwise.
- Sender must be a member of the channel.
- Calls `SendToChannel`.

#### `fleet context channel-read <channel-name>`
- Open to anyone (agents and orchestrator).
- Prints channel description, member list, and full attributed log.
- Each entry formatted as: `[2026-04-04T12:00:00Z] [agent-name] message`

#### `fleet context channel-list`
- Lists all channels: name, description, member count, message count.
- Open to anyone.

#### `fleet context trim --channel <name> [--keep N]`
- Reuses the trim command. `--channel` flag scopes trim to a specific channel's log.
- Default `--keep 500`.

### Error Handling

| Scenario | Behavior |
|---|---|
| `channel-create` with < 2 members | Error: "channel requires at least 2 members" |
| `channel-create` with duplicate name | Error: "channel already exists" |
| `channel-send` from non-member | Error: "agent is not a member of this channel" |
| `channel-send` without `FLEET_AGENT_NAME` | Error: "must be run from within a fleet agent session" |
| `channel-read` / `channel-list` on empty | Prints empty state message, no error |
| `trim --channel` on non-existent channel | Error: "channel not found" |
| Concurrent channel sends | Handled by existing flock |

---

## Out of Scope

- Automatic trimming on append
- Log streaming / tail -f
- Per-agent log filtering
- Adding members to a channel after creation
- Deleting channels
