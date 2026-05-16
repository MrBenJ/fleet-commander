package api

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"strings"
	"testing"
)

// failingResponseWriter simulates a broken HTTP connection — every Write
// returns an error. Used to verify writeJSON logs the failure instead of
// silently dropping it.
type failingResponseWriter struct {
	header http.Header
	status int
}

func newFailingResponseWriter() *failingResponseWriter {
	return &failingResponseWriter{header: http.Header{}}
}

func (f *failingResponseWriter) Header() http.Header        { return f.header }
func (f *failingResponseWriter) WriteHeader(status int)     { f.status = status }
func (f *failingResponseWriter) Write([]byte) (int, error)  { return 0, errors.New("broken pipe") }

// TestWriteJSON_LogsErrorOnWriteFailure verifies that writeJSON captures
// the encode/write error and forwards it to the logger, instead of
// silently dropping it (the C5 behavior).
func TestWriteJSON_LogsErrorOnWriteFailure(t *testing.T) {
	var buf bytes.Buffer
	originalOutput := log.Default().Writer()
	originalFlags := log.Default().Flags()
	log.Default().SetOutput(&buf)
	log.Default().SetFlags(0)
	defer func() {
		log.Default().SetOutput(originalOutput)
		log.Default().SetFlags(originalFlags)
	}()

	w := newFailingResponseWriter()
	writeJSON(w, http.StatusOK, map[string]string{"k": "v"})

	logOut := buf.String()
	if !strings.Contains(logOut, "writeJSON") || !strings.Contains(logOut, "broken pipe") {
		t.Errorf("expected log to mention writeJSON and the underlying error, got %q", logOut)
	}
}

// TestWriteError_LogsErrorOnWriteFailure mirrors the above for writeError.
func TestWriteError_LogsErrorOnWriteFailure(t *testing.T) {
	var buf bytes.Buffer
	originalOutput := log.Default().Writer()
	originalFlags := log.Default().Flags()
	log.Default().SetOutput(&buf)
	log.Default().SetFlags(0)
	defer func() {
		log.Default().SetOutput(originalOutput)
		log.Default().SetFlags(originalFlags)
	}()

	w := newFailingResponseWriter()
	writeError(w, http.StatusInternalServerError, "boom")

	logOut := buf.String()
	if !strings.Contains(logOut, "writeJSON") || !strings.Contains(logOut, "broken pipe") {
		t.Errorf("expected log to mention writeJSON and the underlying error, got %q", logOut)
	}
}

// TestWriteJSONWithLogger_RoutesToCustomLogger ensures the per-handler
// logger override actually receives the diagnostic.
func TestWriteJSONWithLogger_RoutesToCustomLogger(t *testing.T) {
	var buf bytes.Buffer
	custom := log.New(&buf, "[custom] ", 0)

	w := newFailingResponseWriter()
	writeJSONWithLogger(w, http.StatusOK, map[string]string{"k": "v"}, custom)

	if !strings.Contains(buf.String(), "[custom] ") {
		t.Errorf("expected [custom] prefix in log output, got %q", buf.String())
	}
}

// TestWriteJSONWithLogger_NilLoggerFallsBackToDefault verifies nil logger
// doesn't crash.
func TestWriteJSONWithLogger_NilLoggerFallsBackToDefault(t *testing.T) {
	var buf bytes.Buffer
	originalOutput := log.Default().Writer()
	log.Default().SetOutput(&buf)
	defer log.Default().SetOutput(originalOutput)

	w := newFailingResponseWriter()
	writeJSONWithLogger(w, http.StatusOK, map[string]string{"k": "v"}, nil)

	if !strings.Contains(buf.String(), "writeJSON") {
		t.Errorf("expected default logger to capture error, got %q", buf.String())
	}
}

// TestNewHandlersWithLogger_NilLoggerDoesNotCrash verifies the constructor
// tolerates a nil logger (and substitutes the default).
func TestNewHandlersWithLogger_NilLoggerDoesNotCrash(t *testing.T) {
	h := NewHandlersWithLogger("/tmp/x", "/tmp/x/.fleet", nil)
	if h.logger == nil {
		t.Error("expected nil logger to be replaced by default")
	}
}
