# Sidebar Navigation + Multi-Squadron Support

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the full-screen wizard/mission toggle with a collapsible left sidebar where each active squadron is a menu item, and the detail pane shows that squadron's mission control or the wizard.

**Architecture:** The sidebar is a new React component tree rendered alongside a detail pane in a flex layout. The backend gets a squadron listing endpoint and populates the existing `Squadron` field on WebSocket events so the frontend can filter messages per-squadron. A single WebSocket connection at the App level dispatches events to the correct squadron's state.

**Tech Stack:** React 18 + TypeScript (inline styles, CSS variables), Go HTTP API, gorilla/websocket

---

### Task 1: Populate `Squadron` field in WebSocket events

**Files:**
- Modify: `internal/hangar/ws/hub.go:1-132`

- [ ] **Step 1: Write the failing test**

```go
// internal/hangar/ws/hub_test.go
package ws

import (
	"testing"
)

func TestExtractSquadronName(t *testing.T) {
	tests := []struct {
		channelName string
		want        string
	}{
		{"squadron-alpha", "alpha"},
		{"squadron-mega-squad", "mega-squad"},
		{"dm-[a]-[b]", ""},
		{"random-channel", ""},
		{"squadron-", ""},
	}
	for _, tt := range tests {
		got := extractSquadronName(tt.channelName)
		if got != tt.want {
			t.Errorf("extractSquadronName(%q) = %q, want %q", tt.channelName, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hangar/ws/ -run TestExtractSquadronName -v`
Expected: FAIL — `extractSquadronName` not defined

- [ ] **Step 3: Add `extractSquadronName` helper and populate Squadron on broadcast**

In `internal/hangar/ws/hub.go`, add import `"strings"` and the helper function:

```go
func extractSquadronName(channelName string) string {
	if !strings.HasPrefix(channelName, "squadron-") {
		return ""
	}
	name := strings.TrimPrefix(channelName, "squadron-")
	if name == "" {
		return ""
	}
	return name
}
```

Then update `pollChannels()` lines 121-128 to set the Squadron field:

```go
for _, entry := range ch.Log[lastLen:] {
	h.Broadcast(Event{
		Type:      "context_message",
		Squadron:  extractSquadronName(name),
		Agent:     entry.Agent,
		Message:   entry.Message,
		Timestamp: entry.Timestamp.Format(time.RFC3339),
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/hangar/ws/ -run TestExtractSquadronName -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/hangar/ws/hub.go internal/hangar/ws/hub_test.go
git commit -m "feat(ws): populate Squadron field on context_message events"
```

---

### Task 2: Add `GET /api/squadrons` endpoint

**Files:**
- Modify: `internal/hangar/api/handlers.go`
- Modify: `internal/hangar/api/types.go`
- Modify: `internal/hangar/server.go:61-84`

- [ ] **Step 1: Write the failing test**

```go
// internal/hangar/api/handlers_test.go (append to existing file or create)
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleListSquadrons(t *testing.T) {
	// Set up a temp fleet dir with a context.json containing squadron channels
	tmpDir := t.TempDir()
	contextJSON := `{
		"shared": "",
		"agents": {},
		"channels": {
			"squadron-alpha": {
				"name": "squadron-alpha",
				"description": "Alpha squadron",
				"members": ["agent-a", "agent-b"]
			},
			"squadron-beta": {
				"name": "squadron-beta",
				"description": "Beta squadron",
				"members": ["agent-c"]
			},
			"dm-[x]-[y]": {
				"name": "dm-[x]-[y]",
				"description": "DM channel",
				"members": ["x", "y"]
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, "context.json"), []byte(contextJSON), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handlers{fleetDir: tmpDir}
	req := httptest.NewRequest(http.MethodGet, "/api/squadrons", nil)
	w := httptest.NewRecorder()

	h.HandleListSquadrons(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var result []SquadronListItem
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d squadrons, want 2", len(result))
	}

	// Check that DM channel was filtered out
	for _, sq := range result {
		if sq.Name == "dm-[x]-[y]" {
			t.Error("DM channel should not appear in squadron list")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hangar/api/ -run TestHandleListSquadrons -v`
Expected: FAIL — `HandleListSquadrons` and `SquadronListItem` not defined

- [ ] **Step 3: Add the response type**

In `internal/hangar/api/types.go`, add at the end:

```go
type SquadronListItem struct {
	Name   string   `json:"name"`
	Agents []string `json:"agents"`
}
```

- [ ] **Step 4: Add the handler**

In `internal/hangar/api/handlers.go`, add after `HandleSquadronStatus` (after line 348):

```go
// HandleListSquadrons handles GET /api/squadrons — returns active squadron names and their agents.
func (h *Handlers) HandleListSquadrons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, err := fleetctx.Load(h.fleetDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load context: %v", err))
		return
	}

	var squadrons []SquadronListItem
	for name, ch := range ctx.Channels {
		if !strings.HasPrefix(name, "squadron-") {
			continue
		}
		sqName := strings.TrimPrefix(name, "squadron-")
		if sqName == "" {
			continue
		}
		squadrons = append(squadrons, SquadronListItem{
			Name:   sqName,
			Agents: ch.Members,
		})
	}

	if squadrons == nil {
		squadrons = []SquadronListItem{}
	}

	writeJSON(w, http.StatusOK, squadrons)
}
```

- [ ] **Step 5: Register the route**

In `internal/hangar/server.go`, add after line 70 (`/api/squadron/launch`):

```go
s.mux.HandleFunc("/api/squadrons", s.api.HandleListSquadrons)
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/hangar/api/ -run TestHandleListSquadrons -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/hangar/api/handlers.go internal/hangar/api/types.go internal/hangar/api/handlers_test.go internal/hangar/server.go
git commit -m "feat(api): add GET /api/squadrons endpoint for listing active squadrons"
```

---

### Task 3: Broadcast `squadron_launched` event after launch

**Files:**
- Modify: `internal/hangar/api/handlers.go:20-29`
- Modify: `internal/hangar/server.go:52`

- [ ] **Step 1: Add `hub` field to Handlers**

In `internal/hangar/api/handlers.go`, add a hub field and update the constructor:

```go
import (
	// ... existing imports ...
	"github.com/MrBenJ/fleet-commander/internal/hangar/ws"
)

type Handlers struct {
	repoPath string
	fleetDir string
	hub      *ws.Hub
}

func NewHandlers(repoPath, fleetDir string, hub *ws.Hub) *Handlers {
	return &Handlers{repoPath: repoPath, fleetDir: fleetDir, hub: hub}
}
```

- [ ] **Step 2: Broadcast after successful launch**

In `HandleLaunchSquadron`, replace the `w.WriteHeader(http.StatusNoContent)` at line 238 with:

```go
// Broadcast squadron_launched event
agentNames := make([]string, len(req.Agents))
for i, a := range req.Agents {
	agentNames[i] = a.Name
}
if h.hub != nil {
	h.hub.Broadcast(ws.Event{
		Type:     "squadron_launched",
		Squadron: req.Name,
		Agents:   agentNames,
	})
}

w.WriteHeader(http.StatusNoContent)
```

- [ ] **Step 3: Update server.go to pass hub to NewHandlers**

In `internal/hangar/server.go`, line 52, change:

```go
api:      api.NewHandlers(cfg.RepoPath, cfg.FleetDir),
```
to:
```go
api:      nil, // placeholder — set below after hub is created
```

Actually, since `hub` is created on line 53, reorder. Create hub first, then pass it:

```go
func NewServer(cfg Config) *Server {
	logger := log.New(log.Writer(), "[hangar] ", log.LstdFlags)
	hub := ws.NewHub(cfg.FleetDir, logger)
	s := &Server{
		port:     cfg.Port,
		devMode:  cfg.DevMode,
		webFS:    cfg.WebFS,
		fleetDir: cfg.FleetDir,
		mux:      http.NewServeMux(),
		logger:   logger,
		api:      api.NewHandlers(cfg.RepoPath, cfg.FleetDir, hub),
		hub:      hub,
		terminal: terminal.NewProxy(cfg.TmuxPrefix, logger),
		LogCh:    make(chan string, 100),
	}
	s.routes()
	return s
}
```

- [ ] **Step 4: Fix any test compilation issues**

If `TestHandleListSquadrons` from Task 2 creates `&Handlers{fleetDir: tmpDir}`, update it to `&Handlers{fleetDir: tmpDir, hub: nil}` (hub is nil-safe).

Run: `go test ./internal/hangar/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/hangar/api/handlers.go internal/hangar/server.go internal/hangar/api/handlers_test.go
git commit -m "feat(api): broadcast squadron_launched event after successful launch"
```

---

### Task 4: Add agent state polling to WebSocket hub

**Files:**
- Modify: `internal/hangar/ws/hub.go`
- Modify: `internal/hangar/ws/events.go`
- Modify: `internal/hangar/server.go`

- [ ] **Step 1: Write the test**

```go
// internal/hangar/ws/hub_test.go (append)
func TestWorstAgentState(t *testing.T) {
	tests := []struct {
		states []string
		want   string
	}{
		{[]string{"working", "working"}, "working"},
		{[]string{"working", "waiting"}, "waiting"},
		{[]string{"working", "stopped"}, "stopped"},
		{[]string{"waiting", "starting"}, "waiting"},
		{[]string{"starting", "starting"}, "starting"},
		{[]string{}, "starting"},
	}
	for _, tt := range tests {
		got := worstState(tt.states)
		if got != tt.want {
			t.Errorf("worstState(%v) = %q, want %q", tt.states, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hangar/ws/ -run TestWorstAgentState -v`
Expected: FAIL — `worstState` not defined

- [ ] **Step 3: Add hub fields for agent state tracking and monitor integration**

In `internal/hangar/ws/hub.go`, add new imports and fields:

```go
import (
	// ... existing imports ...
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/monitor"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

type Hub struct {
	clients         map[*websocket.Conn]bool
	mu              sync.RWMutex
	fleetDir        string
	repoPath        string
	tmuxPrefix      string
	logger          *log.Logger
	lastLogLen      map[string]int
	lastAgentStates map[string]string
}

func NewHub(fleetDir, repoPath, tmuxPrefix string, logger *log.Logger) *Hub {
	return &Hub{
		clients:         make(map[*websocket.Conn]bool),
		fleetDir:        fleetDir,
		repoPath:        repoPath,
		tmuxPrefix:      tmuxPrefix,
		logger:          logger,
		lastLogLen:      make(map[string]int),
		lastAgentStates: make(map[string]string),
	}
}
```

Add `worstState` helper:

```go
func worstState(states []string) string {
	// Priority: stopped > waiting > starting > working
	priority := map[string]int{"stopped": 3, "waiting": 2, "starting": 1, "working": 0}
	worst := "starting"
	worstP := -1
	for _, s := range states {
		p, ok := priority[s]
		if !ok {
			p = 1 // unknown treated as starting
		}
		if p > worstP {
			worstP = p
			worst = s
		}
	}
	return worst
}
```

- [ ] **Step 4: Add agent state polling to pollChannels**

At the end of `pollChannels()`, after the channel log loop, add agent state polling:

```go
func (h *Hub) pollAgentStates() {
	if h.repoPath == "" {
		return
	}
	f, err := fleet.Load(h.repoPath)
	if err != nil {
		return
	}

	tm := tmux.NewManager(f.TmuxPrefix())
	mon := monitor.NewMonitor(tm)

	// Build squadron membership from context channels
	fctx, err := fleetctx.Load(h.fleetDir)
	if err != nil {
		return
	}
	agentSquadron := make(map[string]string)
	for name, ch := range fctx.Channels {
		sq := extractSquadronName(name)
		if sq == "" {
			continue
		}
		for _, member := range ch.Members {
			agentSquadron[member] = sq
		}
	}

	for _, agent := range f.Agents {
		snap := mon.CheckWithStateFile(agent.Name, agent.StateFilePath)
		newState := string(snap.State)
		oldState := h.lastAgentStates[agent.Name]
		if newState != oldState {
			h.lastAgentStates[agent.Name] = newState
			h.Broadcast(Event{
				Type:      "agent_state",
				Squadron:  agentSquadron[agent.Name],
				Agent:     agent.Name,
				State:     newState,
				Timestamp: snap.Timestamp.Format(time.RFC3339),
			})
		}
	}
}
```

Call `h.pollAgentStates()` at the end of `pollChannels()`.

- [ ] **Step 5: Update server.go to pass new Hub constructor args**

In `internal/hangar/server.go`, update the hub creation:

```go
hub := ws.NewHub(cfg.FleetDir, cfg.RepoPath, cfg.TmuxPrefix, logger)
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/hangar/... -v`
Expected: PASS

Run: `go build ./...`
Expected: compiles without errors

- [ ] **Step 7: Commit**

```bash
git add internal/hangar/ws/hub.go internal/hangar/ws/hub_test.go internal/hangar/server.go
git commit -m "feat(ws): add agent state polling and broadcast agent_state events with squadron field"
```

---

### Task 5: Frontend types and API client updates

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Update WSEvent types to include squadron field**

In `web/src/types.ts`, replace the `WSEvent` type (lines 47-51):

```typescript
export interface ActiveSquadron {
  name: string;
  agents: SquadronAgent[];
  config: { consensus: string; autoMerge: boolean };
}

export type WSEvent =
  | { type: "context_message"; squadron?: string; agent: string; message: string; timestamp: string }
  | { type: "agent_state"; squadron?: string; agent: string; state: string; timestamp: string }
  | { type: "squadron_launched"; squadron: string; agents: string[] }
  | { type: "agent_stopped"; agent: string };
```

- [ ] **Step 2: Add getSquadrons API function**

In `web/src/api.ts`, add at the end:

```typescript
export async function getSquadrons(): Promise<{ name: string; agents: string[] }[]> {
  return fetchJSON("/api/squadrons");
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: no errors (or only pre-existing errors)

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat(web): add ActiveSquadron type, squadron field on WSEvent, getSquadrons API"
```

---

### Task 6: Create `useSquadronWebSocket` hook

**Files:**
- Create: `web/src/hooks/useSquadronWebSocket.ts`

This hook replaces per-MissionControl WebSocket usage. It connects once at the App level and dispatches events to per-squadron state.

- [ ] **Step 1: Create the hook**

```typescript
// web/src/hooks/useSquadronWebSocket.ts
import { useState, useCallback, useRef, useEffect } from "react";
import type { ContextMessage, WSEvent } from "../types";

interface SquadronWebSocketState {
  messages: Record<string, ContextMessage[]>;
  agentStates: Record<string, string>;
  connected: boolean;
}

interface UseSquadronWebSocketOptions {
  onSquadronLaunched?: (name: string, agents: string[]) => void;
}

export function useSquadronWebSocket(options: UseSquadronWebSocketOptions = {}) {
  const [state, setState] = useState<SquadronWebSocketState>({
    messages: {},
    agentStates: {},
    connected: false,
  });
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<number | undefined>(undefined);
  const onSquadronLaunchedRef = useRef(options.onSquadronLaunched);
  onSquadronLaunchedRef.current = options.onSquadronLaunched;

  const handleEvent = useCallback((event: WSEvent) => {
    switch (event.type) {
      case "context_message": {
        const sq = event.squadron;
        if (!sq) return;
        setState((prev) => ({
          ...prev,
          messages: {
            ...prev.messages,
            [sq]: [
              ...(prev.messages[sq] || []),
              { agent: event.agent, message: event.message, timestamp: event.timestamp },
            ],
          },
        }));
        break;
      }
      case "agent_state":
        setState((prev) => ({
          ...prev,
          agentStates: { ...prev.agentStates, [event.agent]: event.state },
        }));
        break;
      case "squadron_launched":
        onSquadronLaunchedRef.current?.(event.squadron, event.agents);
        break;
      case "agent_stopped":
        setState((prev) => ({
          ...prev,
          agentStates: { ...prev.agentStates, [event.agent]: "stopped" },
        }));
        break;
    }
  }, []);

  const connect = useCallback(() => {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${protocol}//${window.location.host}/ws/events`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setState((prev) => ({ ...prev, connected: true }));
    };

    ws.onmessage = (evt) => {
      try {
        const data: WSEvent = JSON.parse(evt.data);
        handleEvent(data);
      } catch {
        // Binary or unparseable — ignore
      }
    };

    ws.onclose = () => {
      setState((prev) => ({ ...prev, connected: false }));
      reconnectTimer.current = window.setTimeout(connect, 2000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [handleEvent]);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const getMessagesForSquadron = useCallback(
    (name: string): ContextMessage[] => state.messages[name] || [],
    [state.messages]
  );

  return {
    connected: state.connected,
    agentStates: state.agentStates,
    getMessagesForSquadron,
    allMessages: state.messages,
  };
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useSquadronWebSocket.ts
git commit -m "feat(web): add useSquadronWebSocket hook for App-level WebSocket with per-squadron dispatch"
```

---

### Task 7: Create Sidebar components

**Files:**
- Create: `web/src/components/sidebar/Sidebar.tsx`
- Create: `web/src/components/sidebar/SquadronSidebarItem.tsx`
- Create: `web/src/components/sidebar/NewSquadronButton.tsx`

- [ ] **Step 1: Create SquadronSidebarItem**

```typescript
// web/src/components/sidebar/SquadronSidebarItem.tsx
import { useState } from "react";
import type { ActiveSquadron } from "../../types";

interface SquadronSidebarItemProps {
  squadron: ActiveSquadron;
  agentStates: Record<string, string>;
  isActive: boolean;
  collapsed: boolean;
  onClick: () => void;
}

const stateColors: Record<string, string> = {
  working: "var(--green)",
  waiting: "var(--orange)",
  stopped: "var(--red)",
  starting: "var(--text-muted)",
};

const statePriority: Record<string, number> = {
  stopped: 3,
  waiting: 2,
  starting: 1,
  working: 0,
};

function getWorstState(agents: { name: string }[], agentStates: Record<string, string>): string {
  let worst = "starting";
  let worstP = -1;
  for (const a of agents) {
    const s = agentStates[a.name] || "starting";
    const p = statePriority[s] ?? 1;
    if (p > worstP) {
      worstP = p;
      worst = s;
    }
  }
  return worst;
}

export function SquadronSidebarItem({
  squadron,
  agentStates,
  isActive,
  collapsed,
  onClick,
}: SquadronSidebarItemProps) {
  const [hovered, setHovered] = useState(false);
  const worstState = getWorstState(squadron.agents, agentStates);
  const dotColor = stateColors[worstState] || stateColors.starting;
  const initial = squadron.name.charAt(0).toUpperCase();

  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      title={collapsed ? `${squadron.name} (${worstState})` : undefined}
      aria-label={`Squadron ${squadron.name}, status: ${worstState}`}
      aria-current={isActive ? "page" : undefined}
      style={{
        width: "100%",
        display: "flex",
        alignItems: "center",
        gap: "0.75rem",
        padding: collapsed ? "0.6rem 0" : "0.6rem 0.75rem",
        justifyContent: collapsed ? "center" : "flex-start",
        background: isActive
          ? "var(--bg-tertiary)"
          : hovered
            ? "var(--bg-secondary)"
            : "transparent",
        border: "none",
        borderLeft: isActive ? "2px solid var(--blue)" : "2px solid transparent",
        cursor: "pointer",
        color: "var(--text-primary)",
        fontSize: "0.85rem",
        fontFamily: "inherit",
        transition: "background 0.15s ease",
      }}
    >
      <span
        style={{
          width: 28,
          height: 28,
          borderRadius: 6,
          background: "var(--bg-secondary)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: "0.8rem",
          fontWeight: 700,
          flexShrink: 0,
          position: "relative",
        }}
      >
        {initial}
        <span
          aria-hidden="true"
          style={{
            position: "absolute",
            bottom: -1,
            right: -1,
            width: 8,
            height: 8,
            borderRadius: "50%",
            background: dotColor,
            border: "2px solid var(--bg-primary)",
            animation: worstState === "waiting" ? "pulse 2s infinite" : undefined,
          }}
        />
      </span>
      {!collapsed && (
        <span
          style={{
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {squadron.name}
        </span>
      )}
    </button>
  );
}
```

- [ ] **Step 2: Create NewSquadronButton**

```typescript
// web/src/components/sidebar/NewSquadronButton.tsx
import { useState } from "react";

interface NewSquadronButtonProps {
  isActive: boolean;
  collapsed: boolean;
  onClick: () => void;
}

export function NewSquadronButton({ isActive, collapsed, onClick }: NewSquadronButtonProps) {
  const [hovered, setHovered] = useState(false);

  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      aria-label="New Squadron"
      aria-current={isActive ? "page" : undefined}
      style={{
        width: "100%",
        display: "flex",
        alignItems: "center",
        gap: "0.75rem",
        padding: collapsed ? "0.6rem 0" : "0.6rem 0.75rem",
        justifyContent: collapsed ? "center" : "flex-start",
        background: isActive
          ? "var(--bg-tertiary)"
          : hovered
            ? "var(--bg-secondary)"
            : "transparent",
        border: "none",
        borderLeft: isActive ? "2px solid var(--blue)" : "2px solid transparent",
        cursor: "pointer",
        color: "var(--blue)",
        fontSize: "0.85rem",
        fontFamily: "inherit",
        transition: "background 0.15s ease",
      }}
    >
      <span
        style={{
          width: 28,
          height: 28,
          borderRadius: 6,
          background: "var(--bg-secondary)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: "1rem",
          fontWeight: 700,
          flexShrink: 0,
        }}
      >
        +
      </span>
      {!collapsed && <span>New Squadron</span>}
    </button>
  );
}
```

- [ ] **Step 3: Create Sidebar**

```typescript
// web/src/components/sidebar/Sidebar.tsx
import type { ActiveSquadron } from "../../types";
import { SquadronSidebarItem } from "./SquadronSidebarItem";
import { NewSquadronButton } from "./NewSquadronButton";

interface SidebarProps {
  squadrons: ActiveSquadron[];
  agentStates: Record<string, string>;
  selectedView: string;
  collapsed: boolean;
  onToggleCollapse: () => void;
  onSelectSquadron: (name: string) => void;
  onSelectWizard: () => void;
}

export function Sidebar({
  squadrons,
  agentStates,
  selectedView,
  collapsed,
  onToggleCollapse,
  onSelectSquadron,
  onSelectWizard,
}: SidebarProps) {
  return (
    <nav
      aria-label="Hangar navigation"
      style={{
        width: collapsed ? 48 : 220,
        height: "100vh",
        background: "var(--bg-primary)",
        borderRight: "1px solid var(--border)",
        display: "flex",
        flexDirection: "column",
        transition: "width 0.2s ease",
        flexShrink: 0,
        overflow: "hidden",
      }}
    >
      {/* Collapse toggle */}
      <div
        style={{
          padding: "0.5rem",
          display: "flex",
          justifyContent: collapsed ? "center" : "flex-end",
        }}
      >
        <button
          onClick={onToggleCollapse}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          style={{
            background: "none",
            border: "none",
            color: "var(--text-secondary)",
            cursor: "pointer",
            fontSize: "1rem",
            padding: "0.25rem 0.5rem",
            borderRadius: 4,
          }}
        >
          {collapsed ? "\u25B6" : "\u25C0"}
        </button>
      </div>

      {/* Squadron list */}
      <div
        style={{
          flex: 1,
          overflowY: "auto",
          overflowX: "hidden",
          padding: "0.25rem 0",
        }}
      >
        {squadrons.map((sq) => (
          <SquadronSidebarItem
            key={sq.name}
            squadron={sq}
            agentStates={agentStates}
            isActive={selectedView === sq.name}
            collapsed={collapsed}
            onClick={() => onSelectSquadron(sq.name)}
          />
        ))}
      </div>

      {/* New Squadron at bottom */}
      <div style={{ borderTop: "1px solid var(--border)", padding: "0.25rem 0" }}>
        <NewSquadronButton
          isActive={selectedView === "wizard"}
          collapsed={collapsed}
          onClick={onSelectWizard}
        />
      </div>
    </nav>
  );
}
```

- [ ] **Step 4: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add web/src/components/sidebar/Sidebar.tsx web/src/components/sidebar/SquadronSidebarItem.tsx web/src/components/sidebar/NewSquadronButton.tsx
git commit -m "feat(web): add collapsible Sidebar with SquadronSidebarItem and NewSquadronButton"
```

---

### Task 8: Update MissionControl to receive messages and agentStates via props

**Files:**
- Modify: `web/src/components/mission/MissionControl.tsx:1-155`

The goal is to remove the internal WebSocket connection and message/agentStates state so these can be managed at the App level.

- [ ] **Step 1: Update props interface**

Replace the `MissionControlProps` interface (lines 8-14) and remove internal WebSocket/state management:

```typescript
interface MissionControlProps {
  squadronName: string;
  agents: SquadronAgent[];
  personas: Persona[];
  consensus: string;
  autoMerge: boolean;
  messages: ContextMessage[];
  agentStates: Record<string, string>;
  connected: boolean;
}
```

- [ ] **Step 2: Remove internal state and WebSocket usage**

Remove these lines from the component body (lines 32-61):
- `const [messages, setMessages] = useState<ContextMessage[]>([]);`
- `const [agentStates, setAgentStates] = useState<Record<string, string>>({});`
- The entire `handleEvent` callback
- The `useWebSocket` call

Add `messages`, `agentStates`, and `connected` to the destructured props.

Remove the `useWebSocket` import at line 3.

- [ ] **Step 3: Update root container height**

Change the root `div` style from `height: "100vh"` to `height: "100%"` (line 69).

- [ ] **Step 4: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: errors in App.tsx (expected — App.tsx will be updated in Task 9)

- [ ] **Step 5: Commit**

```bash
git add web/src/components/mission/MissionControl.tsx
git commit -m "refactor(web): lift MissionControl WebSocket state to props"
```

---

### Task 9: Refactor App.tsx — sidebar + detail pane layout with multi-squadron

**Files:**
- Modify: `web/src/App.tsx:1-83`

This is the integration task. App.tsx becomes the orchestrator: it owns the sidebar state, the squadron list, and the single WebSocket connection.

- [ ] **Step 1: Rewrite App.tsx**

Replace the entire contents of `web/src/App.tsx`:

```typescript
import { useState, useCallback, useEffect } from "react";
import { useFleet } from "./hooks/useFleet";
import { useSquadronWebSocket } from "./hooks/useSquadronWebSocket";
import { getSquadrons } from "./api";
import { WizardLayout } from "./components/wizard/WizardLayout";
import { MissionControl } from "./components/mission/MissionControl";
import { TerminalPage } from "./components/terminal/TerminalPage";
import { ThemeToggle } from "./components/ThemeToggle";
import { Sidebar } from "./components/sidebar/Sidebar";
import type { SquadronAgent, ActiveSquadron } from "./types";

const STORAGE_KEY_SQUADRONS = "fleet-hangar-squadrons";
const STORAGE_KEY_SIDEBAR = "fleet-hangar-sidebar-collapsed";

function loadStoredSquadrons(): ActiveSquadron[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_SQUADRONS);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveStoredSquadrons(squadrons: ActiveSquadron[]) {
  localStorage.setItem(STORAGE_KEY_SQUADRONS, JSON.stringify(squadrons));
}

export function App() {
  // Terminal pages render standalone — no sidebar
  if (window.location.pathname.startsWith("/terminal/")) {
    return (
      <>
        <ThemeToggle />
        <TerminalPage />
      </>
    );
  }

  const { fleet, personas, drivers, loading, error } = useFleet();
  const [squadrons, setSquadrons] = useState<ActiveSquadron[]>(loadStoredSquadrons);
  const [selectedView, setSelectedView] = useState<string>("wizard");
  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(
    () => localStorage.getItem(STORAGE_KEY_SIDEBAR) === "true"
  );

  // Persist squadrons to localStorage
  useEffect(() => {
    saveStoredSquadrons(squadrons);
  }, [squadrons]);

  // Persist sidebar collapsed state
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_SIDEBAR, String(sidebarCollapsed));
  }, [sidebarCollapsed]);

  // On mount, reconcile with backend (discover squadrons from prior sessions)
  useEffect(() => {
    getSquadrons()
      .then((serverSquadrons) => {
        setSquadrons((prev) => {
          const existing = new Set(prev.map((s) => s.name));
          const toAdd: ActiveSquadron[] = [];
          for (const sq of serverSquadrons) {
            if (!existing.has(sq.name)) {
              toAdd.push({
                name: sq.name,
                agents: sq.agents.map((name) => ({
                  name,
                  branch: "",
                  prompt: "",
                  driver: "",
                  persona: "",
                })),
                config: { consensus: "universal", autoMerge: false },
              });
            }
          }
          return toAdd.length > 0 ? [...prev, ...toAdd] : prev;
        });
      })
      .catch(() => {});
  }, []);

  const handleSquadronLaunched = useCallback((name: string, _agents: string[]) => {
    // The full squadron data is added via handleLaunched, so this is just
    // a notification for squadrons launched from other tabs/sessions.
    setSquadrons((prev) => {
      if (prev.some((s) => s.name === name)) return prev;
      return [
        ...prev,
        {
          name,
          agents: _agents.map((n) => ({
            name: n,
            branch: "",
            prompt: "",
            driver: "",
            persona: "",
          })),
          config: { consensus: "universal", autoMerge: false },
        },
      ];
    });
  }, []);

  const { connected, agentStates, getMessagesForSquadron } = useSquadronWebSocket({
    onSquadronLaunched: handleSquadronLaunched,
  });

  const handleLaunched = (
    name: string,
    agents: SquadronAgent[],
    config: { consensus: string; autoMerge: boolean }
  ) => {
    const newSquadron: ActiveSquadron = { name, agents, config };
    setSquadrons((prev) => {
      const filtered = prev.filter((s) => s.name !== name);
      return [...filtered, newSquadron];
    });
    setSelectedView(name);
  };

  if (loading) {
    return (
      <main
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minHeight: "100vh",
        }}
      >
        <div role="status" aria-live="polite" style={{ color: "var(--text-secondary)" }}>
          Loading fleet...
        </div>
      </main>
    );
  }

  if (error) {
    return (
      <main
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minHeight: "100vh",
        }}
      >
        <div role="alert" style={{ color: "var(--red)" }}>
          Error: {error}
        </div>
      </main>
    );
  }

  const activeSquadron = squadrons.find((s) => s.name === selectedView);

  return (
    <div style={{ display: "flex", height: "100vh" }}>
      <a href="#main-content" className="skip-nav">
        Skip to main content
      </a>
      <ThemeToggle />
      <Sidebar
        squadrons={squadrons}
        agentStates={agentStates}
        selectedView={selectedView}
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed((c) => !c)}
        onSelectSquadron={(name) => setSelectedView(name)}
        onSelectWizard={() => setSelectedView("wizard")}
      />
      <main
        id="main-content"
        style={{ flex: 1, overflow: "hidden", height: "100vh" }}
      >
        {selectedView === "wizard" && fleet && (
          <WizardLayout
            personas={personas}
            drivers={drivers}
            currentBranch={fleet.currentBranch}
            onLaunched={handleLaunched}
          />
        )}
        {activeSquadron && (
          <MissionControl
            squadronName={activeSquadron.name}
            agents={activeSquadron.agents}
            personas={personas}
            consensus={activeSquadron.config.consensus}
            autoMerge={activeSquadron.config.autoMerge}
            messages={getMessagesForSquadron(activeSquadron.name)}
            agentStates={agentStates}
            connected={connected}
          />
        )}
      </main>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 3: Verify the full build**

Run: `cd web && npm run build`
Expected: build succeeds

- [ ] **Step 4: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(web): refactor App.tsx to sidebar + detail pane layout with multi-squadron support"
```

---

### Task 10: Add pulse animation and verify full build

**Files:**
- Modify: `web/src/styles/index.css`

The pulse animation was previously injected by MissionControl via a `<style>` tag. Since MissionControl no longer always mounts first (the sidebar needs it too), move it to the global CSS.

- [ ] **Step 1: Add pulse keyframe to global CSS**

In `web/src/styles/index.css`, append at the end:

```css
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
```

- [ ] **Step 2: Remove the inline `<style>` tag from MissionControl**

In `web/src/components/mission/MissionControl.tsx`, remove the line:

```tsx
<style>{`@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }`}</style>
```

- [ ] **Step 3: Run full build**

Run: `go build ./... && cd web && npm run build`
Expected: both compile successfully

- [ ] **Step 4: Commit**

```bash
git add web/src/styles/index.css web/src/components/mission/MissionControl.tsx
git commit -m "refactor: move pulse animation to global CSS"
```

---

### Task 11: End-to-end verification

- [ ] **Step 1: Run Go tests**

Run: `go test ./...`
Expected: all tests pass

- [ ] **Step 2: Run full binary build**

Run: `make build-all`
Expected: builds successfully (web + Go binary with embedded SPA)

- [ ] **Step 3: Manual smoke test**

1. Run `fleet hangar --dev` in a repo with an initialized fleet
2. Verify the sidebar renders on the left with the "New Squadron" button at the bottom
3. Click "New Squadron" — wizard should appear in the detail pane
4. Launch a squadron via the wizard — it should appear as a sidebar item with a status dot
5. Click the squadron item — mission control should appear showing agent pills and context log
6. Collapse the sidebar — should show only the initial letter + status dot
7. Expand the sidebar — full name should reappear
8. Launch a second squadron — both should appear in the sidebar
9. Click between them — detail view should switch
10. Reload the page — squadrons should persist in the sidebar (from localStorage + backend reconciliation)

- [ ] **Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address smoke test findings"
```
