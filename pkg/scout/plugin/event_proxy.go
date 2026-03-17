package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// EventType identifies a browser event category.
type EventType string

const (
	EventDOMMutation    EventType = "dom.mutation"
	EventNavigation     EventType = "navigation"
	EventConsoleLog     EventType = "console.log"
	EventNetworkRequest EventType = "network.request"
	EventNetworkResponse EventType = "network.response"
	EventWSReceived     EventType = "ws.received"
)

// Event is a browser event forwarded to plugins.
type Event struct {
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	URL       string         `json:"url,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// EventProxy forwards browser events to a plugin subprocess as JSON-RPC notifications.
type EventProxy struct {
	client      *Client
	subscribed  map[EventType]bool
	actions     map[string]bool
	rateLimiter *eventRateLimiter
}

// NewEventProxy creates an EventProxy for the given plugin client.
func NewEventProxy(client *Client, subscribe []EventType, actions []string, rateLimit int) *EventProxy {
	subs := make(map[EventType]bool, len(subscribe))
	for _, s := range subscribe {
		subs[s] = true
	}

	acts := make(map[string]bool, len(actions))
	for _, a := range actions {
		acts[a] = true
	}

	if rateLimit <= 0 {
		rateLimit = 100 // default 100 events/sec
	}

	return &EventProxy{
		client:      client,
		subscribed:  subs,
		actions:     acts,
		rateLimiter: newEventRateLimiter(rateLimit),
	}
}

// IsSubscribed returns true if the proxy is subscribed to the given event type.
func (p *EventProxy) IsSubscribed(et EventType) bool {
	return p.subscribed[et]
}

// Emit sends an event to the plugin as a JSON-RPC notification.
// Returns false if rate limited or the event type is not subscribed.
func (p *EventProxy) Emit(event *Event) bool {
	if !p.subscribed[event.Type] {
		return false
	}

	if !p.rateLimiter.allow() {
		return false
	}

	_ = p.client.Notify("event/emit", event)

	return true
}

// RequestAction sends an action request to the plugin and waits for a response.
// Only pre-declared actions in the manifest are allowed.
func (p *EventProxy) RequestAction(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	if !p.actions[action] {
		return nil, fmt.Errorf("plugin event: action %q not allowed by manifest", action)
	}

	raw, err := p.client.Call(ctx, "event/action", map[string]any{
		"action": action,
		"params": params,
	})
	if err != nil {
		return nil, fmt.Errorf("plugin event: action %q: %w", action, err)
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("plugin event: action unmarshal: %w", err)
	}

	return result, nil
}

// EventDispatcher fans out events to multiple EventProxy instances.
type EventDispatcher struct {
	mu      sync.RWMutex
	proxies []*EventProxy
}

// NewEventDispatcher creates a new event dispatcher.
func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{}
}

// Register adds an event proxy to the dispatcher.
func (d *EventDispatcher) Register(proxy *EventProxy) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.proxies = append(d.proxies, proxy)
}

// Dispatch sends an event to all subscribed proxies.
// Returns the number of proxies that received the event.
func (d *EventDispatcher) Dispatch(event *Event) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	count := 0

	for _, p := range d.proxies {
		if p.Emit(event) {
			count++
		}
	}

	return count
}

// Len returns the number of registered proxies.
func (d *EventDispatcher) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return len(d.proxies)
}

// eventRateLimiter is a simple token bucket rate limiter for events.
type eventRateLimiter struct {
	mu       sync.Mutex
	tokens   int
	maxRate  int
	lastFill time.Time
}

func newEventRateLimiter(maxPerSecond int) *eventRateLimiter {
	return &eventRateLimiter{
		tokens:   maxPerSecond,
		maxRate:  maxPerSecond,
		lastFill: time.Now(),
	}
}

func (r *eventRateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastFill)

	if elapsed >= time.Second {
		r.tokens = r.maxRate
		r.lastFill = now
	}

	if r.tokens <= 0 {
		return false
	}

	r.tokens--

	return true
}
