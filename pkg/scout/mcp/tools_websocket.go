package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerWebSocketTools adds WebSocket monitoring and interaction tools.
func registerWebSocketTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "ws_listen",
		Description: "Monitor WebSocket traffic on the current page. Captures sent and received messages for a specified duration.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"urlFilter":{"type":"string","description":"filter WebSocket connections by URL substring"},"duration":{"type":"integer","description":"capture duration in seconds (default 10, max 60)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URLFilter string `json:"urlFilter"`
			Duration  int    `json:"duration"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.Duration <= 0 {
			args.Duration = 10
		}

		if args.Duration > 60 {
			args.Duration = 60
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var wsOpts []scout.WebSocketOption
		if args.URLFilter != "" {
			wsOpts = append(wsOpts, scout.WithWSURLFilter(args.URLFilter))
		}

		messages, stop, err := page.MonitorWebSockets(wsOpts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: ws_listen: %s", err))
		}

		defer stop()

		timer := time.NewTimer(time.Duration(args.Duration) * time.Second)
		defer timer.Stop()

		var captured []wsMessageResult

		for {
			select {
			case <-timer.C:
				return jsonResult(captured)
			case <-ctx.Done():
				return jsonResult(captured)
			case msg, ok := <-messages:
				if !ok {
					return jsonResult(captured)
				}

				captured = append(captured, wsMessageResult{
					Direction: msg.Direction,
					Data:      truncate(msg.Data, 2000),
					Timestamp: msg.Timestamp.Format(time.RFC3339Nano),
					Opcode:    msg.Opcode,
				})
			}
		}
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "ws_send",
		Description: "Send a message to an active WebSocket connection on the current page via JavaScript evaluation.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"script":{"type":"string","description":"JavaScript expression that sends a WebSocket message (e.g. 'myWs.send(JSON.stringify({type:\"ping\"}))')"}},"required":["script"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Script string `json:"script"`
		}

		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.Script == "" {
			return errResult("script is required")
		}

		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		result, err := page.Eval(args.Script)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: ws_send: %s", err))
		}

		return textResult(fmt.Sprintf("Executed: %s", result.String()))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "ws_connections",
		Description: "List active WebSocket connections on the current page.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		js := `(() => {
			const conns = window.__scoutWSConnections || [];
			return conns.map(c => ({
				url: c.url,
				readyState: c.readyState,
				protocol: c.protocol,
				state: ['CONNECTING','OPEN','CLOSING','CLOSED'][c.readyState] || 'UNKNOWN'
			}));
		})()`

		result, err := page.Eval(js)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: ws_connections: %s", err))
		}

		return textResult(result.String())
	})
}

// wsMessageResult is the JSON-serializable form of a captured WebSocket message.
type wsMessageResult struct {
	Direction string `json:"direction"`
	Data      string `json:"data"`
	Timestamp string `json:"timestamp"`
	Opcode    int    `json:"opcode"`
}

// truncate shortens s to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen] + "..."
}
