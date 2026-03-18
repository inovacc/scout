package plugin

import (
	"context"
	"testing"
)

func TestMiddlewareChain_Register(t *testing.T) {
	chain := NewMiddlewareChain()

	// Register with priorities out of order.
	chain.Register(NewMiddlewareProxy(nil, []HookPoint{HookAfterLoad}, 50))
	chain.Register(NewMiddlewareProxy(nil, []HookPoint{HookBeforeNavigate}, 10))
	chain.Register(NewMiddlewareProxy(nil, []HookPoint{HookOnError}, 90))

	if chain.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", chain.Len())
	}

	// Verify sorted by priority.
	if chain.middlewares[0].Priority() != 10 {
		t.Errorf("first priority = %d, want 10", chain.middlewares[0].Priority())
	}

	if chain.middlewares[2].Priority() != 90 {
		t.Errorf("last priority = %d, want 90", chain.middlewares[2].Priority())
	}
}

func TestMiddlewareChain_Execute_NoMiddlewares(t *testing.T) {
	chain := NewMiddlewareChain()

	hookCtx := &HookContext{
		Hook: HookBeforeNavigate,
		URL:  "https://example.com",
	}

	result, err := chain.Execute(context.Background(), hookCtx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.Action != ActionAllow {
		t.Errorf("Action = %q, want allow", result.Action)
	}
}

func TestMiddlewareProxy_Hooks(t *testing.T) {
	hooks := []HookPoint{HookBeforeNavigate, HookAfterLoad}
	proxy := NewMiddlewareProxy(nil, hooks, 25)

	if proxy.Priority() != 25 {
		t.Errorf("Priority() = %d, want 25", proxy.Priority())
	}

	got := proxy.Hooks()
	if len(got) != 2 {
		t.Errorf("Hooks() len = %d, want 2", len(got))
	}
}
