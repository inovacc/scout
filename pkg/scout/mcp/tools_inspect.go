package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func storageKind(sessionStorage bool) string {
	if sessionStorage {
		return "sessionStorage"
	}
	return "localStorage"
}

func registerInspectTools(server *mcp.Server, state *mcpState) {
	server.AddTool(&mcp.Tool{
		Name:        "storage",
		Description: "Manage web storage (localStorage/sessionStorage): get, set, list, or clear",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"action":{"type":"string","enum":["get","set","list","clear"],"description":"action to perform"},"key":{"type":"string","description":"storage key (for get/set)"},"value":{"type":"string","description":"value to store (for set)"},"sessionStorage":{"type":"boolean","description":"use sessionStorage instead of localStorage"}},"required":["action"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Action         string `json:"action"`
			Key            string `json:"key"`
			Value          string `json:"value"`
			SessionStorage bool   `json:"sessionStorage"`
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
			if args.Key == "" {
				return errResult("key is required for get action")
			}
			var val string
			if args.SessionStorage {
				val, err = page.SessionStorageGet(args.Key)
			} else {
				val, err = page.LocalStorageGet(args.Key)
			}
			if err != nil {
				return errResult(err.Error())
			}
			return textResult(val)

		case "set":
			if args.Key == "" {
				return errResult("key is required for set action")
			}
			if args.SessionStorage {
				err = page.SessionStorageSet(args.Key, args.Value)
			} else {
				err = page.LocalStorageSet(args.Key, args.Value)
			}
			if err != nil {
				return errResult(err.Error())
			}
			return textResult(fmt.Sprintf("%s[%q] set", storageKind(args.SessionStorage), args.Key))

		case "list":
			var items map[string]string
			if args.SessionStorage {
				items, err = page.SessionStorageGetAll()
			} else {
				items, err = page.LocalStorageGetAll()
			}
			if err != nil {
				return errResult(err.Error())
			}
			return jsonResult(items)

		case "clear":
			if args.SessionStorage {
				err = page.SessionStorageClear()
			} else {
				err = page.LocalStorageClear()
			}
			if err != nil {
				return errResult(err.Error())
			}
			return textResult(fmt.Sprintf("%s cleared", storageKind(args.SessionStorage)))

		default:
			return errResult(fmt.Sprintf("unknown action %q (use get, set, list, or clear)", args.Action))
		}
	})

	server.AddTool(&mcp.Tool{
		Name:        "hijack",
		Description: "Capture network traffic (HTTP requests/responses and WebSocket frames) for a duration",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"urlFilter":{"type":"string","description":"URL pattern to filter (optional)"},"captureBody":{"type":"boolean","description":"capture request/response bodies"},"duration":{"type":"integer","description":"capture duration in seconds (default 10, max 30)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URLFilter   string `json:"urlFilter"`
			CaptureBody bool   `json:"captureBody"`
			Duration    int    `json:"duration"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &args)

		if args.Duration <= 0 {
			args.Duration = 10
		}
		if args.Duration > 30 {
			args.Duration = 30
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.HijackOption
		if args.CaptureBody {
			opts = append(opts, scout.WithHijackBodyCapture())
		}
		if args.URLFilter != "" {
			opts = append(opts, scout.WithHijackURLFilter(args.URLFilter))
		}

		hijacker, err := page.NewSessionHijacker(opts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: hijack: %s", err))
		}

		var events []scout.HijackEvent
		timeout := time.After(time.Duration(args.Duration) * time.Second)
		ch := hijacker.Events()

	collect:
		for len(events) < 100 {
			select {
			case ev, ok := <-ch:
				if !ok {
					break collect
				}
				events = append(events, ev)
			case <-timeout:
				break collect
			case <-ctx.Done():
				break collect
			}
		}

		hijacker.Stop()
		return jsonResult(events)
	})

	server.AddTool(&mcp.Tool{
		Name:        "har",
		Description: "Export network performance entries via the Performance API",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"action":{"type":"string","enum":["export"],"description":"action to perform"}},"required":["action"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.Action != "export" {
			return errResult(fmt.Sprintf("unknown action %q (use export)", args.Action))
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		result, err := page.Eval(`() => JSON.stringify(performance.getEntriesByType('resource'))`)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: har export: %s", err))
		}

		// Parse and re-marshal for pretty output.
		var entries []any
		if err := json.Unmarshal([]byte(result.String()), &entries); err != nil {
			// Return raw string if parse fails.
			return textResult(result.String())
		}
		return jsonResult(entries)
	})

	server.AddTool(&mcp.Tool{
		Name:        "swagger",
		Description: "Extract OpenAPI/Swagger specification from a URL",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to extract OpenAPI spec from"},"endpointsOnly":{"type":"boolean","description":"extract only endpoint paths"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL           string `json:"url"`
			EndpointsOnly bool   `json:"endpointsOnly"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.SwaggerOption
		if args.EndpointsOnly {
			opts = append(opts, scout.WithSwaggerEndpointsOnly(true))
		}

		spec, err := browser.ExtractSwagger(args.URL, opts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: swagger: %s", err))
		}

		return jsonResult(spec)
	})
}
