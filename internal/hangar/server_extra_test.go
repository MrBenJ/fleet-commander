package hangar

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestServerPort verifies the Port accessor returns the configured port.
func TestServerPort(t *testing.T) {
	srv := NewServer(Config{Port: 4242})
	if srv.Port() != 4242 {
		t.Errorf("Port() = %d, want %d", srv.Port(), 4242)
	}
}

// TestServerPort_Zero verifies that Port() returns 0 when configured for any free port.
func TestServerPort_Zero(t *testing.T) {
	srv := NewServer(Config{Port: 0})
	if srv.Port() != 0 {
		t.Errorf("Port() = %d, want 0", srv.Port())
	}
}

// TestSPAHandlerRewritesPathsToRoot verifies that the SPA handler rewrites
// non-root, non-asset paths to "/" so client-side routes return index.html.
func TestSPAHandlerRewritesPathsToRoot(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    string
	}{
		{"client route", "/squadron/abc", "/"},
		{"deep client route", "/squadron/abc/mission", "/"},
		{"asset (has dot)", "/assets/main.js", "/assets/main.js"},
		{"index (root)", "/", "/"},
		{"png asset", "/logo.png", "/logo.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seenPath string
			downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenPath = r.URL.Path
			})
			h := &spaHandler{fs: downstream}

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if seenPath != tt.want {
				t.Errorf("path %q: rewrote to %q, want %q", tt.path, seenPath, tt.want)
			}
		})
	}
}

// TestServerSquadronRouting verifies that the /api/squadron/{name}/{action}
// router dispatches /status and /info to their handlers and returns plain
// 404 for unknown actions. Both handlers happen to also return 404 (with a
// JSON error body) when the squadron doesn't exist, so we distinguish the
// two by inspecting the response body.
func TestServerSquadronRouting(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	base := fmt.Sprintf("http://%s", srv.Addr())

	cases := []struct {
		path           string
		wantBodyHas    string // distinguishes handler-404 from mux-404
		wantPlainMux404 bool
	}{
		{path: "/api/squadron/foo/status", wantBodyHas: "channel"},
		{path: "/api/squadron/foo/info", wantBodyHas: "squadron"},
		{path: "/api/squadron/foo/garbage", wantPlainMux404: true},
	}
	for _, tc := range cases {
		resp, err := http.Get(base + tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		buf := make([]byte, 256)
		n, _ := resp.Body.Read(buf)
		resp.Body.Close()
		body := string(buf[:n])

		if tc.wantPlainMux404 {
			// Mux 404: not JSON
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s: expected 404, got %d", tc.path, resp.StatusCode)
			}
			if !strings.Contains(body, "404 page not found") {
				t.Errorf("%s: expected mux 404 body, got %q", tc.path, body)
			}
			continue
		}

		// Handler-404: JSON body containing the wantBodyHas marker.
		if !strings.Contains(body, tc.wantBodyHas) {
			t.Errorf("%s: expected handler response containing %q, got %q (status=%d)",
				tc.path, tc.wantBodyHas, body, resp.StatusCode)
		}
	}
}

// TestServerAgentRouting verifies that the /api/agent/{name}/{action} router
// dispatches to /stop or 404 correctly.
func TestServerAgentRouting(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	base := fmt.Sprintf("http://%s", srv.Addr())

	// /api/agent/foo/stop with GET — should 405 (handler reached, wrong method).
	resp, err := http.Get(base + "/api/agent/foo/stop")
	if err != nil {
		t.Fatalf("stop GET: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET /stop, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// /api/agent/foo/garbage — 404
	resp, err = http.Get(base + "/api/agent/foo/garbage")
	if err != nil {
		t.Fatalf("unknown action: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown agent action, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestTUIInit verifies the TUI Init returns a nil command (no startup work).
func TestTUIInit(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")
	if cmd := m.Init(); cmd != nil {
		t.Errorf("Init() should return nil, got %T", cmd)
	}
}

// TestTUICtrlCQuits verifies that Ctrl+C also triggers the quit command.
func TestTUICtrlCQuits(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(TUIModel)
	if !m.quitting {
		t.Error("ctrl+c should set quitting=true")
	}
	if cmd == nil {
		t.Error("ctrl+c should return tea.Quit cmd")
	}
}

// TestTUIWindowSizeMsg verifies that window size updates are recorded.
func TestTUIWindowSizeMsg(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(TUIModel)
	if m.width != 120 || m.height != 40 {
		t.Errorf("expected size 120x40, got %dx%d", m.width, m.height)
	}
}

// TestTUIQuittingViewIsEmpty verifies View returns empty when quitting,
// so Bubble Tea can clean up the screen.
func TestTUIQuittingViewIsEmpty(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(TUIModel)
	if m.View() != "" {
		t.Errorf("View() should be empty when quitting, got: %q", m.View())
	}
}

// TestTUILogTrimsToOneHundred verifies that the log view caps at 100 entries
// to prevent unbounded growth.
func TestTUILogTrimsToOneHundred(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")
	for i := 0; i < 150; i++ {
		updated, _ := m.Update(LogMsg{Message: "msg"})
		m = updated.(TUIModel)
	}
	if len(m.logs) != 100 {
		t.Errorf("expected logs trimmed to 100, got %d", len(m.logs))
	}
}

// silence "unused" warning when context isn't needed.
var _ = context.Background
