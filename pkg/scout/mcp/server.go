package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
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

	// openBrowsers are headed inspection browsers spawned by the "open" tool.
	// They intentionally outlive a single call (the user drives them), so they
	// are NOT closed on idle — only on full server teardown (Serve).
	openBrowsers []*scout.Browser
}

// trackOpenBrowser registers a headed inspection browser for teardown cleanup.
func (s *mcpState) trackOpenBrowser(b *scout.Browser) {
	s.mu.Lock()
	s.openBrowsers = append(s.openBrowsers, b)
	s.mu.Unlock()
}

// closeOpenBrowsers closes every headed inspection browser. Called only on full
// server teardown so a client disconnect does not orphan the Chrome process.
func (s *mcpState) closeOpenBrowsers() {
	s.mu.Lock()
	browsers := s.openBrowsers
	s.openBrowsers = nil
	s.mu.Unlock()

	for _, b := range browsers {
		_ = b.Close()
	}
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
		// Liveness check: if Chrome's process has exited (crash, OOM, killed by
		// the user or the reaper), drop the dead handle and re-launch instead of
		// returning a wedged CDP connection that errors on every call forever.
		// Browser.Done() is closed when the launcher process exits.
		select {
		case <-s.browser.Done():
			_ = s.browser.Close()
			s.browser = nil
			s.page = nil
		default:
			return s.browser, nil
		}
	}

	opts := []scout.Option{
		scout.WithHeadless(s.config.Headless),
		scout.WithNoSandbox(),
		// Real per-operation timeout (Page.timed applies it per call, not as an
		// absolute page-lifetime deadline). A bad selector now fails this one
		// tool call instead of blocking the single stdio transport forever.
		scout.WithTimeout(60 * time.Second),
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
		err = annotateBrowserError(err)

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

// annotateBrowserError appends a recovery hint to errors that indicate the
// browser/CDP connection was lost, so the AI client resets and re-launches
// instead of retrying the same doomed call (which is indistinguishable from a
// selector typo without this hint).
func annotateBrowserError(err error) error {
	if err == nil {
		return nil
	}

	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"eof", "use of closed", "websocket", "cdp connection", "context canceled", "connection reset", "wsarecv", "wsasend"} {
		if strings.Contains(msg, marker) {
			return fmt.Errorf("%w — the browser or page may have closed or crashed; call session_reset, then retry", err)
		}
	}

	return err
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
// If cfg.PluginManager is set, plugin-provided MCP tools are registered.
func NewServer(cfg ServerConfig, cancelOnIdle ...func()) *mcp.Server {
	server, _ := newServerWithState(cfg, cancelOnIdle...)

	return server
}

// newServerWithState builds the server and also returns the internal state, so
// callers (Serve) can tear the browser down when the transport ends.
func newServerWithState(cfg ServerConfig, cancelOnIdle ...func()) (*mcp.Server, *mcpState) {
	state := &mcpState{config: cfg, ariaStore: aria.NewStore(), hooks: newHookRegistry(), policy: urlpolicy.FromEnv()}

	if cfg.IdleTimeout > 0 {
		state.idle = idle.New(cfg.IdleTimeout, func() {
			if cfg.Logger != nil {
				cfg.Logger.Info("idle: releasing browser; server stays up", "timeout", cfg.IdleTimeout)
			}

			// Release the browser to reclaim Chrome memory, but NEVER cancel the
			// server context. A stdio MCP server's lifetime is owned by the
			// client (Claude Code); the next tool call re-launches the browser
			// lazily. Previously this called the Serve cancel, so the process
			// exited after the idle window and the client could not reach it
			// again for the rest of the session ("use once, then dead").
			state.reset()
		})
	}
	_ = cancelOnIdle // retained for API compatibility; idle no longer cancels the transport

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "scout", Version: "1.0.0"},
		&mcp.ServerOptions{Logger: logger},
	)

	registerBrowserTools(server, state)
	registerLocatorTools(server, state) // Playwright-style locators + web-first assertions
	registerCaptureTools(server, state)
	registerHijackTools(server, state)
	registerSessionTools(server, state)
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

	return server, state
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

	server, state := newServerWithState(cfg, cancel)

	// Release the browser when the transport ends (client disconnect / stdin
	// EOF). Without this, the lazily-launched headless Chrome and its session
	// dir are orphaned every time the client goes away. Also close any headed
	// inspection browsers opened via the "open" tool.
	defer state.reset()
	defer state.closeOpenBrowsers()

	return server.Run(ctx, &mcp.StdioTransport{})
}

