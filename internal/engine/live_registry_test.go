package engine

import (
	"testing"
	"time"
)

// liveCount counts entries currently in liveBrowsers (test helper).
func liveCount() int {
	n := 0
	liveBrowsers.Range(func(_, _ any) bool {
		n++
		return true
	})

	return n
}

func TestLiveRegistryRegisterUnregister(t *testing.T) {
	// Snapshot + restore the global map so the test is hermetic.
	var saved []any
	liveBrowsers.Range(func(k, _ any) bool {
		saved = append(saved, k)
		liveBrowsers.Delete(k)
		return true
	})
	t.Cleanup(func() {
		liveBrowsers.Range(func(k, _ any) bool {
			liveBrowsers.Delete(k)
			return true
		})
		for _, k := range saved {
			liveBrowsers.Store(k, struct{}{})
		}
	})

	// Two sentinel Browser pointers — no real browser needed.
	b1 := &Browser{}
	b2 := &Browser{}

	// Initially empty.
	if got := LiveBrowserCount(); got != 0 {
		t.Fatalf("expected 0 live browsers, got %d", got)
	}

	// Register both.
	b1.register()
	b2.register()

	if got := LiveBrowserCount(); got != 2 {
		t.Fatalf("expected 2 live browsers after register, got %d", got)
	}

	// Unregister one.
	b1.unregister()

	if got := LiveBrowserCount(); got != 1 {
		t.Fatalf("expected 1 live browser after one unregister, got %d", got)
	}

	// Idempotent unregister — second call must be a no-op.
	b1.unregister()

	if got := LiveBrowserCount(); got != 1 {
		t.Fatalf("expected 1 live browser after idempotent unregister, got %d", got)
	}

	// Unregister the second.
	b2.unregister()

	if got := LiveBrowserCount(); got != 0 {
		t.Fatalf("expected 0 live browsers after all unregistered, got %d", got)
	}
}

func TestCloseAllLive(t *testing.T) {
	// Hermetic: clear the registry and restore on cleanup.
	var saved []any
	liveBrowsers.Range(func(k, _ any) bool {
		saved = append(saved, k)
		liveBrowsers.Delete(k)
		return true
	})
	t.Cleanup(func() {
		liveBrowsers.Range(func(k, _ any) bool {
			liveBrowsers.Delete(k)
			return true
		})
		for _, k := range saved {
			liveBrowsers.Store(k, struct{}{})
		}
	})

	// Three fake browsers. Close() on a Browser with nil launcher/browser and
	// empty sessionID returns nil immediately (see Close() nil-safety).
	for range 3 {
		b := &Browser{done: make(chan struct{})}
		b.register()
	}

	if got := liveCount(); got != 3 {
		t.Fatalf("setup: liveCount = %d, want 3", got)
	}

	closed := CloseAllLive(2 * time.Second)
	if closed != 3 {
		t.Fatalf("CloseAllLive returned %d, want 3", closed)
	}

	if got := liveCount(); got != 0 {
		t.Fatalf("after CloseAllLive: liveCount = %d, want 0 (registry drained)", got)
	}
}
