package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerNetworkTools adds cookie, header, and block tools.
func registerNetworkTools(server *mcp.Server, state *mcpState) {
	server.AddTool(&mcp.Tool{
		Name:        "cookie",
		Description: "Manage browser cookies (get, set, or clear)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"action":{"type":"string","enum":["get","set","clear"],"description":"action to perform"},"name":{"type":"string","description":"cookie name (for set)"},"value":{"type":"string","description":"cookie value (for set)"},"domain":{"type":"string","description":"cookie domain (for set)"},"path":{"type":"string","description":"cookie path (for set, default /)"}},"required":["action"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Action string `json:"action"`
			Name   string `json:"name"`
			Value  string `json:"value"`
			Domain string `json:"domain"`
			Path   string `json:"path"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		switch args.Action {
		case "get":
			cookies, err := page.GetCookies()
			if err != nil {
				return errResult(err.Error())
			}

			return jsonResult(cookies)
		case "set":
			if args.Name == "" {
				return errResult("name is required for set action")
			}

			path := args.Path
			if path == "" {
				path = "/"
			}

			c := scout.Cookie{
				Name:   args.Name,
				Value:  args.Value,
				Domain: args.Domain,
				Path:   path,
			}
			if err := page.SetCookies(c); err != nil {
				return errResult(err.Error())
			}

			return textResult(fmt.Sprintf("cookie %q set", args.Name))
		case "clear":
			if err := page.ClearCookies(); err != nil {
				return errResult(err.Error())
			}

			return textResult("cookies cleared")
		default:
			return errResult(fmt.Sprintf("unknown action %q (use get, set, or clear)", args.Action))
		}
	})

	server.AddTool(&mcp.Tool{
		Name:        "header",
		Description: "Set custom HTTP headers for subsequent requests",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"headers":{"type":"object","additionalProperties":{"type":"string"},"description":"header name-value pairs"}},"required":["headers"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Headers map[string]string `json:"headers"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		_, err = page.SetHeaders(args.Headers)
		if err != nil {
			return errResult(err.Error())
		}

		return textResult(fmt.Sprintf("%d header(s) set", len(args.Headers)))
	})

	server.AddTool(&mcp.Tool{
		Name:        "block",
		Description: "Block URL patterns from loading (supports wildcards like *.css, *analytics*)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"patterns":{"type":"array","items":{"type":"string"},"description":"URL patterns to block"}},"required":["patterns"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Patterns []string `json:"patterns"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		if err := page.Block(args.Patterns...); err != nil {
			return errResult(err.Error())
		}

		return textResult(fmt.Sprintf("%d URL pattern(s) blocked", len(args.Patterns)))
	})
}
