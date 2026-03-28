package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/inovacc/scout/internal/idle"
	"github.com/inovacc/scout/internal/metrics"
	"github.com/inovacc/scout/pkg/scout"
	"golang.org/x/time/rate"
)

// ServerConfig holds configuration for the agent HTTP server.
type ServerConfig struct {
	Addr        string        // listen address (default: "localhost:9000")
	Headless    bool
	Stealth     bool
	BrowserBin  string
	Logger      *slog.Logger
	IdleTimeout time.Duration // auto-shutdown after inactivity (0 disables)
	RateLimit   float64       // requests per second (0 = unlimited, default: 100)
	APIKey      string        // optional API key for authentication (empty = no auth)
}

// Server wraps a Provider with an HTTP interface for AI agent frameworks.
type Server struct {
	provider *Provider
	browser  *scout.Browser
	config   ServerConfig
	logger   *slog.Logger
	idle     *idle.Timer
	mux      *http.ServeMux
	limiter  *rate.Limiter
}

// CallRequest is the JSON body for POST /call.
type CallRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// CallResponse is the JSON response for POST /call.
type CallResponse struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// HealthResponse is the JSON response for GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Tools   int    `json:"tools"`
	Version string `json:"version"`
}

// NewServer creates a new agent HTTP server.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:9000"
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	opts := []scout.Option{
		scout.WithHeadless(cfg.Headless),
	}
	if cfg.Stealth {
		opts = append(opts, scout.WithStealth())
	}
	if cfg.BrowserBin != "" {
		opts = append(opts, scout.WithExecPath(cfg.BrowserBin))
	}

	browser, err := scout.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("scout: agent server: browser: %w", err)
	}

	provider := NewProvider(browser)

	rps := cfg.RateLimit
	if rps == 0 {
		rps = 100
	}

	s := &Server{
		provider: provider,
		browser:  browser,
		config:   cfg,
		logger:   cfg.Logger,
		mux:      http.NewServeMux(),
		limiter:  rate.NewLimiter(rate.Limit(rps), int(rps)),
	}

	s.registerRoutes()

	return s, nil
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /tools", s.handleToolsOpenAI)
	s.mux.HandleFunc("GET /tools/openai", s.handleToolsOpenAI)
	s.mux.HandleFunc("GET /tools/anthropic", s.handleToolsAnthropic)
	s.mux.HandleFunc("GET /tools/schema", s.handleToolsSchema)
	s.mux.HandleFunc("POST /call", s.handleCall)
	s.mux.HandleFunc("GET /metrics", metrics.PrometheusHandler())
	s.mux.HandleFunc("GET /metrics/json", metrics.Handler())
}

func (s *Server) touch() {
	if s.idle != nil {
		s.idle.Reset()
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.touch()
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Tools:   len(s.provider.Tools()),
		Version: "1.0.0",
	})
}

func (s *Server) handleToolsOpenAI(w http.ResponseWriter, _ *http.Request) {
	s.touch()
	writeJSON(w, http.StatusOK, s.provider.OpenAITools())
}

func (s *Server) handleToolsAnthropic(w http.ResponseWriter, _ *http.Request) {
	s.touch()
	writeJSON(w, http.StatusOK, s.provider.AnthropicTools())
}

func (s *Server) handleToolsSchema(w http.ResponseWriter, _ *http.Request) {
	s.touch()
	data, err := s.provider.ToolSchemaJSON()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleCall(w http.ResponseWriter, r *http.Request) {
	s.touch()

	var req CallRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing 'name' field"})
		return
	}

	s.logger.Info("tool call", "name", req.Name)

	result, err := s.provider.Call(r.Context(), req.Name, req.Arguments)
	if err != nil {
		metrics.Get().ErrorsTotal.Add(1)
		writeJSON(w, http.StatusNotFound, CallResponse{Content: err.Error(), IsError: true})
		return
	}

	metrics.Get().ToolCallsTotal.Add(1)
	if result.IsError {
		metrics.Get().ErrorsTotal.Add(1)
	}

	writeJSON(w, http.StatusOK, result)
}

// corsMiddleware adds CORS headers and handles preflight OPTIONS requests.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware rejects requests with 429 when the rate limit is exceeded.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow() {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authMiddleware rejects requests without a valid Bearer token when APIKey is set.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if no API key configured.
		if s.config.APIKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Allow health and metrics without auth.
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/metrics") {
			next.ServeHTTP(w, r)
			return
		}

		// Check Bearer token.
		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing Authorization header"})
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth || token != s.config.APIKey {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ListenAndServe starts the HTTP server. If IdleTimeout is configured and
// onIdle is provided, the server will call onIdle (typically context cancel)
// after IdleTimeout of inactivity.
func (s *Server) ListenAndServe(ctx context.Context, onIdle ...func()) error {
	if s.config.IdleTimeout > 0 && len(onIdle) > 0 {
		s.idle = idle.New(s.config.IdleTimeout, onIdle[0])
		defer s.idle.Stop()
	}

	ln, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("scout: agent server: listen: %w", err)
	}

	s.logger.Info("agent HTTP server started", "addr", ln.Addr().String())

	srv := &http.Server{
		Handler:           corsMiddleware(s.authMiddleware(s.rateLimitMiddleware(s.mux))),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("scout: agent server: serve: %w", err)
	}

	return nil
}

// Close shuts down the server and browser.
func (s *Server) Close() {
	if s.browser != nil {
		_ = s.browser.Close()
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
