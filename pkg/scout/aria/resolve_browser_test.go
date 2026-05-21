//go:build integration
// +build integration

package aria_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func TestResolveElement_ClickRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body>
		  <button id="b" onclick="document.title='clicked'">Click Me</button>
		</body></html>`))
	}))
	t.Cleanup(srv.Close)

	br := newTestBrowser(t)
	page, err := br.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	snap, err := aria.Capture(context.Background(), page)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	var btnRef string
	for _, n := range snap.Nodes {
		if n.Role == "button" {
			btnRef = n.Ref
			break
		}
	}
	if btnRef == "" {
		t.Fatalf("button ref not found in %+v", snap.Nodes)
	}

	emptyStore := aria.NewStore()
	if _, err := emptyStore.Resolve(snap.PageID, btnRef); !errors.Is(err, aria.ErrNoSnapshot) {
		t.Fatalf("expected ErrNoSnapshot from empty store, got %v", err)
	}

	store := aria.NewStore()
	store.Put(snap.PageID, snap)
	bn, err := store.Resolve(snap.PageID, btnRef)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	el, err := aria.ResolveElement(page, bn)
	if err != nil {
		t.Fatalf("ResolveElement: %v", err)
	}
	if err := el.Click(); err != nil {
		t.Fatalf("Click: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title: %v", err)
	}
	if title != "clicked" {
		t.Errorf("title=%q, want %q", title, "clicked")
	}
}
