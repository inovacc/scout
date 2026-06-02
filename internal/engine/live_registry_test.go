package engine

import (
	"testing"
)

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
