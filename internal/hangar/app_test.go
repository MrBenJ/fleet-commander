package hangar

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestWaitForServerReturnsServerError(t *testing.T) {
	errCh := make(chan error, 1)
	errCh <- errors.New("listen failed")

	err := waitForServer(errCh)
	if err == nil || !strings.Contains(err.Error(), "listen failed") {
		t.Fatalf("waitForServer() = %v, want listen failure", err)
	}
}

func TestWaitForServerTreatsHTTPServerClosedAsClean(t *testing.T) {
	errCh := make(chan error, 1)
	errCh <- http.ErrServerClosed

	if err := waitForServer(errCh); err != nil {
		t.Fatalf("waitForServer() = %v, want nil", err)
	}
}
