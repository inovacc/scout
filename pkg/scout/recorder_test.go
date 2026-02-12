package scout

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/recorder-page", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Recorder Test</title></head>
<body>
<h1>Recorder</h1>
<img id="img" src="/recorder-asset" />
<script>
fetch('/recorder-api').then(r => r.json());
</script>
</body></html>`)
		})

		mux.HandleFunc("/recorder-asset", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			// 1x1 transparent PNG
			_, _ = w.Write([]byte{
				0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
				0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
			})
		})

		mux.HandleFunc("/recorder-api", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		})
	})
}

func TestNetworkRecorderCapturesEntries(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	rec := NewNetworkRecorder(page)
	defer rec.Stop()

	if err := page.Navigate(srv.URL + "/recorder-page"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Give events time to propagate
	time.Sleep(500 * time.Millisecond)

	entries := rec.Entries()
	if len(entries) == 0 {
		t.Fatal("expected at least one recorded entry")
	}

	// Should have captured the page load at minimum
	found := false
	for _, e := range entries {
		if e.Request.URL != "" {
			found = true
			break
		}
	}

	if !found {
		t.Error("no entry with a non-empty request URL")
	}
}

func TestNetworkRecorderExportHAR(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	rec := NewNetworkRecorder(page, WithCreatorName("test-tool", "1.0.0"))
	defer rec.Stop()

	if err := page.Navigate(srv.URL + "/recorder-page"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	data, count, err := rec.ExportHAR()
	if err != nil {
		t.Fatalf("ExportHAR() error: %v", err)
	}

	if count == 0 {
		t.Fatal("ExportHAR() returned 0 entries")
	}

	// Validate JSON structure
	var har struct {
		Log struct {
			Version string `json:"version"`
			Creator struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"creator"`
			Entries []json.RawMessage `json:"entries"`
		} `json:"log"`
	}

	if err := json.Unmarshal(data, &har); err != nil {
		t.Fatalf("ExportHAR() produced invalid JSON: %v", err)
	}

	if har.Log.Version != "1.2" {
		t.Errorf("HAR version = %q, want %q", har.Log.Version, "1.2")
	}

	if har.Log.Creator.Name != "test-tool" {
		t.Errorf("HAR creator name = %q, want %q", har.Log.Creator.Name, "test-tool")
	}

	if len(har.Log.Entries) != count {
		t.Errorf("HAR entries count = %d, ExportHAR count = %d", len(har.Log.Entries), count)
	}
}

func TestNetworkRecorderCaptureBody(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	rec := NewNetworkRecorder(page, WithCaptureBody(true))
	defer rec.Stop()

	if err := page.Navigate(srv.URL + "/json"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	entries := rec.Entries()
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	found := false
	for _, e := range entries {
		if e.Response.Content.Text != "" {
			found = true
			break
		}
	}

	if !found {
		t.Error("WithCaptureBody(true) should capture response body text")
	}
}

func TestNetworkRecorderStopIdempotent(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	rec := NewNetworkRecorder(page)

	// Stop multiple times should not panic
	rec.Stop()
	rec.Stop()
	rec.Stop()
}

func TestNetworkRecorderClear(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	rec := NewNetworkRecorder(page)
	defer rec.Stop()

	if err := page.Navigate(srv.URL + "/recorder-page"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	if len(rec.Entries()) == 0 {
		t.Fatal("expected entries before clear")
	}

	rec.Clear()

	if len(rec.Entries()) != 0 {
		t.Error("Clear() should reset entries to empty")
	}
}

func TestNetworkRecorderNilSafety(t *testing.T) {
	var rec *NetworkRecorder

	// All nil-safe methods should not panic
	rec.Stop()
	rec.Clear()

	entries := rec.Entries()
	if entries != nil {
		t.Error("nil recorder Entries() should return nil")
	}

	data, count, err := rec.ExportHAR()
	if err == nil {
		t.Error("nil recorder ExportHAR() should return error")
	}

	if data != nil || count != 0 {
		t.Error("nil recorder ExportHAR() should return nil data and 0 count")
	}
}

func TestNewNetworkRecorderNilPage(t *testing.T) {
	rec := NewNetworkRecorder(nil)
	if rec != nil {
		t.Error("NewNetworkRecorder(nil) should return nil")
	}
}
