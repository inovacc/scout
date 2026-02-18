package scout

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/map-start", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Map Start</title></head>
<body>
<a href="/map-page1">Page 1</a>
<a href="/map-page2">Page 2</a>
<a href="/map-page3">Page 3</a>
<a href="https://external.com/other">External</a>
</body></html>`)
		})

		mux.HandleFunc("/map-page1", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Page 1</title></head>
<body>
<a href="/map-page1-sub">Sub Page</a>
<a href="/map-start">Back</a>
</body></html>`)
		})

		mux.HandleFunc("/map-page1-sub", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body><a href="/map-page1">Parent</a></body></html>`)
		})

		mux.HandleFunc("/map-page2", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body><p>Page 2 content</p></body></html>`)
		})

		mux.HandleFunc("/map-page3", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body><a href="/map-page2">To Page 2</a></body></html>`)
		})
	})
}

func TestMapBasic(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	urls, err := b.Map(srv.URL+"/map-start",
		WithMapSitemap(false),
		WithMapDelay(0),
		WithMapMaxDepth(1),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(urls) < 4 { // start + page1 + page2 + page3
		t.Errorf("expected at least 4 URLs, got %d: %v", len(urls), urls)
	}

	// External URL should be excluded
	for _, u := range urls {
		if strings.Contains(u, "external.com") {
			t.Errorf("external URL should be filtered: %s", u)
		}
	}
}

func TestMapLimit(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	urls, err := b.Map(srv.URL+"/map-start",
		WithMapSitemap(false),
		WithMapDelay(0),
		WithMapLimit(2),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(urls) > 2 {
		t.Errorf("expected at most 2 URLs, got %d", len(urls))
	}
}

func TestMapDepth(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	// Depth 0: only start page
	urls0, err := b.Map(srv.URL+"/map-start",
		WithMapSitemap(false),
		WithMapDelay(0),
		WithMapMaxDepth(0),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Depth 2: should find sub pages
	urls2, err := b.Map(srv.URL+"/map-start",
		WithMapSitemap(false),
		WithMapDelay(0),
		WithMapMaxDepth(2),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(urls2) <= len(urls0) {
		t.Errorf("depth=2 should find more URLs than depth=0: %d vs %d", len(urls2), len(urls0))
	}
}

func TestMapDedup(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	urls, err := b.Map(srv.URL+"/map-start",
		WithMapSitemap(false),
		WithMapDelay(0),
		WithMapMaxDepth(2),
	)
	if err != nil {
		t.Fatal(err)
	}

	seen := make(map[string]bool)
	for _, u := range urls {
		if seen[u] {
			t.Errorf("duplicate URL: %s", u)
		}

		seen[u] = true
	}
}

func TestMapIncludeExcludePaths(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	// Include only /map-page1*
	urls, err := b.Map(srv.URL+"/map-start",
		WithMapSitemap(false),
		WithMapDelay(0),
		WithMapIncludePaths("/map-page1"),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range urls {
		if !strings.Contains(u, "/map-page1") {
			t.Errorf("URL should match include path: %s", u)
		}
	}

	// Exclude /map-page2
	urls2, err := b.Map(srv.URL+"/map-start",
		WithMapSitemap(false),
		WithMapDelay(0),
		WithMapExcludePaths("/map-page2"),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range urls2 {
		if strings.Contains(u, "/map-page2") {
			t.Errorf("URL should be excluded: %s", u)
		}
	}
}

func TestMapSearchFilter(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	urls, err := b.Map(srv.URL+"/map-start",
		WithMapSitemap(false),
		WithMapDelay(0),
		WithMapSearch("page1"),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range urls {
		if !strings.Contains(strings.ToLower(u), "page1") {
			t.Errorf("URL should contain search term: %s", u)
		}
	}
}

// --- Pure function tests ---

func TestMapDomainAllowed(t *testing.T) {
	tests := []struct {
		url     string
		base    string
		subs    bool
		allowed bool
	}{
		{"https://example.com/page", "example.com", false, true},
		{"https://sub.example.com/page", "example.com", false, false},
		{"https://sub.example.com/page", "example.com", true, true},
		{"https://other.com/page", "example.com", true, false},
	}

	for _, tt := range tests {
		got := mapDomainAllowed(tt.url, tt.base, tt.subs)
		if got != tt.allowed {
			t.Errorf("mapDomainAllowed(%q, %q, %v) = %v, want %v", tt.url, tt.base, tt.subs, got, tt.allowed)
		}
	}
}

func TestMapPathAllowed(t *testing.T) {
	tests := []struct {
		url     string
		include []string
		exclude []string
		allowed bool
	}{
		{"https://x.com/blog/post", []string{"/blog"}, nil, true},
		{"https://x.com/about", []string{"/blog"}, nil, false},
		{"https://x.com/admin", nil, []string{"/admin"}, false},
		{"https://x.com/page", nil, []string{"/admin"}, true},
		{"https://x.com/page", nil, nil, true},
	}

	for _, tt := range tests {
		o := &mapOptions{includePaths: tt.include, excludePaths: tt.exclude}

		got := mapPathAllowed(tt.url, o)
		if got != tt.allowed {
			t.Errorf("mapPathAllowed(%q, include=%v, exclude=%v) = %v, want %v",
				tt.url, tt.include, tt.exclude, got, tt.allowed)
		}
	}
}

func TestMapSearchMatch(t *testing.T) {
	if !mapSearchMatch("https://x.com/blog/go-tips", "go-tips") {
		t.Error("should match path")
	}

	if mapSearchMatch("https://x.com/about", "blog") {
		t.Error("should not match")
	}
}

func TestMapOptions(t *testing.T) {
	o := mapDefaults()
	WithMapLimit(50)(o)
	WithMapSubdomains()(o)
	WithMapDelay(time.Second)(o)
	WithMapMaxDepth(5)(o)
	WithMapSitemap(false)(o)
	WithMapSearch("test")(o)
	WithMapIncludePaths("/a", "/b")(o)
	WithMapExcludePaths("/c")(o)

	if o.limit != 50 || !o.includeSubdoms || o.delay != time.Second || o.maxDepth != 5 {
		t.Error("options not applied")
	}

	if o.useSitemap || o.search != "test" {
		t.Error("options not applied")
	}

	if len(o.includePaths) != 2 || len(o.excludePaths) != 1 {
		t.Error("path options not applied")
	}
}
