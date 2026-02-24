// Package mcp exposes Scout browser automation as an MCP server.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
