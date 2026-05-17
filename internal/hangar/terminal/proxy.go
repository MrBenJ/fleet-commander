package terminal

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"github.com/MrBenJ/fleet-commander/internal/hangar/security"
)

type Proxy struct {
	tmuxPrefix string
	logger     *log.Logger
	upgrader   websocket.Upgrader

	mu     sync.Mutex
	closed bool
	// active tracks in-flight terminal sessions so Shutdown can drain them.
	// The value is the cleanup func registered when the session started.
	active map[uint64]func()
	nextID uint64
}

// NewProxy constructs a Proxy with a permissive origin check, intended for
// tests. Production callers should use NewProxyWithValidator.
func NewProxy(tmuxPrefix string, logger *log.Logger) *Proxy {
	return NewProxyWithValidator(tmuxPrefix, logger, security.New(true, ""))
}

// NewProxyWithValidator constructs a Proxy whose WebSocket upgrader rejects
// cross-origin requests using the supplied validator. This is critical for
// the terminal proxy because it bridges a WebSocket directly to a shell PTY.
func NewProxyWithValidator(tmuxPrefix string, logger *log.Logger, v *security.Validator) *Proxy {
	return &Proxy{
		tmuxPrefix: tmuxPrefix,
		logger:     logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return v.AllowWebSocket(r)
			},
		},
		active: map[uint64]func(){},
	}
}

// register stores a cleanup callback for an in-flight terminal session and
// returns an unregister handle. Callers must invoke the handle when the
// session ends so we don't leak entries — and must not invoke their own
// cleanup if Shutdown already ran (Shutdown takes ownership of all entries
// it sees).
func (p *Proxy) register(cleanup func()) (id uint64, unregister func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		// Shutdown already started: run cleanup synchronously so the caller
		// doesn't leak resources, and return a no-op unregister.
		go cleanup()
		return 0, func() {}
	}
	p.nextID++
	id = p.nextID
	p.active[id] = cleanup
	return id, func() {
		p.mu.Lock()
		delete(p.active, id)
		p.mu.Unlock()
	}
}

// Shutdown drains all in-flight terminal sessions. Idempotent. Returns the
// number of sessions that were cleaned up so callers can log a summary.
func (p *Proxy) Shutdown() int {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0
	}
	p.closed = true
	cleanups := make([]func(), 0, len(p.active))
	for _, fn := range p.active {
		cleanups = append(cleanups, fn)
	}
	p.active = map[uint64]func(){}
	p.mu.Unlock()

	for _, fn := range cleanups {
		fn()
	}
	return len(cleanups)
}

// isClosed reports whether Shutdown has been called. Used by HandleTerminal
// to refuse new sessions during shutdown.
func (p *Proxy) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *Proxy) HandleTerminal(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		http.Error(w, "WebSocket upgrade required", http.StatusUpgradeRequired)
		return
	}

	if p.isClosed() {
		http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		return
	}

	// Extract agent name from path: /ws/terminal/{agent}
	agentName, ok := terminalAgentName(r.URL.Path)
	if !ok {
		http.Error(w, "missing agent name", http.StatusBadRequest)
		return
	}
	sessionName := fmt.Sprintf("%s-%s", p.tmuxPrefix, agentName)

	// Check session exists
	check := exec.Command("tmux", "has-session", "-t", sessionName)
	if err := check.Run(); err != nil {
		http.Error(w, fmt.Sprintf("agent session %q not running", agentName), http.StatusNotFound)
		return
	}

	// Upgrade to WebSocket
	conn, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Printf("terminal upgrade failed for %s: %v", agentName, err)
		return
	}

	p.logger.Printf("Terminal opened → %s", agentName)

	// Start tmux attach with a pty
	cmd := exec.Command("tmux", "attach-session", "-t", sessionName)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		p.logger.Printf("pty start failed for %s: %v", agentName, err)
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "failed to attach"))
		conn.Close()
		return
	}

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			ptmx.Close()
			cmd.Process.Signal(os.Interrupt)
			cmd.Wait()
			conn.Close()
			p.logger.Printf("Terminal closed → %s", agentName)
		})
	}
	defer cleanup()

	// Register the session so Shutdown can tear it down on server exit.
	_, unregister := p.register(cleanup)
	defer unregister()

	// pty → WebSocket (stdout to browser)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				cleanup()
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				cleanup()
				return
			}
		}
	}()

	// WebSocket → pty (browser to stdin)
	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		switch msgType {
		case websocket.BinaryMessage, websocket.TextMessage:
			if len(msg) > 0 && msg[0] == '{' {
				if ws := parseResize(msg); ws != nil {
					pty.Setsize(ptmx, ws)
					continue
				}
			}
			ptmx.Write(msg)
		}
	}
}

func terminalAgentName(path string) (string, bool) {
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[1] != "ws" || parts[2] != "terminal" {
		return "", false
	}
	name := strings.TrimSpace(parts[3])
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", false
	}
	return name, true
}

func parseResize(msg []byte) *pty.Winsize {
	type resizeMsg struct {
		Cols uint16 `json:"cols"`
		Rows uint16 `json:"rows"`
	}
	var r resizeMsg
	if err := json.Unmarshal(msg, &r); err != nil {
		return nil
	}
	if r.Cols > 0 && r.Rows > 0 {
		return &pty.Winsize{Cols: r.Cols, Rows: r.Rows}
	}
	return nil
}
