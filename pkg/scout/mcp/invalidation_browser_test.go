//go:build integration

package mcp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout/aria"
	"github.com/inovacc/scout/pkg/scout/mcp"
)

func TestInvalidation_NavigationClearsStore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body><button>A</button></body></html>`))
	}))
	t.Cleanup(srv.Close)

	state, cleanup := mcp.NewStateForTest(t)
	t.Cleanup(cleanup)

	page, err := state.OpenPage(srv.URL)
	if err != nil {
		t.Fatalf("OpenPage: %v", err)
	}

	snap, err := aria.Capture(context.Background(), page)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	state.AriaStore().Put(snap.PageID, snap)

	// Install hooks exactly as tools_aria.go does after Put.
	state.InstallInvalidationHooks(page)

	// Navigate to a different URL on the same server — fires root frameNavigated.
	if err := page.Navigate(srv.URL + "?other"); err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	_ = page.WaitLoad()

	// The frameNavigated listener goroutine clears the store. Poll up to 2s.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := state.AriaStore().Get(snap.PageID); !ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	_, err = state.AriaStore().Resolve(snap.PageID, "e0")
	if !errors.Is(err, aria.ErrNoSnapshot) {
		t.Errorf("expected ErrNoSnapshot after navigation, got %v", err)
	}
}
