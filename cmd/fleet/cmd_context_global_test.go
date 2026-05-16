package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
)

func TestWriteTextExportIncludesSortedAgentsAndChannels(t *testing.T) {
	ctx := &fleetctx.Context{
		Shared: "shared notes",
		Agents: map[string]string{
			"bravo": "second",
			"alpha": "first",
		},
		Channels: map[string]*fleetctx.Channel{
			"squadron-demo": {
				Name:        "squadron-demo",
				Description: "demo channel",
				Members:     []string{"alpha", "bravo"},
				Log: []fleetctx.LogEntry{{
					Timestamp: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
					Agent:     "alpha",
					Message:   "COMPLETED",
				}},
			},
		},
	}

	f := tempOutputFile(t)
	if err := writeTextExport(f, "/repo", ctx); err != nil {
		t.Fatalf("writeTextExport() error: %v", err)
	}
	out := readTempOutput(t, f)

	alpha := strings.Index(out, "== alpha ==")
	bravo := strings.Index(out, "== bravo ==")
	if alpha == -1 || bravo == -1 || alpha > bravo {
		t.Fatalf("agent sections not sorted:\n%s", out)
	}
	if !strings.Contains(out, "== Channel: squadron-demo ==") {
		t.Fatalf("missing channel section:\n%s", out)
	}
}

func TestWriteLogOnlyExportJSON(t *testing.T) {
	ctx := &fleetctx.Context{
		Log: []fleetctx.LogEntry{{
			Timestamp: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
			Agent:     "alpha",
			Message:   "hello",
		}},
	}

	f := tempOutputFile(t)
	if err := writeLogOnlyExport(f, "/repo", ctx, "json"); err != nil {
		t.Fatalf("writeLogOnlyExport() error: %v", err)
	}

	var got struct {
		FleetPath string              `json:"fleet_path"`
		Log       []fleetctx.LogEntry `json:"log"`
	}
	if err := json.Unmarshal([]byte(readTempOutput(t, f)), &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if got.FleetPath != "/repo" || len(got.Log) != 1 || got.Log[0].Message != "hello" {
		t.Fatalf("unexpected export: %#v", got)
	}
}

func tempOutputFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "export-*")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func readTempOutput(t *testing.T, f *os.File) string {
	t.Helper()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
