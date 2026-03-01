package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerContentTools adds markdown, table, and meta extraction tools.
func registerContentTools(server *mcp.Server, state *mcpState) {
	server.AddTool(&mcp.Tool{
		Name:        "markdown",
		Description: "Extract the current page content as Markdown",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"mainOnly":{"type":"boolean","description":"extract only main content (readability mode)"},"includeImages":{"type":"boolean","description":"include images in output"},"includeLinks":{"type":"boolean","description":"render links as markdown links"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			MainOnly      *bool `json:"mainOnly"`
			IncludeImages *bool `json:"includeImages"`
			IncludeLinks  *bool `json:"includeLinks"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.MarkdownOption
		if args.MainOnly != nil && *args.MainOnly {
			opts = append(opts, scout.WithMainContentOnly())
		}
		if args.IncludeImages != nil {
			opts = append(opts, scout.WithIncludeImages(*args.IncludeImages))
		}
		if args.IncludeLinks != nil {
			opts = append(opts, scout.WithIncludeLinks(*args.IncludeLinks))
		}

		md, err := page.Markdown(opts...)
		if err != nil {
			return errResult(err.Error())
		}

		return textResult(md)
	})

	server.AddTool(&mcp.Tool{
		Name:        "table",
		Description: "Extract table data from the current page as JSON (headers + rows)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector for the table element (default: table)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Selector string `json:"selector"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}
		if args.Selector == "" {
			args.Selector = "table"
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		table, err := page.ExtractTable(args.Selector)
		if err != nil {
			return errResult(err.Error())
		}

		return jsonResult(table)
	})

	server.AddTool(&mcp.Tool{
		Name:        "meta",
		Description: "Extract page metadata: title, description, canonical URL, Open Graph, and Twitter Card tags",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		meta, err := page.ExtractMeta()
		if err != nil {
			return errResult(err.Error())
		}

		return jsonResult(meta)
	})
}
