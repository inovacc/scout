package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout/urlpolicy"
)

// TestProxyBlocksInternalTarget verifies the proxy refuses to navigate to an
// internal/loopback target (SSRF) before any browser work happens.
func TestProxyBlocksInternalTarget(t *testing.T) {
	s := &Server{policy: &urlpolicy.Policy{}} // empty policy => default-deny internal

	for _, target := range []string{
		"http://127.0.0.1:9000/admin",
		"http://169.254.169.254/latest/meta-data/",
		"http://localhost/secret",
	} {
		h := s.handleRoute(Route{Path: "/x", Target: target})
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodGet, "/x", nil))

		if w.Code != http.StatusForbidden {
			t.Errorf("target %q: got status %d, want 403", target, w.Code)
		}
	}
}

// TestProxyAllowsLocalWhenOptedIn verifies the operator opt-in lifts the SSRF
// guard at the policy layer the proxy enforces.
func TestProxyAllowsLocalWhenOptedIn(t *testing.T) {
	p := &urlpolicy.Policy{AllowLocal: true}
	if err := p.Check(context.Background(), "http://127.0.0.1:9000/ok"); err != nil {
		t.Fatalf("opted-in local target blocked: %v", err)
	}
}
