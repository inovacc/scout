// Package proxy provides an HTTP reverse proxy that turns websites into
// REST/JSON endpoints using browser automation. Routes are defined via YAML
// config, each mapping an HTTP path to a target URL with extraction rules.
//
// Usage:
//
//	cfg, _ := proxy.LoadConfig("routes.yaml")
//	srv, _ := proxy.New(cfg)
//	srv.ListenAndServe(":8080")
package proxy

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/urlpolicy"
	"gopkg.in/yaml.v3"
)

// Config is the top-level proxy configuration.
type Config struct {
	Routes   []Route       `yaml:"routes" json:"routes"`
	Defaults RouteDefaults `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	// Token, when set (or via SCOUT_PROXY_TOKEN), requires every request except
	// /health to present "Authorization: Bearer <token>".
	Token string `yaml:"auth_token,omitempty" json:"auth_token,omitempty"`
}

// RouteDefaults are applied to all routes unless overridden.
type RouteDefaults struct {
	CacheTTL string `yaml:"cache_ttl,omitempty" json:"cache_ttl,omitempty"` // e.g. "5m"
	Timeout  string `yaml:"timeout,omitempty" json:"timeout,omitempty"`     // e.g. "30s"
	Stealth  bool   `yaml:"stealth,omitempty" json:"stealth,omitempty"`
}

// Route defines a single API endpoint backed by browser scraping.
type Route struct {
	Path     string        `yaml:"path" json:"path"`
	Method   string        `yaml:"method,omitempty" json:"method,omitempty"` // GET (default)
	Target   string        `yaml:"target" json:"target"`                     // URL template, supports {{.param}}
	Params   []string      `yaml:"params,omitempty" json:"params,omitempty"` // query params to inject into target
	Extract  ExtractConfig `yaml:"extract" json:"extract"`
	CacheTTL string        `yaml:"cache_ttl,omitempty" json:"cache_ttl,omitempty"`
	Timeout  string        `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// ExtractConfig defines what to extract from the target page.
type ExtractConfig struct {
	Selector string            `yaml:"selector" json:"selector"`                 // CSS selector for item container
	Fields   map[string]string `yaml:"fields" json:"fields"`                     // field → CSS selector or selector@attr
	Single   bool              `yaml:"single,omitempty" json:"single,omitempty"` // extract single item vs list
}

// LoadConfig reads a proxy configuration from a YAML or JSON file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("proxy: read config: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("proxy: parse config: %w", err)
	}

	return &cfg, nil
}

// Server is the API proxy HTTP server.
type Server struct {
	config  *Config
	token   string
	policy  *urlpolicy.Policy
	mux     *http.ServeMux
	cache   *responseCache
	logger  *slog.Logger
	browser *scout.Browser
	mu      sync.Mutex
}

// New creates a new proxy server from configuration.
func New(cfg *Config) (*Server, error) {
	token := cfg.Token
	if token == "" {
		token = os.Getenv("SCOUT_PROXY_TOKEN")
	}

	s := &Server{
		config: cfg,
		token:  token,
		mux:    http.NewServeMux(),
		cache:  newResponseCache(),
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		policy: urlpolicy.FromEnv(),
	}

	// Register routes.
	for _, route := range cfg.Routes {
		r := route // capture
		method := strings.ToUpper(r.Method)
		if method == "" {
			method = "GET"
		}

		pattern := method + " " + r.Path
		s.mux.HandleFunc(pattern, s.handleRoute(r))
	}

	// Health endpoint.
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"routes": len(cfg.Routes),
		})
	})

	// Routes listing.
	s.mux.HandleFunc("GET /routes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg.Routes)
	})

	return s, nil
}

// ListenAndServe starts the proxy server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	if err := guardProxyBind(addr, s.token); err != nil {
		return err
	}

	s.logger.Info("proxy starting", "addr", addr, "routes", len(s.config.Routes), "auth", s.token != "")

	srv := &http.Server{
		Addr:              addr,
		Handler:           s.authMiddleware(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	return srv.ListenAndServe()
}

// authMiddleware enforces a Bearer token on every route except /health when a
// token is configured (Config.Token or SCOUT_PROXY_TOKEN).
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
			http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// guardProxyBind fails closed: the proxy navigates to caller-influenced URLs, so
// it must not listen on a non-loopback address unless it is authenticated (a
// token is set) or the operator explicitly opts in via SCOUT_PROXY_ALLOW_REMOTE.
func guardProxyBind(addr, token string) error {
	if token != "" {
		return nil
	}

	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}

	if proxyLoopbackHost(host) {
		return nil
	}

	if v := os.Getenv("SCOUT_PROXY_ALLOW_REMOTE"); v == "1" || v == "true" {
		return nil
	}

	return fmt.Errorf("scout: proxy: refusing to bind non-loopback address %q with no auth_token; set Config.Token / SCOUT_PROXY_TOKEN, or SCOUT_PROXY_ALLOW_REMOTE=1", addr)
}

// proxyLoopbackHost reports whether host is a loopback address. Empty, "0.0.0.0"
// and "::" are treated as non-loopback.
func proxyLoopbackHost(host string) bool {
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}

	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}

	return false
}

// Close shuts down the proxy and releases browser resources.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.browser != nil {
		return s.browser.Close()
	}

	return nil
}

func (s *Server) ensureBrowser() (*scout.Browser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.browser != nil {
		return s.browser, nil
	}

	opts := []scout.Option{
		scout.WithHeadless(true),
		scout.WithTimeout(0),
	}

	if s.config.Defaults.Stealth {
		opts = append(opts, scout.WithStealth())
	}

	b, err := scout.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("proxy: launch browser: %w", err)
	}

	s.browser = b

	return b, nil
}

func (s *Server) handleRoute(route Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Build target URL from template + query params.
		targetURL := route.Target
		for _, param := range route.Params {
			val := r.URL.Query().Get(param)
			targetURL = strings.ReplaceAll(targetURL, "{{."+param+"}}", val)
		}

		// SSRF guard: the target is built from caller-influenced query params, so
		// validate it against the policy (default-deny internal/loopback) before
		// navigating, unless the operator opted in via SCOUT_ALLOW_LOCAL_TARGETS.
		if err := s.policy.Check(r.Context(), targetURL); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusForbidden)

			return
		}

		// Check cache.
		ttl := parseDuration(route.CacheTTL, parseDuration(s.config.Defaults.CacheTTL, 0))
		if ttl > 0 {
			if cached, ok := s.cache.get(targetURL); ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				_, _ = w.Write(cached)

				return
			}
		}

		// Navigate and extract.
		timeout := parseDuration(route.Timeout, parseDuration(s.config.Defaults.Timeout, 30*time.Second))

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		result, err := s.scrapeRoute(ctx, targetURL, route.Extract)
		if err != nil {
			s.logger.Error("proxy: scrape failed", "path", route.Path, "target", targetURL, "error", err)
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusBadGateway)

			return
		}

		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, `{"error": "marshal failed"}`, http.StatusInternalServerError)

			return
		}

		// Cache response.
		if ttl > 0 {
			s.cache.set(targetURL, data, ttl)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "MISS")
		_, _ = w.Write(data)
	}
}

func (s *Server) scrapeRoute(ctx context.Context, targetURL string, extract ExtractConfig) (any, error) {
	browser, err := s.ensureBrowser()
	if err != nil {
		return nil, err
	}

	page, err := browser.NewPage(targetURL)
	if err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	_ = page.WaitLoad()

	// Allow dynamic content to render.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(1 * time.Second):
	}

	if extract.Single {
		return extractSingle(page, extract)
	}

	return extractList(page, extract)
}

func extractList(page *scout.Page, extract ExtractConfig) ([]map[string]string, error) {
	js := buildListExtractionJS(extract)

	result, err := page.Eval(js)
	if err != nil {
		return nil, fmt.Errorf("eval: %w", err)
	}

	raw := result.String()
	if raw == "" || raw == "null" || raw == "[]" {
		return []map[string]string{}, nil
	}

	var items []map[string]string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parse results: %w", err)
	}

	return items, nil
}

func extractSingle(page *scout.Page, extract ExtractConfig) (map[string]string, error) {
	js := buildSingleExtractionJS(extract)

	result, err := page.Eval(js)
	if err != nil {
		return nil, fmt.Errorf("eval: %w", err)
	}

	raw := result.String()
	if raw == "" || raw == "null" {
		return map[string]string{}, nil
	}

	var item map[string]string
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	return item, nil
}

func buildListExtractionJS(extract ExtractConfig) string {
	fieldJS := buildFieldExtractors(extract.Fields)

	return fmt.Sprintf(`(() => {
		const items = document.querySelectorAll(%q);
		const results = [];
		items.forEach(item => {
			const obj = {};
			%s
			results.push(obj);
		});
		return JSON.stringify(results);
	})()`, extract.Selector, fieldJS)
}

func buildSingleExtractionJS(extract ExtractConfig) string {
	fieldJS := buildFieldExtractors(extract.Fields)
	sel := extract.Selector
	if sel == "" {
		sel = "body"
	}

	return fmt.Sprintf(`(() => {
		const item = document.querySelector(%q);
		if (!item) return null;
		const obj = {};
		%s
		return JSON.stringify(obj);
	})()`, sel, fieldJS)
}

func buildFieldExtractors(fields map[string]string) string {
	var parts []string

	for name, sel := range fields {
		// Check for @attr suffix.
		if atIdx := strings.LastIndex(sel, "@"); atIdx > 0 {
			css := sel[:atIdx]
			attr := sel[atIdx+1:]
			parts = append(parts, fmt.Sprintf(
				`{ const el = item.querySelector(%q); obj[%q] = el ? (el.getAttribute(%q) || '') : ''; }`,
				css, name, attr))
		} else {
			parts = append(parts, fmt.Sprintf(
				`{ const el = item.querySelector(%q); obj[%q] = el ? el.textContent.trim() : ''; }`,
				sel, name))
		}
	}

	return strings.Join(parts, "\n\t\t\t")
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}

	return d
}
