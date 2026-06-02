package engine

import (
	"fmt"
	"time"
)

// AutoFreeConfig holds recycling configuration.
type AutoFreeConfig struct {
	Interval  time.Duration
	OnRecycle func() // optional callback before recycle
}

// startAutoFree starts a background goroutine that periodically recycles the browser.
func (b *Browser) startAutoFree(cfg AutoFreeConfig) {
	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-b.done:
				return
			case <-ticker.C:
				if cfg.OnRecycle != nil {
					cfg.OnRecycle()
				}

				if err := b.recycleBrowser(); err != nil {
					// Best-effort: recycle failure is non-fatal.
					_ = err
				}
			}
		}
	}()
}

// pageState captures the URL and cookies of a single page for restore after recycle.
type pageState struct {
	url     string
	cookies []Cookie
}

// recycleBrowser saves state, closes old browser, launches new one, restores state.
func (b *Browser) recycleBrowser() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.browser == nil {
		return fmt.Errorf("scout: recycle: browser is nil")
	}

	// Capture state from open pages.
	pages, err := b.browser.Pages()
	if err != nil {
		return fmt.Errorf("scout: recycle: list pages: %w", err)
	}

	var states []pageState

	for _, rp := range pages {
		info, err := rp.Info()
		if err != nil {
			continue
		}

		u := info.URL
		if u == "" || u == "about:blank" {
			continue
		}

		p := &Page{page: rp, browser: b}

		cookies, err := p.GetCookies(u)
		if err != nil {
			cookies = nil
		}

		states = append(states, pageState{url: u, cookies: cookies})
	}

	// Close old browser.
	_ = b.browser.Close()
	if b.launcher != nil {
		b.launcher.Kill()
		// Remove the old user-data dir before relaunch. Cleanup() blocks on
		// <-l.exit; bound it so a hung process exit cannot deadlock under b.mu.
		l := b.launcher
		boundedCleanup(l.Cleanup, 3*time.Second)
		b.launcher = nil
	}

	b.browser = nil

	// Re-launch with the same options.
	tmp, err := New(func(o *options) { *o = *b.opts })
	if err != nil {
		return fmt.Errorf("scout: recycle: relaunch: %w", err)
	}

	b.browser = tmp.browser
	b.launcher = tmp.launcher

	// Restore pages and cookies.
	for _, s := range states {
		p, err := b.NewPage(s.url)
		if err != nil {
			continue
		}

		if len(s.cookies) > 0 {
			_ = p.SetCookies(s.cookies...)
		}
	}

	b.recycles++

	return nil
}

// boundedCleanup runs fn in a goroutine and waits up to timeout for it to
// return. Reports true if fn completed in time, false if the timeout fired.
// Used by recycleBrowser so Launcher.Cleanup()'s blocking <-l.exit wait cannot
// deadlock the recycle loop while b.mu is held.
func boundedCleanup(fn func(), timeout time.Duration) bool {
	done := make(chan struct{})

	go func() {
		fn()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
