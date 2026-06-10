package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := LoadConfig(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
		if err == nil {
			t.Fatal("expected error for missing file")
		}

		if !strings.Contains(err.Error(), "proxy: read config") {
			t.Errorf("error = %v, want prefix 'proxy: read config'", err)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.yaml")
		// Unbalanced bracket / invalid mapping makes the YAML unparseable.
		if err := os.WriteFile(path, []byte("routes: [unclosed\n  : :"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := LoadConfig(path)
		if err == nil {
			t.Fatal("expected parse error")
		}

		if !strings.Contains(err.Error(), "proxy: parse config") {
			t.Errorf("error = %v, want prefix 'proxy: parse config'", err)
		}
	})

	t.Run("empty file yields empty config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.yaml")
		if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}

		if cfg == nil {
			t.Fatal("cfg is nil")
		}

		if len(cfg.Routes) != 0 {
			t.Errorf("routes = %d, want 0", len(cfg.Routes))
		}
	})
}

func TestLoadConfig_EnvExpansion(t *testing.T) {
	t.Setenv("PROXY_TEST_TARGET", "https://expanded.example.com")

	dir := t.TempDir()
	path := filepath.Join(dir, "env.yaml")
	content := `
routes:
  - path: /api/v1/items
    target: ${PROXY_TEST_TARGET}/list
    extract:
      selector: ".item"
      fields:
        name: "h2"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := "https://expanded.example.com/list"
	if got := cfg.Routes[0].Target; got != want {
		t.Errorf("target = %q, want %q", got, want)
	}
}

func TestLoadConfig_JSON(t *testing.T) {
	// YAML is a superset of JSON, so LoadConfig parses JSON content too.
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.json")
	content := `{
		"routes": [
			{
				"path": "/api/json",
				"method": "post",
				"target": "https://example.com/api",
				"extract": {"selector": ".x", "fields": {"a": "b"}, "single": true}
			}
		],
		"defaults": {"cache_ttl": "1m", "stealth": false}
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(cfg.Routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(cfg.Routes))
	}

	r := cfg.Routes[0]
	// LoadConfig preserves the method verbatim (lowercase here); normalization to
	// upper happens later at route registration, not at load time.
	if r.Path != "/api/json" || r.Method != strings.ToLower(http.MethodPost) {
		t.Errorf("route = %+v", r)
	}

	if !r.Extract.Single {
		t.Error("expected Single=true")
	}

	if cfg.Defaults.CacheTTL != "1m" {
		t.Errorf("defaults.CacheTTL = %q", cfg.Defaults.CacheTTL)
	}
}

func TestBuildFieldExtractors(t *testing.T) {
	tests := []struct {
		name      string
		fields    map[string]string
		wantSubs  []string // substrings that must appear
		notWantSub string  // substring that must NOT appear ("" to skip)
	}{
		{
			name:   "text field uses textContent",
			fields: map[string]string{"title": "h1"},
			wantSubs: []string{
				`querySelector("h1")`,
				`obj["title"]`,
				"textContent.trim()",
			},
			notWantSub: "getAttribute",
		},
		{
			name:   "attr field uses getAttribute",
			fields: map[string]string{"link": "a@href"},
			wantSubs: []string{
				`querySelector("a")`,
				`obj["link"]`,
				`getAttribute("href")`,
			},
			notWantSub: "textContent",
		},
		{
			name:   "leading @ is treated as plain selector not attr",
			fields: map[string]string{"weird": "@foo"},
			// LastIndex("@foo","@")==0 which is not > 0, so it stays a text field.
			wantSubs: []string{
				`querySelector("@foo")`,
				"textContent.trim()",
			},
			notWantSub: "getAttribute",
		},
		{
			name:   "last @ wins for selector with multiple @",
			fields: map[string]string{"data": "div[data-x]@data-id"},
			wantSubs: []string{
				`querySelector("div[data-x]")`,
				`getAttribute("data-id")`,
			},
			notWantSub: "",
		},
		{
			name:     "empty fields yields empty string",
			fields:   map[string]string{},
			wantSubs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFieldExtractors(tt.fields)

			if len(tt.fields) == 0 {
				if got != "" {
					t.Errorf("expected empty string, got %q", got)
				}

				return
			}

			for _, sub := range tt.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("output missing %q\ngot: %s", sub, got)
				}
			}

			if tt.notWantSub != "" && strings.Contains(got, tt.notWantSub) {
				t.Errorf("output should not contain %q\ngot: %s", tt.notWantSub, got)
			}
		})
	}
}

func TestBuildFieldExtractors_MultipleFieldsJoined(t *testing.T) {
	got := buildFieldExtractors(map[string]string{
		"a": "h1",
		"b": "h2",
	})

	// Two separate field-extractor blocks joined by a newline-tab separator.
	if n := strings.Count(got, "const el ="); n != 2 {
		t.Errorf("expected 2 field blocks, got %d\n%s", n, got)
	}

	if !strings.Contains(got, "\n") {
		t.Error("expected joined blocks to be newline-separated")
	}
}

func TestBuildSingleExtractionJS_EmptySelectorDefaultsToBody(t *testing.T) {
	js := buildSingleExtractionJS(ExtractConfig{
		Selector: "",
		Fields:   map[string]string{"title": "h1"},
		Single:   true,
	})

	if !strings.Contains(js, `querySelector("body")`) {
		t.Errorf("empty selector should default to body, got:\n%s", js)
	}

	// Guard clause for missing element must be present.
	if !strings.Contains(js, "if (!item) return null;") {
		t.Error("missing null guard")
	}
}

func TestBuildListExtractionJS_Structure(t *testing.T) {
	js := buildListExtractionJS(ExtractConfig{
		Selector: ".row",
		Fields:   map[string]string{"name": "td"},
	})

	for _, sub := range []string{
		`querySelectorAll(".row")`,
		"results.push(obj)",
		"JSON.stringify(results)",
	} {
		if !strings.Contains(js, sub) {
			t.Errorf("list JS missing %q\n%s", sub, js)
		}
	}
}

// TestHandleRoute_CacheHit exercises the cache-HIT branch of handleRoute, which
// returns before any browser/network call. We pre-seed the cache so the request
// never reaches ensureBrowser/scrapeRoute.
func TestHandleRoute_CacheHit(t *testing.T) {
	cfg := &Config{
		Routes: []Route{
			{
				Path:     "/api/cached",
				Target:   "https://example.com/data?q={{.q}}",
				Params:   []string{"q"},
				CacheTTL: "5m",
				Extract:  ExtractConfig{Selector: ".x", Fields: map[string]string{"a": "b"}},
			},
		},
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = srv.Close() }()

	// The cache key is the fully-rendered target URL after param substitution.
	cachedBody := []byte(`[{"a":"cached-value"}]`)
	srv.cache.set("https://example.com/data?q=widgets", cachedBody, 5*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/api/cached?q=widgets", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("X-Cache"); got != "HIT" {
		t.Errorf("X-Cache = %q, want HIT", got)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}

	if rec.Body.String() != string(cachedBody) {
		t.Errorf("body = %q, want %q", rec.Body.String(), cachedBody)
	}
}

// TestHandleRoute_MethodAndPathRouting confirms New wires routes under the
// correct method+path and that a non-registered method 404s, all without a
// browser (we use a cache hit to avoid scraping).
func TestHandleRoute_MethodRouting(t *testing.T) {
	cfg := &Config{
		Routes: []Route{
			{
				Path:     "/api/post-only",
				Method:   "POST",
				Target:   "https://example.com/p",
				CacheTTL: "5m",
				Extract:  ExtractConfig{Selector: ".x", Fields: map[string]string{"a": "b"}},
			},
		},
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = srv.Close() }()

	srv.cache.set("https://example.com/p", []byte(`[{"ok":"1"}]`), 5*time.Minute)

	// Correct method hits the cached route.
	post := httptest.NewRecorder()
	srv.mux.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/post-only", nil))
	if post.Code != http.StatusOK {
		t.Errorf("POST status = %d, want 200", post.Code)
	}

	// Wrong method on a POST-only path should not match -> 405 (ServeMux).
	get := httptest.NewRecorder()
	srv.mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/post-only", nil))
	if get.Code == http.StatusOK {
		t.Errorf("GET on POST-only route returned 200, expected non-200")
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	cfg := &Config{
		Routes: []Route{
			{Path: "/a", Target: "https://x", Extract: ExtractConfig{Selector: "x", Fields: map[string]string{"f": "g"}}},
			{Path: "/b", Target: "https://y", Extract: ExtractConfig{Selector: "y", Fields: map[string]string{"f": "g"}}},
		},
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = srv.Close() }()

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status field = %v", body["status"])
	}

	if body["routes"] != float64(2) {
		t.Errorf("routes field = %v, want 2", body["routes"])
	}
}

func TestServer_RoutesEndpoint(t *testing.T) {
	cfg := &Config{
		Routes: []Route{
			{Path: "/one", Method: "GET", Target: "https://x", Extract: ExtractConfig{Selector: "x", Fields: map[string]string{"f": "g"}}},
		},
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = srv.Close() }()

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/routes", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var routes []Route
	if err := json.Unmarshal(rec.Body.Bytes(), &routes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(routes) != 1 || routes[0].Path != "/one" {
		t.Errorf("routes = %+v", routes)
	}
}

func TestServer_Close_NilBrowserIdempotent(t *testing.T) {
	srv, err := New(&Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// browser was never launched -> Close must be a no-op, repeatable.
	if err := srv.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}

	if err := srv.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestNew_DefaultMethodIsGET(t *testing.T) {
	// A route with no Method should be registered as GET and cache-hit.
	cfg := &Config{
		Routes: []Route{
			{
				Path:     "/api/default-method",
				Target:   "https://example.com/d",
				CacheTTL: "5m",
				Extract:  ExtractConfig{Selector: ".x", Fields: map[string]string{"a": "b"}},
			},
		},
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = srv.Close() }()

	srv.cache.set("https://example.com/d", []byte(`[]`), 5*time.Minute)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/default-method", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET status = %d, want 200 (default method should be GET)", rec.Code)
	}

	if rec.Header().Get("X-Cache") != "HIT" {
		t.Errorf("expected cache HIT, X-Cache=%q", rec.Header().Get("X-Cache"))
	}
}
