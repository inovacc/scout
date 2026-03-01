// Package mcp exposes Scout browser automation as an MCP server.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerConfig holds configuration for the MCP server.
type ServerConfig struct {
	Headless bool
	Stealth  bool
	Logger   *slog.Logger
}

// mcpState holds the lazy-initialized browser and current page.
type mcpState struct {
	mu      sync.Mutex
	browser *scout.Browser
	page    *scout.Page
	config  ServerConfig
}

func (s *mcpState) ensureBrowser(ctx context.Context) (*scout.Browser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.browser != nil {
		return s.browser, nil
	}

	opts := []scout.Option{
		scout.WithHeadless(s.config.Headless),
		scout.WithNoSandbox(),
	}
	if s.config.Stealth {
		opts = append(opts, scout.WithStealth())
	}

	b, err := scout.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("scout-mcp: launch browser: %w", err)
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
	return p, nil
}

func (s *mcpState) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.browser != nil {
		_ = s.browser.Close()
	}
	s.browser = nil
	s.page = nil
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

// NewServer creates an MCP server with Scout tools and resources.
func NewServer(cfg ServerConfig) *mcp.Server {
	state := &mcpState{config: cfg}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "scout", Version: "1.0.0"},
		&mcp.ServerOptions{Logger: logger},
	)

	// --- Tools ---

	server.AddTool(&mcp.Tool{
		Name:        "navigate",
		Description: "Navigate the browser to a URL",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to navigate to"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ URL string `json:"url"` }
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if err := page.Navigate(args.URL); err != nil {
			return errResult(err.Error())
		}

		_ = page.WaitLoad()

		title, _ := page.Title()
		url, _ := page.URL()
		return textResult(fmt.Sprintf("Navigated to %s (%s)", url, title))
	})

	server.AddTool(&mcp.Tool{
		Name:        "click",
		Description: "Click an element by CSS selector",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector"}},"required":["selector"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ Selector string `json:"selector"` }
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		el, err := page.Element(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		if err := el.Click(); err != nil {
			return errResult(err.Error())
		}

		return textResult(fmt.Sprintf("Clicked %s", args.Selector))
	})

	server.AddTool(&mcp.Tool{
		Name:        "type",
		Description: "Type text into an element by CSS selector",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector"},"text":{"type":"string","description":"text to type"}},"required":["selector","text"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Selector string `json:"selector"`
			Text     string `json:"text"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		el, err := page.Element(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		if err := el.Input(args.Text); err != nil {
			return errResult(err.Error())
		}

		return textResult(fmt.Sprintf("Typed into %s", args.Selector))
	})

	server.AddTool(&mcp.Tool{
		Name:        "screenshot",
		Description: "Take a screenshot of the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"fullPage":{"type":"boolean","description":"capture full page"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ FullPage bool `json:"fullPage"` }
		_ = json.Unmarshal(req.Params.Arguments, &args)

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var data []byte
		if args.FullPage {
			data, err = page.FullScreenshot()
		} else {
			data, err = page.Screenshot()
		}
		if err != nil {
			return errResult(err.Error())
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{
				MIMEType: "image/png",
				Data:     data,
			}},
		}, nil
	})

	server.AddTool(&mcp.Tool{
		Name:        "snapshot",
		Description: "Get the accessibility tree of the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"interactableOnly":{"type":"boolean","description":"only include interactable elements"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ InteractableOnly bool `json:"interactableOnly"` }
		_ = json.Unmarshal(req.Params.Arguments, &args)

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.SnapshotOption
		if args.InteractableOnly {
			opts = append(opts, scout.WithSnapshotInteractableOnly())
		}

		snap, err := page.SnapshotWithOptions(opts...)
		if err != nil {
			return errResult(err.Error())
		}

		return textResult(snap)
	})

	server.AddTool(&mcp.Tool{
		Name:        "extract",
		Description: "Extract text from an element by CSS selector",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector"}},"required":["selector"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ Selector string `json:"selector"` }
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		el, err := page.Element(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		text, err := el.Text()
		if err != nil {
			return errResult(err.Error())
		}

		return textResult(text)
	})

	server.AddTool(&mcp.Tool{
		Name:        "eval",
		Description: "Evaluate JavaScript in the page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string","description":"JavaScript expression"}},"required":["expression"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ Expression string `json:"expression"` }
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		result, err := page.Eval(args.Expression)
		if err != nil {
			return errResult(err.Error())
		}

		return textResult(result.String())
	})

	server.AddTool(&mcp.Tool{
		Name:        "back",
		Description: "Navigate back in browser history",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if err := page.NavigateBack(); err != nil {
			return errResult(err.Error())
		}

		return textResult("Navigated back")
	})

	server.AddTool(&mcp.Tool{
		Name:        "forward",
		Description: "Navigate forward in browser history",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if err := page.NavigateForward(); err != nil {
			return errResult(err.Error())
		}

		return textResult("Navigated forward")
	})

	server.AddTool(&mcp.Tool{
		Name:        "wait",
		Description: "Wait for a page condition (load, selector)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector to wait for"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ Selector string `json:"selector"` }
		_ = json.Unmarshal(req.Params.Arguments, &args)

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if args.Selector != "" {
			if _, err := page.WaitSelector(args.Selector); err != nil {
				return errResult(err.Error())
			}
			return textResult(fmt.Sprintf("Found %s", args.Selector))
		}

		if err := page.WaitLoad(); err != nil {
			return errResult(err.Error())
		}

		return textResult("Page loaded")
	})

	// --- Resources ---

	server.AddResource(&mcp.Resource{
		URI:  "scout://page/markdown",
		Name: "Page Markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return nil, err
		}

		md, err := page.Markdown()
		if err != nil {
			return nil, err
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: req.Params.URI, Text: md}},
		}, nil
	})

	server.AddResource(&mcp.Resource{
		URI:  "scout://page/url",
		Name: "Page URL",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return nil, err
		}

		u, err := page.URL()
		if err != nil {
			return nil, err
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: req.Params.URI, Text: u}},
		}, nil
	})

	server.AddTool(&mcp.Tool{
		Name:        "search",
		Description: "Search the web using a search engine",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"search query"},"engine":{"type":"string","description":"search engine: google, bing, duckduckgo","default":"google"}},"required":["query"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Query  string `json:"query"`
			Engine string `json:"engine"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.SearchOption
		switch args.Engine {
		case "bing":
			opts = append(opts, scout.WithSearchEngine(scout.Bing))
		case "duckduckgo", "ddg":
			opts = append(opts, scout.WithSearchEngine(scout.DuckDuckGo))
		default:
			// google is the default
		}

		results, err := browser.Search(args.Query, opts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: search: %s", err))
		}

		data, err := json.Marshal(results)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: marshal results: %s", err))
		}

		return textResult(string(data))
	})

	server.AddTool(&mcp.Tool{
		Name:        "fetch",
		Description: "Fetch a URL and extract its content as markdown, html, text, or metadata",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to fetch"},"mode":{"type":"string","description":"extraction mode: markdown, html, text, links, meta, full","default":"full"},"mainOnly":{"type":"boolean","description":"extract main content only using readability scoring"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL      string `json:"url"`
			Mode     string `json:"mode"`
			MainOnly bool   `json:"mainOnly"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.WebFetchOption
		if args.Mode != "" {
			opts = append(opts, scout.WithFetchMode(args.Mode))
		}
		if args.MainOnly {
			opts = append(opts, scout.WithFetchMainContent())
		}

		result, err := browser.WebFetch(args.URL, opts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: fetch: %s", err))
		}

		data, err := json.Marshal(result)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: marshal result: %s", err))
		}

		return textResult(string(data))
	})

	server.AddTool(&mcp.Tool{
		Name:        "pdf",
		Description: "Generate a PDF of the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"landscape":{"type":"boolean","description":"landscape orientation"},"printBackground":{"type":"boolean","description":"print background graphics"},"scale":{"type":"number","description":"scale factor (0.1 to 2.0)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Landscape       bool    `json:"landscape"`
			PrintBackground bool    `json:"printBackground"`
			Scale           float64 `json:"scale"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &args)

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var data []byte
		if args.Landscape || args.PrintBackground || args.Scale > 0 {
			data, err = page.PDFWithOptions(scout.PDFOptions{
				Landscape:       args.Landscape,
				PrintBackground: args.PrintBackground,
				Scale:           args.Scale,
			})
		} else {
			data, err = page.PDF()
		}
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: pdf: %s", err))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{
				MIMEType: "application/pdf",
				Data:     data,
			}},
		}, nil
	})

	server.AddTool(&mcp.Tool{
		Name:        "session_list",
		Description: "List current session info (URL, title of current page)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.mu.Lock()
		hasPage := state.page != nil
		state.mu.Unlock()

		if !hasPage {
			return textResult(`{"status":"no active session"}`)
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		u, _ := page.URL()
		title, _ := page.Title()

		info := map[string]string{
			"status": "active",
			"url":    u,
			"title":  title,
		}
		data, _ := json.Marshal(info)
		return textResult(string(data))
	})

	server.AddTool(&mcp.Tool{
		Name:        "session_reset",
		Description: "Close the current browser and page, forcing re-initialization on next use",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.reset()
		return textResult("Session reset")
	})

	server.AddTool(&mcp.Tool{
		Name:        "open",
		Description: "Open a URL in a visible (headed) browser for manual inspection. The browser remains open for interactive analysis.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to open"},"devtools":{"type":"boolean","description":"open Chrome DevTools automatically"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL      string `json:"url"`
			DevTools bool   `json:"devtools"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		// Launch a separate headed browser for inspection.
		opts := []scout.Option{
			scout.WithHeadless(false),
			scout.WithNoSandbox(),
		}
		if state.config.Stealth {
			opts = append(opts, scout.WithStealth())
		}
		if args.DevTools {
			opts = append(opts, scout.WithDevTools())
		}

		b, err := scout.New(opts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: open: %s", err))
		}

		page, err := b.NewPage(args.URL)
		if err != nil {
			_ = b.Close()
			return errResult(fmt.Sprintf("scout-mcp: open: %s", err))
		}

		_ = page.WaitLoad()

		title, _ := page.Title()
		u, _ := page.URL()

		return textResult(fmt.Sprintf("Opened %s (%s) in headed browser. Close the browser window when done.", u, title))
	})

	// --- Diagnostic Tools ---
	registerDiagTools(server, state)
	registerContentTools(server, state)
	registerNetworkTools(server, state)

	// --- Resources ---

	server.AddResource(&mcp.Resource{
		URI:  "scout://page/title",
		Name: "Page Title",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return nil, err
		}

		title, err := page.Title()
		if err != nil {
			return nil, err
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: req.Params.URI, Text: title}},
		}, nil
	})

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
		} else {
			if len(b) > 0 && b[len(b)-1] != '_' {
				b = append(b, '_')
			}
		}
	}
	// Trim trailing underscore.
	if len(b) > 0 && b[len(b)-1] == '_' {
		b = b[:len(b)-1]
	}
	return string(b)
}

// Serve starts the MCP server on stdio. Blocks until context is cancelled.
func Serve(ctx context.Context, logger *slog.Logger, headless, stealth bool) error {
	cfg := ServerConfig{
		Headless: headless,
		Stealth:  stealth,
		Logger:   logger,
	}

	server := NewServer(cfg)
	return server.Run(ctx, &mcp.StdioTransport{})
}

// ServeSSE starts the MCP server with HTTP+SSE transport on the given address.
// Blocks until the context is cancelled.
func ServeSSE(ctx context.Context, logger *slog.Logger, addr string, headless, stealth bool) error {
	cfg := ServerConfig{
		Headless: headless,
		Stealth:  stealth,
		Logger:   logger,
	}

	handler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		return NewServer(cfg)
	}, nil)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
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
