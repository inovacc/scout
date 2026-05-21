package mcp

import (
	"sync"

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
}
