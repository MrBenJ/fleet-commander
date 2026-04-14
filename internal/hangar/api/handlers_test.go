package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGetPersonas(t *testing.T) {
	h := NewHandlers("/tmp/fake-repo", "/tmp/fake-repo/.fleet")

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/personas", nil)
	w := httptest.NewRecorder()

	h.HandleGetPersonas(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var personas []PersonaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &personas); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(personas) < 5 {
		t.Fatalf("expected at least 5 personas, got %d", len(personas))
	}

	if personas[0].Name == "" || personas[0].DisplayName == "" {
		t.Error("persona missing name or displayName")
	}
}

func TestHandleGetDrivers(t *testing.T) {
	h := NewHandlers("/tmp/fake-repo", "/tmp/fake-repo/.fleet")

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/drivers", nil)
	w := httptest.NewRecorder()

	h.HandleGetDrivers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var drivers []DriverResponse
	if err := json.Unmarshal(w.Body.Bytes(), &drivers); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(drivers) < 3 {
		t.Fatalf("expected at least 3 drivers, got %d", len(drivers))
	}
}

func TestHandleGetPersonasWrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake-repo", "/tmp/fake-repo/.fleet")

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/personas", nil)
	w := httptest.NewRecorder()

	h.HandleGetPersonas(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
