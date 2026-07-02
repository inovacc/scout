package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestPage_OutlivesOperationTimeout proves a page older than its per-operation
// timeout can still execute operations. Before the fix, NewPage stored a rod
// page carrying an ABSOLUTE deadline measured from creation, so every operation
// failed with context.DeadlineExceeded once the page outlived the timeout (the
// REPL became unusable ~30s after opening a tab). Now each self-contained op
// gets a fresh budget via Page.timed().
func TestPage_OutlivesOperationTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("browser test")
	}

	b, err := New(WithHeadless(true), WithNoSandbox(), WithTimeout(2*time.Second))
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html><body><h1 id=\"x\">hi</h1></body></html>"))
	}))
	defer srv.Close()

	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	if err := p.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	// Age the page well past the 2s per-operation timeout.
	time.Sleep(4 * time.Second)

	// Before the fix this failed with context.DeadlineExceeded.
	if _, err := p.Eval("() => 1 + 1"); err != nil {
		t.Fatalf("eval on aged page failed (page-lifetime deadline regression): %v", err)
	}

	// A second aged operation must also succeed (fresh budget each time).
	if _, err := p.HTML(); err != nil {
		t.Fatalf("HTML on aged page failed: %v", err)
	}
}
