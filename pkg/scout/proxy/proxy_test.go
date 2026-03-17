package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.yaml")

	content := `
routes:
  - path: /api/v1/products
    method: GET
    target: https://example.com/catalog
    extract:
      selector: ".product"
      fields:
        name: "h3"
        price: ".price"
    cache_ttl: 5m

  - path: /api/v1/search
    target: "https://example.com/search?q={{.query}}"
    params: [query]
    extract:
      selector: ".result"
      fields:
        title: "a"
        url: "a@href"

defaults:
  stealth: true
  timeout: 30s
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(cfg.Routes) != 2 {
		t.Fatalf("routes count = %d, want 2", len(cfg.Routes))
	}

	if cfg.Routes[0].Path != "/api/v1/products" {
		t.Errorf("route[0].Path = %q", cfg.Routes[0].Path)
	}

	if cfg.Routes[0].CacheTTL != "5m" {
		t.Errorf("route[0].CacheTTL = %q", cfg.Routes[0].CacheTTL)
	}

	if cfg.Routes[1].Params[0] != "query" {
		t.Errorf("route[1].Params = %v", cfg.Routes[1].Params)
	}

	if !cfg.Defaults.Stealth {
		t.Error("defaults.Stealth = false")
	}
}

func TestBuildListExtractionJS(t *testing.T) {
	extract := ExtractConfig{
		Selector: ".product",
		Fields: map[string]string{
			"name":  "h3",
			"price": ".price",
			"link":  "a@href",
		},
	}

	js := buildListExtractionJS(extract)

	if js == "" {
		t.Error("empty JS")
	}

	// Should contain the selector.
	if !containsStr(js, ".product") {
		t.Error("missing selector in JS")
	}

	// Should handle @attr.
	if !containsStr(js, "getAttribute") {
		t.Error("missing getAttribute for @attr fields")
	}
}

func TestBuildSingleExtractionJS(t *testing.T) {
	extract := ExtractConfig{
		Selector: "#main",
		Fields: map[string]string{
			"title": "h1",
		},
		Single: true,
	}

	js := buildSingleExtractionJS(extract)
	if !containsStr(js, "#main") {
		t.Error("missing selector")
	}
}

func TestResponseCache(t *testing.T) {
	c := newResponseCache()

	// Miss.
	if _, ok := c.get("key"); ok {
		t.Error("expected cache miss")
	}

	// Set and hit.
	c.set("key", []byte(`{"data":1}`), 5*time.Second)

	data, ok := c.get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["data"] != float64(1) {
		t.Errorf("cached data = %v", parsed)
	}

	// Expired entry.
	c.set("expired", []byte(`{}`), -1*time.Second)

	if _, ok := c.get("expired"); ok {
		t.Error("expected miss for expired entry")
	}
}

func TestParseDuration(t *testing.T) {
	if d := parseDuration("5m", time.Second); d != 5*time.Minute {
		t.Errorf("parseDuration(5m) = %v", d)
	}

	if d := parseDuration("", 3*time.Second); d != 3*time.Second {
		t.Errorf("parseDuration('') = %v", d)
	}

	if d := parseDuration("bad", 7*time.Second); d != 7*time.Second {
		t.Errorf("parseDuration(bad) = %v", d)
	}
}

func TestNew(t *testing.T) {
	cfg := &Config{
		Routes: []Route{
			{
				Path:   "/api/test",
				Target: "https://example.com",
				Extract: ExtractConfig{
					Selector: "div",
					Fields:   map[string]string{"text": "p"},
				},
			},
		},
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if srv.mux == nil {
		t.Error("mux is nil")
	}

	if err := srv.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
