package engine

import (
	"sync"
	"time"
)

// liveBrowsers is a process-wide registry of all live *Browser instances.
// It is populated by register() on successful launch and drained by
// unregister() inside closeOnce.Do — keeping it in sync with reality without
// any additional locking (sync.Map handles concurrent access internally).
var liveBrowsers sync.Map //nolint:gochecknoglobals // intentional process-wide registry

// register adds b to the live-browser registry.
// Called on every successful return from New.
func (b *Browser) register() {
	liveBrowsers.Store(b, struct{}{})
}

// unregister removes b from the live-browser registry.
// Called from inside closeOnce.Do — safe to call multiple times (no-op after first).
func (b *Browser) unregister() {
	liveBrowsers.Delete(b)
}

// LiveBrowserCount returns the number of currently registered (live) Browser
// instances. Primarily for testing; also available to Phase 4 diagnostics.
func LiveBrowserCount() int {
	n := 0
	liveBrowsers.Range(func(_, _ any) bool {
		n++
		return true
	})

	return n
}

// CloseAllLive closes every browser currently in the live registry, bounding
// each individual Close() with timeout. It is the best-effort teardown path
// invoked by the main() signal handler on SIGINT/SIGTERM.
//
// Each browser is closed in its own goroutine; a per-browser select waits for
// either Close() to return or the timeout to elapse, so one hung browser can
// never block teardown of the others. Returns the number of browsers whose
// Close() completed (returned, error or nil) before its deadline.
//
// Close() calls unregister() internally, so the map is drained for every
// browser that finishes in time. Browsers that time out are left registered
// (their state is unknown); next-startup reaping handles their session dirs.
func CloseAllLive(timeout time.Duration) int {
	var targets []*Browser

	liveBrowsers.Range(func(k, _ any) bool {
		if b, ok := k.(*Browser); ok && b != nil {
			targets = append(targets, b)
		}

		return true
	})

	if len(targets) == 0 {
		return 0
	}

	type result struct {
		ok bool
	}

	results := make(chan result, len(targets))

	for _, b := range targets {
		go func(b *Browser) {
			done := make(chan struct{})

			go func() {
				_ = b.Close()
				close(done)
			}()

			select {
			case <-done:
				results <- result{ok: true}
			case <-time.After(timeout):
				results <- result{ok: false}
			}
		}(b)
	}

	closed := 0

	for range targets {
		if r := <-results; r.ok {
			closed++
		}
	}

	return closed
}
