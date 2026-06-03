package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// WSMessage is the JSON-serializable form of a captured WebSocket frame.
type WSMessage struct {
	Direction string `json:"direction"`
	Data      string `json:"data"`
	Timestamp string `json:"timestamp"`
	Opcode    int    `json:"opcode"`
}

// WSListenInput configures a bounded WebSocket capture on the caller's page.
type WSListenInput struct {
	URLFilter string `json:"urlFilter,omitempty" jsonschema:"filter WebSocket connections by URL substring"`
	Duration  int    `json:"duration,omitempty"  jsonschema:"capture duration in seconds (default 10, max 60)"`
}

// WSListenOutput holds the captured WebSocket frames.
type WSListenOutput struct {
	Messages []WSMessage `json:"messages"`
}

// WSListen monitors WebSocket traffic on the page for a bounded duration and
// returns the captured frames. It always returns (no error) when the duration
// or context elapses with zero frames.
func WSListen(ctx context.Context, p *scout.Page, in WSListenInput) (*WSListenOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: ws_listen: nil page")
	}

	duration := in.Duration
	if duration <= 0 {
		duration = 10
	}
	if duration > 60 {
		duration = 60
	}

	var wsOpts []scout.WebSocketOption
	if in.URLFilter != "" {
		wsOpts = append(wsOpts, scout.WithWSURLFilter(in.URLFilter))
	}

	messages, stop, err := p.MonitorWebSockets(wsOpts...)
	if err != nil {
		return nil, fmt.Errorf("tools: ws_listen: %w", err)
	}
	defer stop()

	timer := time.NewTimer(time.Duration(duration) * time.Second)
	defer timer.Stop()

	captured := make([]WSMessage, 0)

	for {
		select {
		case <-timer.C:
			return &WSListenOutput{Messages: captured}, nil
		case <-ctx.Done():
			return &WSListenOutput{Messages: captured}, nil
		case msg, ok := <-messages:
			if !ok {
				return &WSListenOutput{Messages: captured}, nil
			}

			captured = append(captured, WSMessage{
				Direction: msg.Direction,
				Data:      truncateWS(msg.Data, 2000),
				Timestamp: msg.Timestamp.Format(time.RFC3339Nano),
				Opcode:    msg.Opcode,
			})
		}
	}
}

// WSSendInput carries a JavaScript expression that sends a WebSocket message.
type WSSendInput struct {
	Script string `json:"script" jsonschema:"JavaScript expression that sends a WebSocket message (e.g. 'myWs.send(JSON.stringify({type:\"ping\"}))')"`
}

// WSSendOutput holds the stringified evaluation result.
type WSSendOutput struct {
	Result string `json:"result"`
}

// WSSend executes a JavaScript expression on the page (typically to push a
// frame onto an active WebSocket) and returns its stringified result.
func WSSend(_ context.Context, p *scout.Page, in WSSendInput) (*WSSendOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: ws_send: nil page")
	}

	if in.Script == "" {
		return nil, fmt.Errorf("tools: ws_send: script required")
	}

	result, err := p.Eval(in.Script)
	if err != nil {
		return nil, fmt.Errorf("tools: ws_send: %w", err)
	}

	return &WSSendOutput{Result: result.String()}, nil
}

// WSConnectionsInput takes no fields (reads the current page).
type WSConnectionsInput struct{}

// WSConnectionsOutput holds the stringified list of active WebSocket connections.
type WSConnectionsOutput struct {
	Result string `json:"result"`
}

// wsConnectionsJS reads the bridge-tracked WebSocket connection registry.
// Expressed as a function literal: rod's Eval invokes it via .apply(), so an
// IIFE would fail with "(...).apply is not a function".
const wsConnectionsJS = `() => {
	const conns = window.__scoutWSConnections || [];
	return conns.map(c => ({
		url: c.url,
		readyState: c.readyState,
		protocol: c.protocol,
		state: ['CONNECTING','OPEN','CLOSING','CLOSED'][c.readyState] || 'UNKNOWN'
	}));
}`

// WSConnections lists active WebSocket connections on the page.
func WSConnections(_ context.Context, p *scout.Page, _ WSConnectionsInput) (*WSConnectionsOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: ws_connections: nil page")
	}

	result, err := p.Eval(wsConnectionsJS)
	if err != nil {
		return nil, fmt.Errorf("tools: ws_connections: %w", err)
	}

	return &WSConnectionsOutput{Result: result.String()}, nil
}

// truncateWS shortens s to maxLen, appending "..." if truncated.
// The returned string never exceeds maxLen characters.
func truncateWS(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return s[:maxLen]
	}

	return s[:maxLen-3] + "..."
}
