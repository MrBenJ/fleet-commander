// Package security provides shared HTTP/WebSocket origin validation for the
// hangar server. Browsers always send an Origin header for cross-site requests
// (and for every WebSocket handshake); validating it against the request Host
// blocks drive-by attacks from foreign tabs and DNS-rebind attempts.
package security

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Validator decides whether an incoming request's Origin is allowed.
//
// Two modes:
//   - production (devMode=false): allow requests whose Origin host equals the
//     request Host, including loopback aliases (localhost vs 127.0.0.1).
//   - dev (devMode=true): additionally allow Vite dev server origins
//     (port 5173 on loopback hosts).
//
// Empty Origin is treated as "non-browser client" and allowed, because
// browsers always send Origin on the affected paths. CLI tools and tests
// connect without an Origin header.
type Validator struct {
	devMode bool
}

// New constructs a Validator. devMode=true also permits the Vite dev server.
func New(devMode bool) *Validator {
	return &Validator{devMode: devMode}
}

// AllowWebSocket returns true if the WebSocket upgrade request's Origin
// header is allowed. The Host header on the request identifies the server's
// own host:port; Origin must match it (or be an accepted dev origin).
func (v *Validator) AllowWebSocket(r *http.Request) bool {
	if r == nil {
		return false
	}
	return v.allow(r.Header.Get("Origin"), r.Host)
}

// AllowCrossSiteRequest returns true if a REST request's Origin is allowed.
// Used as CSRF defense for non-GET handlers.
func (v *Validator) AllowCrossSiteRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	return v.allow(r.Header.Get("Origin"), r.Host)
}

func (v *Validator) allow(origin, host string) bool {
	if origin == "" {
		// Browsers always send Origin on the protected paths; an absent
		// Origin therefore can't be a browser-driven cross-origin attack.
		// CLI tools and tests need this branch.
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if hostsEqual(u.Host, host) {
		return true
	}
	if v.devMode && isViteDevHost(u.Host) {
		return true
	}
	return false
}

// hostsEqual compares two host:port values, normalizing loopback aliases so
// that "localhost:4242" and "127.0.0.1:4242" are treated as the same origin.
func hostsEqual(a, b string) bool {
	if a == b {
		return true
	}
	ah, ap := splitHostPort(a)
	bh, bp := splitHostPort(b)
	if ap != bp {
		return false
	}
	return isLoopback(ah) && isLoopback(bh)
}

func splitHostPort(s string) (host, port string) {
	h, p, err := net.SplitHostPort(s)
	if err != nil {
		return s, ""
	}
	return h, p
}

func isLoopback(h string) bool {
	if h == "" {
		return false
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	// strip brackets if any
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		h = h[1 : len(h)-1]
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func isViteDevHost(h string) bool {
	host, port := splitHostPort(h)
	if port != "5173" {
		return false
	}
	return isLoopback(host)
}
