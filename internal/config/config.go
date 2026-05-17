// Package config provides environment-driven defaults for Fleet Commander
// runtime settings.
//
// Precedence (highest → lowest):
//  1. Explicit values supplied by callers (e.g. CLI flags)
//  2. Environment variables (FLEET_*)
//  3. Built-in defaults defined here
//
// Callers should obtain a [Config] via [Load], then override individual
// fields with flag values when the flag was actually set by the user. This
// keeps the env-vs-flag merge in one place rather than scattered through
// command bodies.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Default values used when neither a flag nor an env var supplies one.
const (
	DefaultPort      = 4242
	DefaultListen    = "127.0.0.1"
	DefaultLogLevel  = "info"
	DefaultLogFormat = "text"
)

// LogLevel values accepted by [Config.LogLevel].
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// LogFormat values accepted by [Config.LogFormat].
const (
	LogFormatText = "text"
	LogFormatJSON = "json"
)

// Env var names. Kept as constants so callers and docs reference one source.
const (
	EnvPort      = "FLEET_PORT"
	EnvListen    = "FLEET_LISTEN"
	EnvLogLevel  = "FLEET_LOG_LEVEL"
	EnvLogFormat = "FLEET_LOG_FORMAT"
)

// Config is the resolved runtime configuration.
type Config struct {
	// Port the hangar HTTP server listens on. Default: 4242.
	Port int

	// Listen address the hangar HTTP server binds to. Default: 127.0.0.1
	// (localhost only — opt in to LAN exposure explicitly).
	Listen string

	// LogLevel is one of debug, info, warn, error. Default: info.
	LogLevel string

	// LogFormat is one of text, json. Default: text.
	LogFormat string
}

// Load reads configuration from environment variables, falling back to the
// built-in defaults. It returns an error only when an env var is set to an
// invalid value — missing vars are not an error.
func Load() (Config, error) {
	return loadFrom(os.LookupEnv)
}

// loadFrom is the testable core: pluggable lookup function so unit tests
// don't have to mutate process env.
func loadFrom(lookup func(string) (string, bool)) (Config, error) {
	cfg := Defaults()

	if raw, ok := lookup(EnvPort); ok {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return cfg, fmt.Errorf("%s: %q is not a valid integer", EnvPort, raw)
		}
		if n < 1 || n > 65535 {
			return cfg, fmt.Errorf("%s: %d is out of range (1-65535)", EnvPort, n)
		}
		cfg.Port = n
	}

	if raw, ok := lookup(EnvListen); ok {
		v := strings.TrimSpace(raw)
		if v == "" {
			return cfg, fmt.Errorf("%s: must not be empty", EnvListen)
		}
		cfg.Listen = v
	}

	if raw, ok := lookup(EnvLogLevel); ok {
		v := strings.ToLower(strings.TrimSpace(raw))
		if !isValidLogLevel(v) {
			return cfg, fmt.Errorf("%s: %q is not one of debug|info|warn|error", EnvLogLevel, raw)
		}
		cfg.LogLevel = v
	}

	if raw, ok := lookup(EnvLogFormat); ok {
		v := strings.ToLower(strings.TrimSpace(raw))
		if !isValidLogFormat(v) {
			return cfg, fmt.Errorf("%s: %q is not one of text|json", EnvLogFormat, raw)
		}
		cfg.LogFormat = v
	}

	return cfg, nil
}

// Defaults returns the built-in defaults — useful for tests and for callers
// that want to construct a Config without touching the environment.
func Defaults() Config {
	return Config{
		Port:      DefaultPort,
		Listen:    DefaultListen,
		LogLevel:  DefaultLogLevel,
		LogFormat: DefaultLogFormat,
	}
}

// MergeFlags overlays caller-supplied flag values on top of the env-loaded
// config. A zero value means "flag was not set, keep current value":
//   - port == 0 → keep existing
//   - listen == "" → keep existing
//   - level == "" → keep existing
//   - format == "" → keep existing
//
// Returns an error if the supplied values are themselves invalid.
func (c Config) MergeFlags(port int, listen, level, format string) (Config, error) {
	out := c
	if port != 0 {
		if port < 1 || port > 65535 {
			return out, fmt.Errorf("port: %d is out of range (1-65535)", port)
		}
		out.Port = port
	}
	if listen != "" {
		out.Listen = listen
	}
	if level != "" {
		l := strings.ToLower(level)
		if !isValidLogLevel(l) {
			return out, fmt.Errorf("log level: %q is not one of debug|info|warn|error", level)
		}
		out.LogLevel = l
	}
	if format != "" {
		f := strings.ToLower(format)
		if !isValidLogFormat(f) {
			return out, fmt.Errorf("log format: %q is not one of text|json", format)
		}
		out.LogFormat = f
	}
	return out, nil
}

// Addr returns the host:port pair the hangar should listen on.
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Listen, c.Port)
}

// ErrInvalidConfig is returned when a Config fails its own validation.
var ErrInvalidConfig = errors.New("invalid fleet config")

// Validate enforces the same constraints as Load — useful when a Config is
// constructed by hand (e.g. in tests or by combining flags + env outside of
// MergeFlags).
func (c Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("%w: port %d out of range", ErrInvalidConfig, c.Port)
	}
	if strings.TrimSpace(c.Listen) == "" {
		return fmt.Errorf("%w: listen must not be empty", ErrInvalidConfig)
	}
	if !isValidLogLevel(c.LogLevel) {
		return fmt.Errorf("%w: log level %q", ErrInvalidConfig, c.LogLevel)
	}
	if !isValidLogFormat(c.LogFormat) {
		return fmt.Errorf("%w: log format %q", ErrInvalidConfig, c.LogFormat)
	}
	return nil
}

func isValidLogLevel(s string) bool {
	switch s {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		return true
	}
	return false
}

func isValidLogFormat(s string) bool {
	switch s {
	case LogFormatText, LogFormatJSON:
		return true
	}
	return false
}
