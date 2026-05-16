package hangar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/MrBenJ/fleet-commander/internal/hangar/api"
	"github.com/MrBenJ/fleet-commander/internal/hangar/security"
	"github.com/MrBenJ/fleet-commander/internal/hangar/terminal"
	"github.com/MrBenJ/fleet-commander/internal/hangar/ws"
)

// DefaultListenHost is the loopback bind address used when Config.Listen is
// empty. The hangar exposes a PTY proxy and squadron-control API: binding to
// localhost by default prevents anyone else on the LAN from driving it.
const DefaultListenHost = "127.0.0.1"

// chanWriter routes log output to a channel instead of stderr,
// so log messages display cleanly in the Bubble Tea TUI.
type chanWriter struct {
	ch  chan string
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *chanWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// incomplete line — put it back
			w.buf.WriteString(line)
			break
		}
		msg := strings.TrimRight(line, "\n")
		if msg == "" {
			continue
		}
		select {
		case w.ch <- msg:
		default:
		}
	}
	return len(p), nil
}

type Server struct {
	port       int
	listenHost string
	addr       string
	addrMu     sync.RWMutex
	devMode    bool
	webFS      fs.FS
	mux        *http.ServeMux
	logger     *log.Logger
	server     *http.Server
	fleetDir   string
	api        *api.Handlers
	hub        *ws.Hub
	terminal   *terminal.Proxy
	validator  *security.Validator
	logChOnce  sync.Once
	LogCh      chan string
}

type Config struct {
	Port            int
	Listen          string // host to bind on; defaults to 127.0.0.1
	DevMode         bool
	WebFS           fs.FS
	RepoPath        string // repo root — for fleet.Load()
	FleetDir        string // .fleet directory — for context/channels
	TmuxPrefix      string
	ControlSquadron string // when set, open directly to mission control for this squadron
}

func NewServer(cfg Config) *Server {
	logCh := make(chan string, 100)
	cw := &chanWriter{ch: logCh}
	logger := log.New(cw, "[hangar] ", log.Ltime)
	listenHost := cfg.Listen
	if listenHost == "" {
		listenHost = DefaultListenHost
	}
	validator := security.New(cfg.DevMode)
	s := &Server{
		port:       cfg.Port,
		listenHost: listenHost,
		devMode:    cfg.DevMode,
		webFS:      cfg.WebFS,
		fleetDir:   cfg.FleetDir,
		mux:        http.NewServeMux(),
		logger:     logger,
		api:        api.NewHandlersWithLogger(cfg.RepoPath, cfg.FleetDir, logger),
		hub:        ws.NewHubWithValidator(cfg.FleetDir, cfg.RepoPath, cfg.TmuxPrefix, logger, validator),
		terminal:   terminal.NewProxyWithValidator(cfg.TmuxPrefix, logger, validator),
		validator:  validator,
		LogCh:      logCh,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.Handle("/ws/terminal/", http.HandlerFunc(s.terminal.HandleTerminal))
	s.mux.HandleFunc("/ws/events", s.hub.HandleWebSocket)
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/fleet", s.api.HandleGetFleet)
	s.mux.HandleFunc("/api/fleet/personas", s.api.HandleGetPersonas)
	s.mux.HandleFunc("/api/fleet/drivers", s.api.HandleGetDrivers)
	s.mux.HandleFunc("/api/drivers/available", s.api.HandleAvailableDrivers)
	s.mux.HandleFunc("/api/fleet/branches", s.api.HandleGetBranches)
	s.mux.HandleFunc("/api/squadron/launch", s.api.HandleLaunchSquadron)
	s.mux.HandleFunc("/api/squadron/generate", s.api.HandleGenerate)
	s.mux.HandleFunc("/api/squadron/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			s.api.HandleSquadronStatus(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/info") {
			s.api.HandleSquadronInfo(w, r)
			return
		}
		http.NotFound(w, r)
	})
	s.mux.HandleFunc("/api/agent/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/stop") {
			s.api.HandleStopAgent(w, r)
			return
		}
		http.NotFound(w, r)
	})

	if s.devMode {
		target, _ := url.Parse("http://localhost:5173")
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = &http.Transport{
			DisableKeepAlives: true,
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			s.logger.Printf("dev proxy error (%s %s): %v", r.Method, r.URL.Path, err)
			http.Error(w, "Vite dev server unavailable", http.StatusBadGateway)
		}
		s.mux.Handle("/", proxy)
	} else if s.webFS != nil {
		spa := &spaHandler{fs: http.FileServerFS(s.webFS)}
		s.mux.Handle("/", spa)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		s.logger.Printf("handleHealth write: %v", err)
	}
}

type spaHandler struct {
	fs http.Handler
}

// ServeHTTP routes requests to the SPA. A path is treated as a real asset
// only if its *last* segment contains a dot ("/assets/main.js" → asset,
// "/foo.bar/baz" → SPA route). The previous heuristic checked the whole
// path with strings.Contains and misrouted any path with a dotted directory.
func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !isAssetPath(r.URL.Path) {
		r.URL.Path = "/"
	}
	h.fs.ServeHTTP(w, r)
}

// isAssetPath returns true if the path looks like a static asset and should
// be served directly rather than rewritten to index.html.
func isAssetPath(p string) bool {
	if p == "" || p == "/" {
		return false
	}
	idx := strings.LastIndex(p, "/")
	last := p[idx+1:]
	return strings.Contains(last, ".")
}

// csrfMiddleware enforces an Origin allowlist on non-GET/HEAD/OPTIONS
// requests. Defense-in-depth against drive-by browser POSTs, which would
// otherwise be able to launch or stop squadrons.
func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if !s.validator.AllowCrossSiteRequest(r) {
			http.Error(w, "forbidden: cross-origin request denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Start binds the listener, registers shutdown on ctx cancellation, and
// blocks serving requests. Returns nil on graceful shutdown, or an error
// from net.Listen / server.Serve otherwise.
func (s *Server) Start(ctx context.Context) error {
	addr := net.JoinHostPort(s.listenHost, fmt.Sprintf("%d", s.port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w (try --port)", addr, err)
	}
	s.addrMu.Lock()
	s.addr = listener.Addr().String()
	s.addrMu.Unlock()

	s.server = &http.Server{Handler: s.csrfMiddleware(s.mux)}

	// serveCtx is canceled either when the caller cancels ctx OR when Serve
	// returns of its own accord (listener closed, fatal error). Either path
	// must unblock the shutdown goroutine so Start always returns instead
	// of hanging on <-shutdownDone.
	serveCtx, serveCancel := context.WithCancel(ctx)
	defer serveCancel()

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-serveCtx.Done()
		shutdownCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			s.logger.Printf("server shutdown: %v", err)
		}
	}()

	go s.hub.PollLoop(ctx)

	s.log(fmt.Sprintf("Server started on %s", s.addr))
	err = s.server.Serve(listener)
	// Signal the shutdown goroutine so it proceeds even when Serve returned
	// before ctx was canceled (e.g., a listener-level failure).
	serveCancel()
	<-shutdownDone
	s.closeLogCh()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// closeLogCh closes the log channel exactly once so consumers can range
// over it cleanly. Safe to call multiple times.
func (s *Server) closeLogCh() {
	s.logChOnce.Do(func() {
		close(s.LogCh)
	})
}

func (s *Server) Addr() string {
	s.addrMu.RLock()
	defer s.addrMu.RUnlock()
	return s.addr
}

func (s *Server) Port() int {
	return s.port
}

// ListenHost returns the host portion of the bind address (e.g. "127.0.0.1").
func (s *Server) ListenHost() string {
	return s.listenHost
}

func (s *Server) log(msg string) {
	s.logger.Print(msg)
}
