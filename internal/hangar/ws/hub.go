package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/hangar/security"
	"github.com/MrBenJ/fleet-commander/internal/monitor"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

type Hub struct {
	clients    map[*websocket.Conn]bool
	mu         sync.RWMutex
	closed     bool
	fleetDir   string
	repoPath   string
	logger     *log.Logger
	lastLogLen map[string]int
	monitor    *monitor.Monitor
	lastStates map[string]string
	upgrader   websocket.Upgrader
}

// NewHub constructs a Hub with a permissive origin check, intended for tests.
// Production callers should use NewHubWithValidator.
func NewHub(fleetDir, repoPath, tmuxPrefix string, logger *log.Logger) *Hub {
	return NewHubWithValidator(fleetDir, repoPath, tmuxPrefix, logger, security.New(true))
}

// NewHubWithValidator constructs a Hub whose WebSocket upgrader rejects
// cross-origin requests according to the supplied validator.
func NewHubWithValidator(fleetDir, repoPath, tmuxPrefix string, logger *log.Logger, v *security.Validator) *Hub {
	tm := tmux.NewManager(tmuxPrefix)
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		fleetDir:   fleetDir,
		repoPath:   repoPath,
		logger:     logger,
		lastLogLen: make(map[string]int),
		monitor:    monitor.NewMonitor(tm),
		lastStates: make(map[string]string),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return v.AllowWebSocket(r)
			},
		},
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		http.Error(w, "WebSocket upgrade required", http.StatusUpgradeRequired)
		return
	}

	// Refuse new connections once shutdown has started — otherwise a client
	// can race in between Shutdown's close and the listener stopping.
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		return
	}
	h.mu.Unlock()

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	h.mu.Lock()
	if h.closed {
		// Lost the race: Shutdown started after we passed the first check
		// but before we registered. Close the freshly-upgraded conn and bail.
		h.mu.Unlock()
		conn.Close()
		return
	}
	h.clients[conn] = true
	h.mu.Unlock()

	h.logger.Printf("Browser connected (%d clients)", h.ClientCount())

	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
			h.logger.Printf("Browser disconnected (%d clients)", h.ClientCount())
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

func (h *Hub) PollLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.pollChannels()
			h.pollAgentStates()
		}
	}
}

// Shutdown closes every connected WebSocket client and refuses new
// connections. Idempotent: a second call is a no-op. Returns the count of
// connections that were closed so callers can log a meaningful summary.
func (h *Hub) Shutdown() int {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return 0
	}
	h.closed = true
	closing := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		closing = append(closing, c)
	}
	// Empty the map up front so concurrent broadcast iterations see a clean
	// state once we release the lock.
	h.clients = map[*websocket.Conn]bool{}
	h.mu.Unlock()

	for _, c := range closing {
		// Best-effort: send a close frame so well-behaved clients get a
		// reason, then forcibly close. We don't care about errors here —
		// the connection is going away regardless.
		_ = c.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"))
		c.Close()
	}
	return len(closing)
}

func (h *Hub) pollAgentStates() {
	f, err := fleet.Load(h.repoPath)
	if err != nil {
		return
	}

	for _, a := range f.Agents {
		snap := h.monitor.CheckWithStateFile(a.Name, a.StateFilePath)
		state := string(snap.State)

		if prev, ok := h.lastStates[a.Name]; !ok || prev != state {
			h.lastStates[a.Name] = state
			h.Broadcast(Event{
				Type:      "agent_state",
				Agent:     a.Name,
				State:     state,
				Timestamp: snap.Timestamp.Format(time.RFC3339),
			})
		}
	}
}

func (h *Hub) pollChannels() {
	fctx, err := fleetctx.Load(h.fleetDir)
	if err != nil {
		return
	}

	for name, ch := range fctx.Channels {
		lastLen, exists := h.lastLogLen[name]
		if !exists {
			lastLen = 0
		}

		if len(ch.Log) > lastLen {
			for _, entry := range ch.Log[lastLen:] {
				h.Broadcast(Event{
					Type:      "context_message",
					Agent:     entry.Agent,
					Message:   entry.Message,
					Timestamp: entry.Timestamp.Format(time.RFC3339),
				})
			}
			h.lastLogLen[name] = len(ch.Log)
		}
	}
}
