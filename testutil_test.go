package scout

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testRouteRegistrars collects route setup functions from other test files.
var testRouteRegistrars []func(*http.ServeMux)

// registerTestRoutes adds a route setup function to be called by newTestServer.
func registerTestRoutes(fn func(*http.ServeMux)) {
	testRouteRegistrars = append(testRouteRegistrars, fn)
}

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Test Page</title></head>
<body>
<h1>Hello World</h1>
<p id="info">Some text</p>
<input id="name" type="text" value="default"/>
<button id="btn" onclick="document.getElementById('info').textContent='Clicked'">Click Me</button>
<a id="link" href="/page2">Go to page 2</a>
<select id="sel">
  <option value="a">Alpha</option>
  <option value="b">Beta</option>
  <option value="c">Gamma</option>
</select>
<div id="parent"><span id="child">Child Text</span></div>
<div id="hidden" style="display:none">Hidden</div>
</body></html>`)
	})

	mux.HandleFunc("/page2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Page Two</title></head>
<body><h1>Page 2</h1><a href="/">Back</a></body></html>`)
	})

	mux.HandleFunc("/json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"name":"test","value":42}`)
	})

	mux.HandleFunc("/echo-headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><body><pre id="ua">%s</pre><pre id="custom">%s</pre></body></html>`,
			r.UserAgent(), r.Header.Get("X-Custom"))
	})

	mux.HandleFunc("/set-cookie", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "test", Value: "hello", Path: "/"})
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>Cookie set</body></html>`)
	})

	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/page2", http.StatusFound)
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Slow Page</title></head>
<body>
<script>
setTimeout(() => {
  const el = document.createElement('div');
  el.id = 'delayed';
  el.textContent = 'Loaded';
  document.body.appendChild(el);
}, 200);
</script>
</body></html>`)
	})

	// Register routes from other test files
	for _, fn := range testRouteRegistrars {
		fn(mux)
	}

	return httptest.NewServer(mux)
}

func newTestBrowser(t *testing.T) *Browser {
	t.Helper()

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30e9), // 30s
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}
