package sdk

import "context"

// EventHandler handles browser event notifications from Scout.
type EventHandler interface {
	// OnEvent is called when a subscribed browser event occurs.
	OnEvent(ctx context.Context, event EventData)
}

// EventHandlerFunc adapts a function to EventHandler.
type EventHandlerFunc func(ctx context.Context, event EventData)

func (f EventHandlerFunc) OnEvent(ctx context.Context, event EventData) {
	f(ctx, event)
}

// EventData is a browser event received from Scout.
type EventData struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	URL       string         `json:"url,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// ActionRequest is a request from a plugin to perform an action in response to an event.
type ActionRequest struct {
	Action string         `json:"action"` // inject_js, screenshot, navigate
	Params map[string]any `json:"params,omitempty"`
}
