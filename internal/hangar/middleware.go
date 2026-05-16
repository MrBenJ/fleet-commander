package hangar

import (
	"bufio"
	"errors"
	"log"
	"net"
	"net/http"
	"time"
)

var errHijackUnsupported = errors.New("hangar: underlying ResponseWriter does not support hijacking")

// statusRecorder wraps http.ResponseWriter so middleware can observe the
// final status code and response size. Forwards Flush and Hijack to the
// underlying writer so SSE and WebSocket upgrades still work.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		// Implicit 200 once any bytes are written without a prior WriteHeader.
		r.wroteHeader = true
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack must be present so the wrapped ResponseWriter still satisfies
// http.Hijacker — gorilla/websocket needs this for the WS upgrade.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errHijackUnsupported
	}
	// Once hijacked, the request leaves the http server's bookkeeping.
	// We treat that as a "switching protocols" event in the log.
	conn, rw, err := h.Hijack()
	if err == nil {
		r.wroteHeader = true
		r.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

// accessLogger is HTTP middleware that records method, path, status and
// duration for every request. It uses the same *log.Logger the rest of the
// server uses; once the codebase migrates to slog, the call site swaps the
// logger in and this middleware's shape stays the same.
func accessLogger(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		logger.Printf("%s %s -> %d (%s, %d bytes)",
			r.Method,
			r.URL.Path,
			rec.status,
			time.Since(start).Round(time.Millisecond),
			rec.bytes,
		)
	})
}
