package mcp

import (
	"context"

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
}
