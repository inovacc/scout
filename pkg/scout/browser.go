package scout

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	"github.com/inovacc/scout/pkg/stealth"
)

// Browser wraps a rod browser instance with a simplified API.
type Browser struct {
	browser *rod.Browser
	opts    *options
}

// New creates and connects a new headless browser with the given options.
func New(opts ...Option) (*Browser, error) {
	o := defaults()
	for _, fn := range opts {
		fn(o)
	}

	l := launcher.New().Headless(o.headless)

	if o.execPath != "" {
		l = l.Bin(o.execPath)
	}

	if o.proxy != "" {
		l = l.Proxy(o.proxy)
	}

	if o.userDataDir != "" {
		l = l.UserDataDir(o.userDataDir)
	}

	if o.noSandbox {
		l = l.NoSandbox(true)
	}

	if len(o.env) > 0 {
		l = l.Env(o.env...)
	}

	if o.xvfb {
		l = l.XVFB(o.xvfbArgs...)
	}

	for name, values := range o.launchFlags {
		l = l.Set(flags.Flag(name), values...)
	}

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("scout: launch browser: %w", err)
	}

	b := rod.New().ControlURL(u)
	if o.slowMotion > 0 {
		b = b.SlowMotion(o.slowMotion)
	}

	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("scout: connect browser: %w", err)
	}

	if o.ignoreCerts {
		if err := b.IgnoreCertErrors(true); err != nil {
			_ = b.Close()
			return nil, fmt.Errorf("scout: ignore cert errors: %w", err)
		}
	}

	if o.incognito {
		ctx, err := b.Incognito()
		if err != nil {
			_ = b.Close()
			return nil, fmt.Errorf("scout: incognito mode: %w", err)
		}

		return &Browser{browser: ctx, opts: o}, nil
	}

	return &Browser{browser: b, opts: o}, nil
}

// NewPage creates a new browser tab and navigates to the given URL.
// If stealth mode is enabled, the page is created with anti-detection measures.
func (b *Browser) NewPage(url string) (*Page, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: browser is nil")
	}

	var (
		rodPage *rod.Page
		err     error
	)

	if b.opts.stealth {
		rodPage, err = stealth.Page(b.browser)
		if err != nil {
			return nil, fmt.Errorf("scout: create stealth page: %w", err)
		}

		if url != "" {
			if err := rodPage.Navigate(url); err != nil {
				return nil, fmt.Errorf("scout: navigate: %w", err)
			}
		}
	} else {
		rodPage, err = b.browser.Page(proto.TargetCreateTarget{URL: url})
		if err != nil {
			return nil, fmt.Errorf("scout: create page: %w", err)
		}
	}

	if b.opts.userAgent != "" {
		if err := rodPage.SetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: b.opts.userAgent,
		}); err != nil {
			return nil, fmt.Errorf("scout: set user agent: %w", err)
		}
	}

	if b.opts.timeout > 0 {
		rodPage = rodPage.Timeout(b.opts.timeout)
	}

	p := &Page{page: rodPage, browser: b}

	if b.opts.windowW > 0 && b.opts.windowH > 0 {
		if err := p.SetViewport(b.opts.windowW, b.opts.windowH); err != nil {
			return nil, fmt.Errorf("scout: set viewport: %w", err)
		}
	}

	if b.opts.windowState != "" && b.opts.windowState != WindowStateNormal {
		if err := p.setWindowState(b.opts.windowState); err != nil {
			return nil, fmt.Errorf("scout: set initial window state: %w", err)
		}
	}

	return p, nil
}

// Pages returns all open pages (tabs) in the browser.
func (b *Browser) Pages() ([]*Page, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: browser is nil")
	}

	rodPages, err := b.browser.Pages()
	if err != nil {
		return nil, fmt.Errorf("scout: list pages: %w", err)
	}

	pages := make([]*Page, len(rodPages))
	for i, rp := range rodPages {
		pages[i] = &Page{page: rp, browser: b}
	}

	return pages, nil
}

// Version returns the browser version string.
func (b *Browser) Version() (string, error) {
	if b == nil || b.browser == nil {
		return "", fmt.Errorf("scout: browser is nil")
	}

	v, err := b.browser.Version()
	if err != nil {
		return "", fmt.Errorf("scout: get version: %w", err)
	}

	return v.Product, nil
}

// Close shuts down the browser. It is nil-safe and idempotent.
func (b *Browser) Close() error {
	if b == nil || b.browser == nil {
		return nil
	}

	if err := b.browser.Close(); err != nil {
		return fmt.Errorf("scout: close browser: %w", err)
	}

	b.browser = nil

	return nil
}
