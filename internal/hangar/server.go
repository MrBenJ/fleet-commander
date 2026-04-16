package hangar

import (
	"bytes"
	"context"
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
	"github.com/MrBenJ/fleet-commander/internal/hangar/terminal"
	"github.com/MrBenJ/fleet-commander/internal/hangar/ws"
)

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
	port     int
	addr     string
	addrMu   sync.RWMutex
	devMode  bool
	webFS    fs.FS
	mux      *http.ServeMux
	logger   *log.Logger
	server   *http.Server
	fleetDir string
	api      *api.Handlers
	hub      *ws.Hub
	terminal *terminal.Proxy
	LogCh    chan string
}

type Config struct {
	Port             int
	DevMode          bool
	WebFS            fs.FS
	RepoPath         string // repo root — for fleet.Load()
	FleetDir         string // .fleet directory — for context/channels
	TmuxPrefix       string
	ControlSquadron  string // when set, open directly to mission control for this squadron
}

func NewServer(cfg Config) *Server {
	logCh := make(chan string, 100)
	cw := &chanWriter{ch: logCh}
	logger := log.New(cw, "[hangar] ", log.Ltime)
	s := &Server{
		port:     cfg.Port,
		devMode:  cfg.DevMode,
		webFS:    cfg.WebFS,
		fleetDir: cfg.FleetDir,
		mux:      http.NewServeMux(),
		logger:   logger,
		api:      api.NewHandlers(cfg.RepoPath, cfg.FleetDir),
		hub:      ws.NewHub(cfg.FleetDir, cfg.RepoPath, cfg.TmuxPrefix, logger),
		terminal: terminal.NewProxy(cfg.TmuxPrefix, logger),
		LogCh:    logCh,
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
	w.Write([]byte(`{"status":"ok"}`))
}

type spaHandler struct {
	fs http.Handler
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path != "/" && !strings.Contains(path, ".") {
		r.URL.Path = "/"
	}
	h.fs.ServeHTTP(w, r)
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port %d in use: %w (try --port)", s.port, err)
	}
	s.addrMu.Lock()
	s.addr = listener.Addr().String()
	s.addrMu.Unlock()

	s.server = &http.Server{Handler: s.mux}

	go func() {
		<-ctx.Done()
		s.server.Shutdown(context.Background())
	}()

	go s.hub.PollLoop(ctx)

	s.log(fmt.Sprintf("Server started on %s", s.addr))
	return s.server.Serve(listener)
}

func (s *Server) Addr() string {
	s.addrMu.RLock()
	defer s.addrMu.RUnlock()
	return s.addr
}

func (s *Server) Port() int {
	return s.port
}

func (s *Server) log(msg string) {
	s.logger.Print(msg)
}
