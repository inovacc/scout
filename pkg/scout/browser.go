package scout

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/inovacc/scout/pkg/rod"
	"github.com/inovacc/scout/pkg/rod/lib/launcher"
	"github.com/inovacc/scout/pkg/rod/lib/launcher/flags"
	"github.com/inovacc/scout/pkg/rod/lib/proto"
	"github.com/inovacc/scout/pkg/stealth"
)

// Browser wraps a rod browser instance with a simplified API.
type Browser struct {
	browser  *rod.Browser
	opts     *options
	launcher *launcher.Launcher // nil for remote CDP connections
}

// New creates and connects a new headless browser with the given options.
func New(opts ...Option) (*Browser, error) {
	o := defaults()
	for _, fn := range opts {
		fn(o)
	}

	if o.injectErr != nil {
		return nil, o.injectErr
	}

	// Default to maximized window in headed mode unless explicitly set.
	if !o.headless && o.windowState == "" {
		o.windowState = WindowStateMaximized
	}

	var (
		u string
		l *launcher.Launcher
	)

	if o.remoteCDP != "" {
		// Remote CDP endpoint — skip launcher entirely.
		u = o.remoteCDP
	} else {
		var err error
		u, l, err = launchLocal(o)
		if err != nil {
			return nil, err
		}
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

		return &Browser{browser: ctx, opts: o, launcher: l}, nil
	}

	return &Browser{browser: b, opts: o, launcher: l}, nil
}

// launchLocal starts a local browser process and returns the CDP WebSocket URL and launcher.
func launchLocal(o *options) (string, *launcher.Launcher, error) {
	l := launcher.New().Headless(o.headless)

	if o.execPath != "" {
		l = l.Bin(o.execPath)
	} else if o.browserType != "" && o.browserType != BrowserChrome {
		binPath, err := resolveBrowser(context.Background(), o.browserType)
		if err != nil {
			return "", nil, fmt.Errorf("scout: resolve %s browser: %w", o.browserType, err)
		}

		l = l.Bin(binPath)
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

	if o.stealth {
		l = l.Set(flags.Flag("disable-blink-features"), "AutomationControlled")
	}

	for name, values := range o.launchFlags {
		l = l.Set(flags.Flag(name), values...)
	}

	if o.devtools {
		l = l.Set(flags.Flag("auto-open-devtools-for-tabs"))
	}

	// In headed mode, set --window-size so the OS window matches the desired dimensions.
	// In headless mode, SetViewport (CDP emulation) handles this after connection.
	if !o.headless && o.windowW > 0 && o.windowH > 0 {
		l = l.Set(flags.WindowSize, strconv.Itoa(o.windowW)+","+strconv.Itoa(o.windowH))
	}

	for _, id := range o.extensionIDs {
		dir, err := extensionPathByID(id)
		if err != nil {
			return "", nil, err
		}
		o.extensions = append(o.extensions, dir)
	}

	if o.bridge {
		bridgeDir, err := writeBridgeExtension()
		if err != nil {
			return "", nil, err
		}

		o.extensions = append(o.extensions, bridgeDir)
	}

	if len(o.extensions) > 0 {
		joined := strings.Join(o.extensions, ",")
		l = l.Set(flags.Flag("load-extension"), joined)
		l = l.Set(flags.Flag("disable-extensions-except"), joined)
	}

	u, err := l.Launch()
	if err != nil {
		return "", nil, fmt.Errorf("scout: launch browser: %w", err)
	}

	return u, l, nil
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

	hasInject := len(b.opts.injectScripts) > 0

	if b.opts.stealth {
		rodPage, err = stealth.Page(b.browser)
		if err != nil {
			return nil, fmt.Errorf("scout: create stealth page: %w", err)
		}

		for _, script := range b.opts.injectScripts {
			if _, err := rodPage.EvalOnNewDocument(script); err != nil {
				return nil, fmt.Errorf("scout: inject script: %w", err)
			}
		}

		if url != "" {
			if err := rodPage.Navigate(url); err != nil {
				return nil, fmt.Errorf("scout: navigate: %w", err)
			}
		}
	} else if hasInject {
		// Create page blank, inject scripts, then navigate so scripts run before page JS.
		rodPage, err = b.browser.Page(proto.TargetCreateTarget{})
		if err != nil {
			return nil, fmt.Errorf("scout: create page: %w", err)
		}

		for _, script := range b.opts.injectScripts {
			if _, err := rodPage.EvalOnNewDocument(script); err != nil {
				return nil, fmt.Errorf("scout: inject script: %w", err)
			}
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
		override := &proto.NetworkSetUserAgentOverride{
			UserAgent: b.opts.userAgent,
		}
		if b.opts.userAgentMetadata != nil {
			override.UserAgentMetadata = b.opts.userAgentMetadata
		}
		if err := rodPage.SetUserAgent(override); err != nil {
			return nil, fmt.Errorf("scout: set user agent: %w", err)
		}
	}

	if b.opts.timeout > 0 {
		rodPage = rodPage.Timeout(b.opts.timeout)
	}

	p := &Page{page: rodPage, browser: b}

	if len(b.opts.blockPatterns) > 0 {
		if err := p.SetBlockedURLs(b.opts.blockPatterns...); err != nil {
			return nil, fmt.Errorf("scout: set block patterns: %w", err)
		}
	}

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

	if b.opts.smartWait && url != "" {
		_ = p.WaitFrameworkReady()
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

// Close shuts down the browser and kills any orphan child processes.
// It is nil-safe and idempotent.
func (b *Browser) Close() error {
	if b == nil || b.browser == nil {
		return nil
	}

	err := b.browser.Close()

	// Best-effort zombie cleanup: kill the process tree even if CDP close failed.
	if b.launcher != nil {
		b.launcher.Kill()
		b.launcher = nil
	}

	b.browser = nil

	if err != nil {
		return fmt.Errorf("scout: close browser: %w", err)
	}

	return nil
}
