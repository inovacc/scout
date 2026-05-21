//go:build integration
// +build integration

package aria_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inovacc/scout/internal/engine"
	"github.com/inovacc/scout/pkg/scout/aria"
)


func newTestBrowser(t *testing.T) *engine.Browser {
	t.Helper()
	br, err := engine.New(engine.WithHeadless(true))
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	t.Cleanup(func() { _ = br.Close() })
	return br
}

func TestCapture_Iframe_RealBrowser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/inner", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body><input aria-label="Inner Input"></body></html>`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><body>
		  <button>Outer Click</button>
		  <iframe src="/inner"></iframe>
		</body></html>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	br := newTestBrowser(t)
	page, err := br.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	_ = page.WaitLoad()

	snap, err := aria.Capture(context.Background(), page)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	var sawOuter, sawInner bool
	for _, n := range snap.Nodes {
		if n.Role == "button" && strings.Contains(n.Name, "Outer Click") {
			sawOuter = true
		}
		if n.Role == "textbox" && strings.Contains(n.Name, "Inner Input") {
			if !strings.HasPrefix(n.Ref, "f") {
				t.Errorf("inner ref %q lacks frame prefix", n.Ref)
			}
			sawInner = true
		}
	}
	if !sawOuter || !sawInner {
		t.Fatalf("missing expected nodes; got %d nodes", len(snap.Nodes))
	}
}

func TestCapture_SimpleForm_RealBrowser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<!doctype html><html><body>
  <h1>Simple Form</h1>
  <input type="text" aria-label="Email">
  <button>Submit</button>
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
	if snap.Version == 0 || snap.PageID == "" {
		t.Fatalf("snapshot missing identity: %+v", snap)
	}

	var sawButton, sawTextbox bool
	for _, n := range snap.Nodes {
		if n.Role == "button" && strings.Contains(n.Name, "Submit") {
			sawButton = true
		}
		if n.Role == "textbox" && strings.Contains(n.Name, "Email") {
			sawTextbox = true
		}
	}
	if !sawButton || !sawTextbox {
		t.Fatalf("missing expected nodes (sawButton=%v sawTextbox=%v); got %d nodes", sawButton, sawTextbox, len(snap.Nodes))
	}
}
