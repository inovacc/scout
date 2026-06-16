package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerHijackTools adds network interception tools.
func registerHijackTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "hijack_watch",
		Description: "Watch and capture network traffic for a duration",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url_pattern": {"type": "string", "description": "URL pattern filter (glob), default *"},
				"collect_for_ms": {"type": "integer", "description": "capture duration in milliseconds, default 5000"},
				"body": {"type": "boolean", "description": "capture response bodies, default true"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URLPattern   string `json:"url_pattern"`
			CollectForMS int    `json:"collect_for_ms"`
			Body         *bool  `json:"body"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		// Set defaults
		if args.URLPattern == "" {
			args.URLPattern = "*"
		}
		if args.CollectForMS <= 0 {
			args.CollectForMS = 5000
		}
		captureBody := true
		if args.Body != nil {
			captureBody = *args.Body
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var hijackOpts []scout.HijackOption
		if captureBody {
			hijackOpts = append(hijackOpts, scout.WithHijackBodyCapture())
		}
		if args.URLPattern != "*" {
			hijackOpts = append(hijackOpts, scout.WithHijackURLFilter(args.URLPattern))
		}

		hijacker, err := page.NewSessionHijacker(hijackOpts...)
		if err != nil {
			return errResult(err.Error())
		}
		defer hijacker.Stop()

		var events []any
		timeout := time.After(time.Duration(args.CollectForMS) * time.Millisecond)

	loop:
		for {
			select {
			case ev, ok := <-hijacker.Events():
				if !ok {
					break loop
				}
				events = append(events, ev)
			case <-timeout:
				break loop
			case <-ctx.Done():
				break loop
			}
		}

		return jsonResult(events)
	})
}
