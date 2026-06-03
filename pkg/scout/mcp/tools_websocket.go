package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/internal/metrics"
	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerWebSocketTools adds WebSocket monitoring and interaction tools.
func registerWebSocketTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "ws_listen",
		Description: "Monitor WebSocket traffic on the current page. Captures sent and received messages for a specified duration.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"urlFilter":{"type":"string","description":"filter WebSocket connections by URL substring"},"duration":{"type":"integer","description":"capture duration in seconds (default 10, max 60)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.WSListenInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult(err.Error())
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		out, err := tools.WSListen(ctx, page, in)
		if err != nil {
			return errResult(err.Error())
		}

		metrics.Get().WebSocketOpsTotal.Add(1)

		return jsonResult(out.Messages)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "ws_send",
		Description: "Send a message to an active WebSocket connection on the current page via JavaScript evaluation.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"script":{"type":"string","description":"JavaScript expression that sends a WebSocket message (e.g. 'myWs.send(JSON.stringify({type:\"ping\"}))')"}},"required":["script"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.WSSendInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult(err.Error())
		}

		if in.Script == "" {
			return errResult("script is required")
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		out, err := tools.WSSend(ctx, page, in)
		if err != nil {
			return errResult(err.Error())
		}

		metrics.Get().WebSocketOpsTotal.Add(1)

		return textResult(fmt.Sprintf("Executed: %s", out.Result))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "ws_connections",
		Description: "List active WebSocket connections on the current page.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.WSConnectionsInput
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
				return errResult(err.Error())
			}
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		out, err := tools.WSConnections(ctx, page, in)
		if err != nil {
			return errResult(err.Error())
		}

		metrics.Get().WebSocketOpsTotal.Add(1)

		return textResult(out.Result)
	})
}
