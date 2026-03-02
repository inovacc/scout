package scout

import (
	"fmt"
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/wait-react", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>React Wait</title></head>
<body>
<div id="root" data-reactroot></div>
<script>
window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = {renderers: new Map([[1, {}]])};
window.React = {version: '18.2.0'};
var root = document.getElementById('root');
root._reactRootContainer = {};
// Simulate async React render
setTimeout(function() {
	root.innerHTML = '<h1>Hello React</h1>';
}, 100);
</script>
</body></html>`)
		})

		mux.HandleFunc("/wait-angular", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Angular Wait</title></head>
<body>
<app-root ng-version="17.3.0">
<h1>Angular App</h1>
</app-root>
<script>
window.getAllAngularTestabilities = function() {
	return [{
		whenStable: function(cb) { setTimeout(cb, 50); }
	}];
};
</script>
</body></html>`)
		})

		mux.HandleFunc("/wait-plain", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Plain Page</title></head>
<body><h1>Hello World</h1><p>No framework here.</p></body></html>`)
		})
	})
}

func TestWaitFrameworkReady_React(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/wait-react")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	if err := page.WaitFrameworkReady(); err != nil {
		t.Fatalf("WaitFrameworkReady: %v", err)
	}

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text: %v", err)
	}

	if text != "Hello React" {
		t.Errorf("expected 'Hello React', got %q", text)
	}
}

func TestWaitFrameworkReady_Angular(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/wait-angular")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	if err := page.WaitFrameworkReady(); err != nil {
		t.Fatalf("WaitFrameworkReady: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title: %v", err)
	}

	if title != "Angular Wait" {
		t.Errorf("expected 'Angular Wait', got %q", title)
	}
}

func TestWaitFrameworkReady_Fallback(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/wait-plain")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	// Should fall back to WaitLoad + WaitDOMStable without error
	if err := page.WaitFrameworkReady(); err != nil {
		t.Fatalf("WaitFrameworkReady on plain page: %v", err)
	}

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text: %v", err)
	}

	if text != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", text)
	}
}

func TestWaitFrameworkReady_NilPage(t *testing.T) {
	var p *Page

	err := p.WaitFrameworkReady()
	if err == nil {
		t.Fatal("expected error for nil page")
	}

	if err.Error() != "scout: wait framework ready: nil page" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWithSmartWait(t *testing.T) {
	opts := defaults()
	if opts.smartWait {
		t.Fatal("smartWait should be false by default")
	}

	WithSmartWait()(opts)

	if !opts.smartWait {
		t.Fatal("smartWait should be true after WithSmartWait()")
	}
}

func TestWithSmartWait_NewPage(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b, err := New(WithHeadless(true), WithNoSandbox(), WithTimeout(30e9), WithSmartWait())
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}

	defer func() { _ = b.Close() }()

	// SmartWait should auto-run WaitFrameworkReady on NewPage
	page, err := b.NewPage(srv.URL + "/wait-plain")
	if err != nil {
		t.Fatalf("NewPage with SmartWait: %v", err)
	}

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text: %v", err)
	}

	if text != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", text)
	}
}
