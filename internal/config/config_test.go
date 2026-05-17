package config

import (
	"errors"
	"strings"
	"testing"
)

// mapLookup builds a lookup function from a static map. Allows tests to
// exercise loadFrom without touching process env.
func mapLookup(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestDefaults(t *testing.T) {
	got := Defaults()
	if got.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", got.Port, DefaultPort)
	}
	if got.Listen != DefaultListen {
		t.Errorf("Listen = %q, want %q", got.Listen, DefaultListen)
	}
	if got.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", got.LogLevel, DefaultLogLevel)
	}
	if got.LogFormat != DefaultLogFormat {
		t.Errorf("LogFormat = %q, want %q", got.LogFormat, DefaultLogFormat)
	}
}

func TestLoadFrom_NoEnvUsesDefaults(t *testing.T) {
	cfg, err := loadFrom(mapLookup(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != Defaults() {
		t.Errorf("got %+v, want defaults %+v", cfg, Defaults())
	}
}

func TestLoadFrom_OverridesAllFields(t *testing.T) {
	cfg, err := loadFrom(mapLookup(map[string]string{
		EnvPort:      "8080",
		EnvListen:    "0.0.0.0",
		EnvLogLevel:  "debug",
		EnvLogFormat: "json",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Config{Port: 8080, Listen: "0.0.0.0", LogLevel: "debug", LogFormat: "json"}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadFrom_TrimsWhitespace(t *testing.T) {
	cfg, err := loadFrom(mapLookup(map[string]string{
		EnvPort:     "  4000  ",
		EnvListen:   "  ::1  ",
		EnvLogLevel: "  WARN  ",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 4000 {
		t.Errorf("Port = %d, want 4000", cfg.Port)
	}
	if cfg.Listen != "::1" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "::1")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn (lowercased)", cfg.LogLevel)
	}
}

func TestLoadFrom_PortValidation(t *testing.T) {
	cases := map[string]string{
		"non-numeric":  "not-a-number",
		"zero":         "0",
		"negative":     "-1",
		"out of range": "70000",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadFrom(mapLookup(map[string]string{EnvPort: raw}))
			if err == nil {
				t.Fatalf("expected error for %q", raw)
			}
			if !strings.Contains(err.Error(), EnvPort) {
				t.Errorf("error should mention %s, got %q", EnvPort, err)
			}
		})
	}
}

func TestLoadFrom_EmptyListenRejected(t *testing.T) {
	_, err := loadFrom(mapLookup(map[string]string{EnvListen: "   "}))
	if err == nil {
		t.Fatal("expected error for empty listen value")
	}
}

func TestLoadFrom_LogLevelValidation(t *testing.T) {
	_, err := loadFrom(mapLookup(map[string]string{EnvLogLevel: "verbose"}))
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestLoadFrom_LogFormatValidation(t *testing.T) {
	_, err := loadFrom(mapLookup(map[string]string{EnvLogFormat: "xml"}))
	if err == nil {
		t.Fatal("expected error for invalid log format")
	}
}

func TestMergeFlags_ZeroValuesArePassthrough(t *testing.T) {
	base := Config{Port: 4242, Listen: "127.0.0.1", LogLevel: "info", LogFormat: "text"}
	got, err := base.MergeFlags(0, "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != base {
		t.Errorf("zero flags should pass through; got %+v", got)
	}
}

func TestMergeFlags_OverridesEnvLoadedValues(t *testing.T) {
	base := Config{Port: 4242, Listen: "127.0.0.1", LogLevel: "info", LogFormat: "text"}
	got, err := base.MergeFlags(9000, "0.0.0.0", "debug", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Config{Port: 9000, Listen: "0.0.0.0", LogLevel: "debug", LogFormat: "json"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestMergeFlags_PortOutOfRange(t *testing.T) {
	base := Defaults()
	_, err := base.MergeFlags(70000, "", "", "")
	if err == nil {
		t.Fatal("expected error for out-of-range port")
	}
}

func TestMergeFlags_InvalidLogLevel(t *testing.T) {
	base := Defaults()
	_, err := base.MergeFlags(0, "", "loud", "")
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestAddr(t *testing.T) {
	cfg := Config{Listen: "127.0.0.1", Port: 4242}
	if got := cfg.Addr(); got != "127.0.0.1:4242" {
		t.Errorf("Addr = %q, want 127.0.0.1:4242", got)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"defaults are valid", Defaults(), false},
		{"port zero", Config{Port: 0, Listen: "127.0.0.1", LogLevel: "info", LogFormat: "text"}, true},
		{"empty listen", Config{Port: 1, Listen: "  ", LogLevel: "info", LogFormat: "text"}, true},
		{"bad level", Config{Port: 1, Listen: "x", LogLevel: "loud", LogFormat: "text"}, true},
		{"bad format", Config{Port: 1, Listen: "x", LogLevel: "info", LogFormat: "yaml"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr && err != nil && !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("error should wrap ErrInvalidConfig, got %v", err)
			}
		})
	}
}

// TestLoad_UsesProcessEnv exercises the real Load() against a value set in
// os.Setenv to confirm the wiring works end-to-end.
func TestLoad_UsesProcessEnv(t *testing.T) {
	t.Setenv(EnvPort, "5555")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 5555 {
		t.Errorf("Port = %d, want 5555", cfg.Port)
	}
}
