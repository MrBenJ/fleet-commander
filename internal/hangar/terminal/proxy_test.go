package terminal

import (
	"log"
	"testing"
)

func TestParseResize(t *testing.T) {
	tests := []struct {
		input    string
		wantNil  bool
		wantCols uint16
		wantRows uint16
	}{
		{`{"cols": 80, "rows": 24}`, false, 80, 24},
		{`{"cols": 120, "rows": 40}`, false, 120, 40},
		{`{"cols": 0, "rows": 0}`, true, 0, 0},
		{`not json`, true, 0, 0},
		{`{"type": "data"}`, true, 0, 0},
	}

	for _, tt := range tests {
		ws := parseResize([]byte(tt.input))
		if tt.wantNil {
			if ws != nil {
				t.Errorf("parseResize(%q) = %+v, want nil", tt.input, ws)
			}
			continue
		}
		if ws == nil {
			t.Errorf("parseResize(%q) = nil, want cols=%d rows=%d", tt.input, tt.wantCols, tt.wantRows)
			continue
		}
		if ws.Cols != tt.wantCols || ws.Rows != tt.wantRows {
			t.Errorf("parseResize(%q) = cols=%d rows=%d, want cols=%d rows=%d",
				tt.input, ws.Cols, ws.Rows, tt.wantCols, tt.wantRows)
		}
	}
}

func TestNewProxy(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	p := NewProxy("fleet", logger)

	if p.tmuxPrefix != "fleet" {
		t.Errorf("expected prefix 'fleet', got %q", p.tmuxPrefix)
	}
}
