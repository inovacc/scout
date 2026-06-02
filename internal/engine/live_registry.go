package engine

import "sync"

// liveBrowsers is a process-wide registry of all live *Browser instances.
// It is populated by register() on successful launch and drained by
// unregister() inside closeOnce.Do — keeping it in sync with reality without
// any additional locking (sync.Map handles concurrent access internally).
//
// Task 3.2 will add CloseAllLive() that iterates this map during signal-handler
// teardown. Do NOT add it here.
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
