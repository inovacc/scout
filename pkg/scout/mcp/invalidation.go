package mcp

import (
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/pkg/scout"
)

// hookRegistry tracks which pageIDs have already had invalidation hooks installed,
// so we never double-install goroutines on a single page.
type hookRegistry struct {
	mu      sync.Mutex
	installed map[string]struct{}
}

func newHookRegistry() *hookRegistry {
	return &hookRegistry{installed: make(map[string]struct{})}
}

// tryInstall returns true if pageID was not yet registered (caller should install).
// Returns false if already installed (caller should skip).
func (r *hookRegistry) tryInstall(pageID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.installed[pageID]; ok {
		return false
	}
	r.installed[pageID] = struct{}{}
	return true
}

// installInvalidationHooks subscribes to CDP events and clears the aria store entry
// for the page on root-frame navigation or target destruction.
//
// It is safe to call multiple times for the same page — hooks are only installed once
// (guarded by state.hooks). Prefer calling at first-snapshot time in tools_aria.go.
func installInvalidationHooks(page *scout.Page, state *mcpState) {
	pageID := string(page.RodPage().TargetID)

	if !state.hooks.tryInstall(pageID) {
		return // already installed
	}

	rodPage := page.RodPage()

	// Root-frame navigation: ParentID is empty for the top-level frame.
	go rodPage.EachEvent(func(e *proto.PageFrameNavigated) {
		if e.Frame.ParentID == "" {
			state.ariaStore.Clear(pageID)
		}
	})()

	// Target destroyed: page tab closed or browser killed.
	go rodPage.EachEvent(func(e *proto.TargetTargetDestroyed) bool {
		if string(e.TargetID) == pageID {
			state.ariaStore.Clear(pageID)
			return true // stop listening
		}
		return false
	})()

	installMajorMutationHook(page, state, pageID)
}

// installMajorMutationHook subscribes to the bridge's mutation stream and
// clears the aria store entry for the page whenever ≥ majorThreshold mutations
// land within majorWindow. Phase A heuristic: 20 mutations / 100ms.
//
// Returns without doing anything if the bridge is not available on this page
// (e.g. headless contexts without the bridge extension).
func installMajorMutationHook(page *scout.Page, state *mcpState, pageID string) {
	bridge, err := page.Bridge()
	if err != nil || bridge == nil {
		return
	}

	const (
		majorThreshold = 20
		majorWindow    = 100 * time.Millisecond
	)

	var (
		mu     sync.Mutex
		window []time.Time // timestamps of recent mutation batches
	)

	bridge.OnMutation(func(events []scout.MutationEvent) {
		now := time.Now()
		mu.Lock()
		// drop timestamps older than majorWindow
		cutoff := now.Add(-majorWindow)
		i := 0
		for ; i < len(window); i++ {
			if window[i].After(cutoff) {
				break
			}
		}
		window = window[i:]

		// each MutationEvent represents one CDP-side mutation;
		// all events in the batch arrived together so timestamp all at now
		for range events {
			window = append(window, now)
		}

		if len(window) >= majorThreshold {
			window = window[:0] // reset to avoid re-firing on next event
			mu.Unlock()
			state.ariaStore.Clear(pageID)
			return
		}
		mu.Unlock()
	})
}
