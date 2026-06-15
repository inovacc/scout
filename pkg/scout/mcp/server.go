package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/idle"
	"github.com/inovacc/scout/internal/interaction"
	"github.com/inovacc/scout/internal/metrics"
	"github.com/inovacc/scout/internal/redact"
	"github.com/inovacc/scout/internal/tracing"
	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/aria"
	"github.com/inovacc/scout/pkg/scout/plugin"
	"github.com/inovacc/scout/pkg/scout/urlpolicy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerConfig holds configuration for the MCP server.
type ServerConfig struct {
	Headless      bool
	Stealth       bool
	BrowserBin    string
	Logger        *slog.Logger
	IdleTimeout   time.Duration   // auto-shutdown after inactivity (0 disables)
	PluginManager *plugin.Manager // optional plugin manager for dynamic tools
}

// mcpState holds the lazy-initialized browser and current page.
type mcpState struct {
	mu        sync.Mutex
	browser   *scout.Browser
	page      *scout.Page
	config    ServerConfig
	idle      *idle.Timer
	ariaStore *aria.Store
	hooks     *hookRegistry
	policy    *urlpolicy.Policy
}

// touch resets the idle timer on activity.
func (s *mcpState) touch() {
	if s.idle != nil {
		s.idle.Reset()
	}
}

func (s *mcpState) ensureBrowser(_ context.Context) (*scout.Browser, error) {
	s.touch()
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.browser != nil {
		return s.browser, nil
	}

	opts := []scout.Option{
		scout.WithHeadless(s.config.Headless),
		scout.WithNoSandbox(),
		scout.WithTimeout(0), // disable rod's 30s page timeout; MCP manages its own lifecycle
	}
	if s.config.BrowserBin != "" {
		opts = append(opts, scout.WithExecPath(s.config.BrowserBin))
	}

	if s.config.Stealth {
		opts = append(opts, scout.WithStealth())
	}

	b, err := scout.New(opts...) //nolint:contextcheck
	if err != nil {
		return nil, fmt.Errorf("scout-mcp: launch browser: %w", err)
	}

	// Enforce the SSRF URL-policy on EVERY outbound request (redirects and
	// in-page fetch from the eval tool), not just the navigate-time check.
	if s.policy != nil {
		policy := s.policy
		b.InstallRequestFilter(func(rawURL string) bool {
			return policy.Check(context.Background(), rawURL) == nil
		})
	}

	s.browser = b

	return b, nil
}

func (s *mcpState) ensurePage(ctx context.Context) (*scout.Page, error) {
	if _, err := s.ensureBrowser(ctx); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.page != nil {
		return s.page, nil
	}

	p, err := s.browser.NewPage("")
	if err != nil {
		return nil, fmt.Errorf("scout-mcp: create page: %w", err)
	}

	s.page = p
	metrics.Get().PagesCreated.Add(1)
	metrics.Get().PagesActive.Add(1)

	return p, nil
}

// checkURL enforces the SSRF URL-policy for untrusted MCP callers. A nil policy
// (should not happen) allows everything.
func (s *mcpState) checkURL(ctx context.Context, rawURL string) error {
	if s.policy == nil {
		return nil
	}
	return s.policy.Check(ctx, rawURL)
}

func (s *mcpState) reset() {
	s.mu.Lock()

	// Close page first to terminate its CDP session before killing the browser process.
	if s.page != nil {
		_ = s.page.Close()
		metrics.Get().PagesActive.Add(-1)
	}

	hadBrowser := s.browser != nil
	if hadBrowser {
		_ = s.browser.Close()
	}

	s.browser = nil
	s.page = nil
	s.mu.Unlock()

	// Allow the OS to fully release CDP port and temp dirs before re-init.
	if hadBrowser {
		time.Sleep(500 * time.Millisecond)
	}
}

// addTracedTool registers an MCP tool with OpenTelemetry tracing instrumentation.
func addTracedTool(server *mcp.Server, tool *mcp.Tool, handler func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	name := tool.Name
	server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scout: mcp: recovered panic", "tool", name, "panic", r, "stack", string(debug.Stack()))
				err = fmt.Errorf("scout: mcp: internal error in tool %s: %v", name, r)
				result = nil
			}
		}()

		start := time.Now()
		ctx, finish := tracing.MCPToolSpan(ctx, name)

		result, err = handler(ctx, req)

		metrics.Get().ToolCallsTotal.Add(1)

		switch {
		case err != nil:
			metrics.Get().ErrorsTotal.Add(1)
			finish(err)
		case result != nil && result.IsError:
			metrics.Get().ErrorsTotal.Add(1)
			finish(fmt.Errorf("tool error"))
		default:
			finish(nil)
		}

		ok := err == nil && (result == nil || !result.IsError)

		ev := interaction.Event{
			Kind:       "mcp_tool",
			Source:     "mcp",
			Name:       name,
			OK:         &ok,
			DurationMS: time.Since(start).Milliseconds(),
		}
		if err != nil {
			ev.Error = err.Error()
		}
		if args := mcpToolArgs(req); args != nil {
			ev.Input = redact.Map(args)
		}

		interaction.Emit(ev)

		return result, err
	})
}

// mcpToolArgs returns the call's arguments as a map for capture, or nil.
func mcpToolArgs(req *mcp.CallToolRequest) map[string]any {
	if req == nil {
		return nil
	}
	if len(req.Params.Arguments) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(req.Params.Arguments, &m) == nil {
		return m
	}
	return nil
}

func errResult(msg string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}, nil
}

func textResult(msg string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, nil
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errResult(fmt.Sprintf("scout-mcp: marshal: %s", err))
	}

	return textResult(string(data))
}

// NewServer creates an MCP server with Scout tools and resources.
// If cancelOnIdle is non-nil and cfg.IdleTimeout > 0, the idle timer will
// call cancelOnIdle when the timeout expires.
// If cfg.PluginManager is set, plugin-provided MCP tools are registered.
func NewServer(cfg ServerConfig, cancelOnIdle ...func()) *mcp.Server {
	state := &mcpState{config: cfg, ariaStore: aria.NewStore(), hooks: newHookRegistry(), policy: urlpolicy.FromEnv()}

	if cfg.IdleTimeout > 0 && len(cancelOnIdle) > 0 && cancelOnIdle[0] != nil {
		cb := cancelOnIdle[0]
		state.idle = idle.New(cfg.IdleTimeout, func() {
			if cfg.Logger != nil {
				cfg.Logger.Warn("idle timeout reached, shutting down", "timeout", cfg.IdleTimeout)
			}

			state.reset()
			cb()
		})
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "scout", Version: "1.0.0"},
		&mcp.ServerOptions{Logger: logger},
	)

	registerBrowserTools(server, state)
	registerCaptureTools(server, state)
	registerSessionTools(server, state)
	registerSwarmTools(server, state)
	registerWebSocketTools(server, state)
	registerAriaTools(server, state)
	registerGatherTool(server, state)   // unified via pkg/scout/tools
	registerTestSiteTool(server, state) // unified via pkg/scout/tools
	registerSitemapTool(server, state)  // unified via pkg/scout/tools
	registerReportTools(server, state)  // unified via pkg/scout/tools
	registerRunbookTools(server, state) // unified via pkg/scout/tools
	registerCrawlTool(server, state)    // unified via pkg/scout/tools
	registerFormTools(server, state)    // unified via pkg/scout/tools
	registerResources(server, state)

	if cfg.PluginManager != nil {
		cfg.PluginManager.RegisterMCPTools(server)
	}

	return server
}

// RegisterWebMCPTools adds discovered WebMCP tools to the MCP server.
// Each tool is registered with a namespaced name like "webmcp_<origin>_<name>".
// The callFn is invoked when the tool is called, wrapping page.CallWebMCPTool.
func RegisterWebMCPTools(server *mcp.Server, tools []scout.WebMCPTool, callFn func(name string, params map[string]any) (*scout.WebMCPToolResult, error)) {
	for _, t := range tools {
		tool := t // capture

		origin := sanitizeMCPName(tool.ServerURL)
		if origin == "" {
			origin = sanitizeMCPName(tool.Source)
		}

		mcpName := "webmcp_" + origin + "_" + sanitizeMCPName(tool.Name)

		schema := tool.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		server.AddTool(&mcp.Tool{
			Name:        mcpName,
			Description: fmt.Sprintf("[WebMCP] %s", tool.Description),
			InputSchema: schema,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args map[string]any
			if len(req.Params.Arguments) > 0 {
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					return errResult(err.Error())
				}
			}

			result, err := callFn(tool.Name, args)
			if err != nil {
				return errResult(err.Error())
			}

			if result.IsError {
				return errResult(result.Content)
			}

			return textResult(result.Content)
		})
	}
}

// sanitizeMCPName replaces non-alphanumeric characters with underscores for tool naming.
func sanitizeMCPName(s string) string {
	var b []byte

	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b = append(b, c)
		} else if len(b) > 0 && b[len(b)-1] != '_' {
			b = append(b, '_')
		}
	}
	// Trim trailing underscore.
	if len(b) > 0 && b[len(b)-1] == '_' {
		b = b[:len(b)-1]
	}

	return string(b)
}

// Serve starts the MCP server on stdio. Blocks until context is cancelled or
// idle timeout expires.
func Serve(ctx context.Context, logger *slog.Logger, headless, stealth bool, browserBin string, idleTimeout time.Duration) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cfg := ServerConfig{
		Headless:    headless,
		Stealth:     stealth,
		BrowserBin:  browserBin,
		Logger:      logger,
		IdleTimeout: idleTimeout,
	}

	interaction.Init("mcp")
	defer func() { _ = interaction.Close("ok") }()

	server := NewServer(cfg, cancel)

	return server.Run(ctx, &mcp.StdioTransport{})
}

// ServeSSE starts the MCP server with HTTP+SSE transport on the given address.
// Blocks until the context is cancelled or idle timeout expires.
func ServeSSE(ctx context.Context, logger *slog.Logger, addr string, headless, stealth bool, browserBin string, idleTimeout time.Duration) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cfg := ServerConfig{
		Headless:    headless,
		Stealth:     stealth,
		BrowserBin:  browserBin,
		Logger:      logger,
		IdleTimeout: idleTimeout,
	}

	interaction.Init("mcp")
	defer func() { _ = interaction.Close("ok") }()

	handler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		return NewServer(cfg, cancel)
	}, nil)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	if err := guardSSEBind(addr); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("scout: mcp: %w", err)
	}

	logger.Info("MCP SSE server listening", "addr", ln.Addr().String())

	errCh := make(chan error, 1)

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("scout: mcp: %w", err)
		}

		close(errCh)
	}()

	select {
	case <-ctx.Done():
		_ = srv.Close()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// guardSSEBind fails closed: the MCP SSE transport exposes full browser control
// (navigate/eval/...) with no authentication, so it must not listen on a
// non-loopback address unless the operator opts in via SCOUT_MCP_ALLOW_REMOTE
// (intended only when fronted by an authenticating proxy).
func guardSSEBind(addr string) error {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}

	if mcpLoopbackHost(host) {
		return nil
	}

	if v := os.Getenv("SCOUT_MCP_ALLOW_REMOTE"); v == "1" || v == "true" {
		return nil
	}

	return fmt.Errorf("scout: mcp: refusing to bind non-loopback address %q for the unauthenticated SSE transport; bind localhost or set SCOUT_MCP_ALLOW_REMOTE=1 (only behind an authenticating proxy)", addr)
}

// mcpLoopbackHost reports whether host is a loopback address. Empty, "0.0.0.0"
// and "::" are treated as non-loopback.
func mcpLoopbackHost(host string) bool {
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}

	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}

	return false
}
