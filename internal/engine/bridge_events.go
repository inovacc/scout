package engine

// BridgeEvent represents an event streamed from a browser page via the bridge WebSocket.
type BridgeEvent struct {
	Type      string         `json:"type"`
	PageID    string         `json:"pageID"`
	Timestamp int64          `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// Standard bridge event types.
const (
	BridgeEventDOMMutation = "dom.mutation"
	BridgeEventUserClick   = "user.click"
	BridgeEventUserInput   = "user.input"
	BridgeEventNavigation  = "navigation"
	BridgeEventConsoleLog  = "console.log"
)

// Events returns a read-only channel that receives all bridge events from
// connected browser clients.
func (s *BridgeServer) Events() <-chan BridgeEvent {
	if s == nil {
		ch := make(chan BridgeEvent)
		close(ch)

		return ch
	}

	return s.events
}

// Subscribe registers a callback for events of the given type. If eventType is
// empty, the callback receives all events. Subscriptions are not removable;
// they live for the lifetime of the server.
func (s *BridgeServer) Subscribe(eventType string, fn func(BridgeEvent)) {
	if s == nil || fn == nil {
		return
	}

	s.subMu.Lock()
	defer s.subMu.Unlock()

	s.eventSubs = append(s.eventSubs, eventSub{
		eventType: eventType,
		fn:        fn,
	})
}
