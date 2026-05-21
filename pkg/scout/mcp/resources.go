package mcp

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerResources adds MCP resources for page content.
func registerResources(server *mcp.Server, state *mcpState) {
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

	// scout://snapshot/{page-id} — current ARIA snapshot for a page, rendered as
	// YAML with [ref=eN] tags. Phase A: read-only; Phase B will add action tools
	// that consume these refs.
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "scout://snapshot/{pageId}",
		Name:        "Page accessibility snapshot",
		Description: "Current ARIA snapshot for a page, rendered as YAML with [ref=eN] tags.",
		MIMEType:    "text/yaml",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		pageID := strings.TrimPrefix(req.Params.URI, "scout://snapshot/")
		// strip ?v=... if present
		if i := strings.Index(pageID, "?"); i >= 0 {
			pageID = pageID[:i]
		}
		snap, ok := state.ariaStore.Get(pageID)
		if !ok {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		var buf bytes.Buffer
		if err := snap.RenderYAML(&buf); err != nil {
			return nil, fmt.Errorf("scout: mcp: render snapshot: %w", err)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "text/yaml",
				Text:     buf.String(),
			}},
		}, nil
	})
}
