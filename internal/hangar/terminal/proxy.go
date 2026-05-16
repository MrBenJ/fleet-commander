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
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Proxy struct {
	tmuxPrefix string
	logger     *log.Logger
}

func NewProxy(tmuxPrefix string, logger *log.Logger) *Proxy {
	return &Proxy{
		tmuxPrefix: tmuxPrefix,
		logger:     logger,
	}
}

func (p *Proxy) HandleTerminal(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		http.Error(w, "WebSocket upgrade required", http.StatusUpgradeRequired)
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
	conn, err := upgrader.Upgrade(w, r, nil)
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
