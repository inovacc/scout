// Package browser provides standalone browser management for Chromium-based browsers.
//
// It supports detecting locally installed browsers (Chrome, Brave, Edge),
// downloading browsers to a local cache, and managing the browser cache lifecycle.
//
// This package is dependency-free from the rest of the scout ecosystem and can be
// used independently via:
//
//	go get github.com/inovacc/scout/pkg/browser
//
// Basic usage:
//
//	mgr := browser.NewManager()
//	path, err := mgr.Resolve("brave") // find or download Brave
//	browsers, err := mgr.List()        // list all detected + downloaded browsers
//	err = mgr.Clean()                  // remove downloaded browsers from cache
package browser
