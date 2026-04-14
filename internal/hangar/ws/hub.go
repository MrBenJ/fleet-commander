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
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Hub struct {
	clients    map[*websocket.Conn]bool
	mu         sync.RWMutex
	fleetDir   string
	logger     *log.Logger
	lastLogLen map[string]int
}

func NewHub(fleetDir string, logger *log.Logger) *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		fleetDir:   fleetDir,
		logger:     logger,
		lastLogLen: make(map[string]int),
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	h.mu.Lock()
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
