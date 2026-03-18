package plugin

import (
	"testing"
	"time"
)

func TestEventRateLimiter(t *testing.T) {
	rl := newEventRateLimiter(5)

	// Should allow 5 events.
	for i := range 5 {
		if !rl.allow() {
			t.Fatalf("expected allow at event %d", i)
		}
	}

	// 6th should be blocked.
	if rl.allow() {
		t.Error("expected rate limit at event 6")
	}

	// After a second, tokens refill.
	rl.lastFill = time.Now().Add(-2 * time.Second)

	if !rl.allow() {
		t.Error("expected allow after refill")
	}
}

func TestEventDispatcher(t *testing.T) {
	d := NewEventDispatcher()

	if d.Len() != 0 {
		t.Errorf("Len() = %d, want 0", d.Len())
	}

	event := &Event{
		Type:      EventNavigation,
		Timestamp: time.Now(),
		URL:       "https://example.com",
	}

	// No proxies — dispatch returns 0.
	if n := d.Dispatch(event); n != 0 {
		t.Errorf("Dispatch with no proxies = %d, want 0", n)
	}
}

func TestMiddlewareChain_Empty(t *testing.T) {
	chain := NewMiddlewareChain()

	if chain.Len() != 0 {
		t.Errorf("Len() = %d, want 0", chain.Len())
	}
}

func TestHasHook(t *testing.T) {
	hooks := []HookPoint{HookBeforeNavigate, HookAfterLoad}

	if !hasHook(hooks, HookBeforeNavigate) {
		t.Error("expected true for before_navigate")
	}

	if hasHook(hooks, HookOnError) {
		t.Error("expected false for on_error")
	}
}

func TestEventProxy_NotSubscribed(t *testing.T) {
	proxy := NewEventProxy(nil, []EventType{EventNavigation}, nil, 100)

	event := &Event{Type: EventConsoleLog}

	if proxy.Emit(event) {
		t.Error("expected false for unsubscribed event type")
	}

	if !proxy.IsSubscribed(EventNavigation) {
		t.Error("expected true for subscribed event type")
	}

	if proxy.IsSubscribed(EventConsoleLog) {
		t.Error("expected false for unsubscribed event type")
	}
}
