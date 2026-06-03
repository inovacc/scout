//go:build integration

package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

func newCaptureBrowser(t *testing.T) *scout.Browser {
	t.Helper()
	b, err := scout.New()
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func TestCaptureFlowRecordsAPICall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><script>fetch('/api/data')</script></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := newCaptureBrowser(t)
	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = page.Close() }()

	capt, err := CaptureFlow(context.Background(), page, CaptureOptions{
		URL: srv.URL, Name: "t", URLFilter: []string{"*api*"},
	})
	if err != nil {
		t.Fatalf("CaptureFlow: %v", err)
	}
	found := false
	for _, e := range capt.Entries {
		if e.URL == srv.URL+"/api/data" && e.RespBody != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("api call not captured: %+v", capt.Entries)
	}
}
