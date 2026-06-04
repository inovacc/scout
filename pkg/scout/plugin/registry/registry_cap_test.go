package registry

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFetchIndexBodyCap proves FetchIndex rejects an oversized registry index
// instead of reading it into memory unbounded — the representative regression
// for the HARDENING-V2 Wave 6 io.LimitReader byte-caps (the same +1-sentinel
// pattern is applied to every remote-response read across the codebase).
func TestFetchIndexBodyCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"version":"1","plugins":[`)
		// Stream just past the 8 MiB cap without buffering it all server-side.
		chunk := strings.Repeat(`{"name":"x","description":"y"},`, 1024)
		for written := 0; written <= 8<<20; written += len(chunk) {
			_, _ = io.WriteString(w, chunk)
		}
	}))
	defer srv.Close()

	if _, err := FetchIndex(srv.URL); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("FetchIndex(oversized) = %v, want an 'exceeds' cap error", err)
	}
}
