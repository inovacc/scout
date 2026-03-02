package scout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inovacc/scout/pkg/scout/rod"
	"github.com/inovacc/scout/pkg/scout/rod/lib/launcher"
	"github.com/inovacc/scout/pkg/scout/rod/lib/launcher/flags"
	"github.com/inovacc/scout/pkg/scout/rod/lib/proto"
	"github.com/inovacc/scout/pkg/scout/stealth"
)

// Browser wraps a rod browser instance with a simplified API.
// For standalone browser detection, download, and cache management without the
// full scout dependency, see the pkg/browser/ package.
type Browser struct {
	browser  *rod.Browser
	opts     *options
	launcher *launcher.Launcher // nil for remote CDP connections

	// closeOnce ensures Close() is idempotent and safe for concurrent use.
	closeOnce sync.Once
	closed    atomic.Bool

	// AutoFree fields for periodic browser recycling.
	mu       sync.Mutex
	done     chan struct{}
	recycles int

	// webmcpRegistry accumulates discovered WebMCP tools when auto-discover is enabled.
	webmcpRegistry *WebMCPRegistry

	// bridgeServer is the WebSocket server for bridge communication, if enabled.
	bridgeServer *BridgeServer

	// version is the browser product string, eagerly fetched at creation time.
	version string

	// vpn holds VPN connection state and proxy auth configuration.
	vpn *vpnState

	// vpnRot manages automatic VPN server rotation across NewPage calls.
	vpnRot *vpnRotator

	// fpRot manages automatic fingerprint rotation across NewPage calls.
	fpRot *fingerprintRotator

	// sessionID tracks this browser's session directory under ~/.scout/sessions/.
	sessionID string
}

// New creates and connects a new headless browser with the given options.
func New(opts ...Option) (*Browser, error) { //nolint:maintidx
	// Clean up orphaned browsers on startup.
	_, _ = CleanOrphans()

	// Periodic orphan watchdog is started after browser creation (below).

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

	// If a VPN provider is set, connect and derive proxy URL + auth.
	if o.vpnProvider != nil && o.proxy == "" {
		conn, err := o.vpnProvider.Connect(context.Background(), "")
		if err != nil {
			return nil, fmt.Errorf("scout: vpn: connect: %w", err)
		}

		if dp, ok := o.vpnProvider.(*DirectProxy); ok {
			o.proxy = dp.ProxyURL()
			if dp.username != "" && o.proxyAuth == nil {
				o.proxyAuth = &proxyAuthConfig{username: dp.username, password: dp.password}
			}
		} else {
			o.proxy = fmt.Sprintf("%s://%s:%d", conn.Protocol, conn.Server.Host, conn.Port)
		}
	}

	var (
		u string
		l *launcher.Launcher
	)

	switch {
	case o.electronCDP != "":
		// Connect to running Electron app via CDP.
		var resolveErr error

		u, resolveErr = lookupElectronCDP(o.electronCDP)
		if resolveErr != nil {
			return nil, fmt.Errorf("scout: resolve electron CDP: %w", resolveErr)
		}
	case o.electronApp != "":
		// Launch Electron app with CDP debugging.
		var err error

		u, l, err = launchElectron(o)
		if err != nil {
			return nil, err
		}
	case o.remoteCDP != "":
		// Remote CDP endpoint — skip launcher entirely.
		// If URL already contains a full path (e.g., /devtools/browser/UUID), use as-is.
		// Otherwise resolve via /json/version to get the full WebSocket URL.
		if strings.Contains(o.remoteCDP, "/devtools/") {
			u = o.remoteCDP
		} else {
			resolved, resolveErr := launcher.ResolveURL(o.remoteCDP)
			if resolveErr != nil {
				u = o.remoteCDP
			} else {
				u = resolved
			}
		}
	default:
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

	// Eagerly fetch and cache the browser version.
	var cachedVersion string
	if v, vErr := b.Version(); vErr == nil {
		cachedVersion = v.Product
	}

	// Set up proxy authentication handler if configured.
	if o.proxyAuth != nil {
		wait := b.HandleAuth(o.proxyAuth.username, o.proxyAuth.password)

		go func() { _ = wait() }()
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

		br := &Browser{browser: ctx, opts: o, launcher: l, done: make(chan struct{}), version: cachedVersion}
		if o.webmcpAutoDiscover {
			br.webmcpRegistry = NewWebMCPRegistry()
		}

		if o.bridge && o.bridgePort != 0 {
			if err := br.startBridgeServer(); err != nil {
				_ = ctx.Close()
				return nil, err
			}
		}

		if o.autoFreeInterval > 0 {
			br.startAutoFree(AutoFreeConfig{
				Interval:  o.autoFreeInterval,
				OnRecycle: o.autoFreeCallback,
			})
		}

		if o.vpnRotation != nil && o.vpnProvider != nil {
			br.vpnRot = newVPNRotator(o.vpnProvider, *o.vpnRotation)
		}

		if o.fpRotation != nil {
			br.fpRot = newFingerprintRotator(*o.fpRotation)
		}

		// Register session in tracker for local launches.
		if l != nil {
			br.registerSession()
		}

		// Periodic orphan watchdog — kills dangling browsers whose scout died.
		StartOrphanWatchdog(DefaultOrphanCheckInterval, br.done)

		return br, nil
	}

	br := &Browser{browser: b, opts: o, launcher: l, done: make(chan struct{}), version: cachedVersion}
	if o.webmcpAutoDiscover {
		br.webmcpRegistry = NewWebMCPRegistry()
	}

	if o.bridge && o.bridgePort != 0 {
		if err := br.startBridgeServer(); err != nil {
			_ = b.Close()
			return nil, err
		}
	}

	if o.autoFreeInterval > 0 {
		br.startAutoFree(AutoFreeConfig{
			Interval:  o.autoFreeInterval,
			OnRecycle: o.autoFreeCallback,
		})
	}

	if o.vpnRotation != nil && o.vpnProvider != nil {
		br.vpnRot = newVPNRotator(o.vpnProvider, *o.vpnRotation)
	}

	if o.fpRotation != nil {
		br.fpRot = newFingerprintRotator(*o.fpRotation)
	}

	// Register session in tracker for local launches.
	if l != nil {
		br.registerSession()
	}

	// Periodic orphan watchdog — kills dangling browsers whose scout died.
	StartOrphanWatchdog(DefaultOrphanCheckInterval, br.done)

	return br, nil
}

// launchLocal starts a local browser process and returns the CDP WebSocket URL and launcher.
func launchLocal(o *options) (string, *launcher.Launcher, error) {
	// Session reuse: resolve data dir from explicit session ID, domain hash, or auto-find.
	if o.userDataDir == "" {
		if o.sessionID != "" {
			// Explicit session ID — data dir is SessionsDir/<id>.
			o.userDataDir = SessionDir(o.sessionID)
		} else if o.reusableSession {
			// Auto-find a matching reusable session.
			browserName := string(o.browserType)
			if browserName == "" {
				browserName = "chrome"
			}

			if found := FindReusableSession(browserName, o.headless); found != nil {
				o.userDataDir = found.Dir
				o.sessionID = found.ID
			}
		}
	}

	// Always use a deterministic hash dir — never let launcher generate UUID.
	if o.userDataDir == "" {
		browserName := string(o.browserType)
		if browserName == "" {
			browserName = "chrome"
		}

		hash := SessionHash(o.targetURL, browserName)
		o.userDataDir = SessionDir(hash)
		o.sessionID = hash
		// If scout.pid already exists, this is a reuse.
		if _, err := ReadSessionInfo(hash); err == nil {
			o.reusableSession = true
		}
	}

	l := launcher.New().HeadlessNew(o.headless)

	switch {
	case o.execPath != "":
		l = l.Bin(o.execPath)
	case o.autoDetect && o.browserType == "":
		if path, _, err := bestDetectedBrowser(); err == nil && path != "" {
			l = l.Bin(path)
		}
		// If detection fails, fall through to rod auto-detect.
	case o.browserType != "" && o.browserType != BrowserChrome:
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

	// Apply TLS profile flags.
	switch o.tlsProfile {
	case "randomized":
		l = l.Set(flags.Flag("disable-http2"))
	case "chrome", "":
		// Default Chrome TLS — no extra flags.
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

	// Only load bridge extension in headed mode or headless=new.
	// Old --headless does not support extensions.
	if o.bridge && !o.headless {
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
func (b *Browser) NewPage(url string) (*Page, error) { //nolint:maintidx
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: browser is nil")
	}

	// VPN rotation: if due, connect to next server and recycle the browser
	// so the new proxy launch flag takes effect.
	if b.vpnRot != nil {
		if conn, err := b.vpnRot.rotateIfNeeded(context.Background()); err != nil {
			return nil, fmt.Errorf("scout: vpn: rotate: %w", err)
		} else if conn != nil {
			// Update proxy option for the recycled browser.
			if dp, ok := b.vpnRot.provider.(*DirectProxy); ok {
				b.opts.proxy = dp.ProxyURL()
				if dp.username != "" {
					b.opts.proxyAuth = &proxyAuthConfig{username: dp.username, password: dp.password}
				}
			} else {
				b.opts.proxy = fmt.Sprintf("%s://%s:%d", conn.Protocol, conn.Server.Host, conn.Port)
			}

			if err := b.recycleBrowser(); err != nil {
				return nil, fmt.Errorf("scout: vpn: rotate: recycle: %w", err)
			}
		}
	}

	var (
		rodPage *rod.Page
		err     error
	)

	// Resolve fingerprint: rotation takes precedence over static.
	if b.fpRot != nil {
		domain := domainFromURL(url)
		b.opts.fingerprint = b.fpRot.forPage(domain)
	}

	hasInject := len(b.opts.injectScripts) > 0
	hasFP := b.opts.fingerprint != nil

	switch {
	case b.opts.stealth:
		rodPage, err = stealth.Page(b.browser)
		if err != nil {
			return nil, fmt.Errorf("scout: create stealth page: %w", err)
		}

		if hasFP {
			if _, err := rodPage.EvalOnNewDocument(b.opts.fingerprint.ToJS()); err != nil {
				return nil, fmt.Errorf("scout: inject fingerprint: %w", err)
			}
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
	case hasInject || hasFP:
		// Create page blank, inject scripts, then navigate so scripts run before page JS.
		rodPage, err = b.browser.Page(proto.TargetCreateTarget{})
		if err != nil {
			return nil, fmt.Errorf("scout: create page: %w", err)
		}

		if hasFP {
			if _, err := rodPage.EvalOnNewDocument(b.opts.fingerprint.ToJS()); err != nil {
				return nil, fmt.Errorf("scout: inject fingerprint: %w", err)
			}
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
	default:
		rodPage, err = b.browser.Page(proto.TargetCreateTarget{URL: url})
		if err != nil {
			return nil, fmt.Errorf("scout: create page: %w", err)
		}
	}

	if b.opts.userAgent == "" && hasFP {
		b.opts.userAgent = b.opts.fingerprint.UserAgent
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

	if b.opts.autoBypass != nil && url != "" {
		_ = b.opts.autoBypass.SolveAll(p)
	}

	// Auto-attach session hijacker if enabled.
	if b.opts.hijack {
		var hijackOpts []HijackOption

		if b.opts.hijackFilter != nil {
			if len(b.opts.hijackFilter.URLPatterns) > 0 {
				hijackOpts = append(hijackOpts, WithHijackURLFilter(b.opts.hijackFilter.URLPatterns...))
			}

			if b.opts.hijackFilter.CaptureBody {
				hijackOpts = append(hijackOpts, WithHijackBodyCapture())
			}
		}

		hijacker, hijackErr := p.NewSessionHijacker(hijackOpts...)
		if hijackErr != nil {
			return nil, fmt.Errorf("scout: auto-hijack: %w", hijackErr)
		}

		p.hijacker = hijacker
	}

	// Eagerly collect page environment info (browser version, UA, screen, etc.).
	if url != "" {
		_, _ = p.CollectInfo()
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

// Version returns the browser version string (cached from startup).
func (b *Browser) Version() (string, error) {
	if b == nil || b.browser == nil {
		return "", fmt.Errorf("scout: browser is nil")
	}

	if b.version != "" {
		return b.version, nil
	}

	v, err := b.browser.Version()
	if err != nil {
		return "", fmt.Errorf("scout: get version: %w", err)
	}

	b.version = v.Product

	return b.version, nil
}

// Close shuts down the browser and kills any orphan child processes.
// It is nil-safe, idempotent, and safe for concurrent use.
func (b *Browser) Close() error {
	if b == nil {
		return nil
	}

	var closeErr error

	b.closeOnce.Do(func() {
		// 1. Signal the autofree goroutine to stop.
		select {
		case <-b.done:
		default:
			close(b.done)
		}

		// 2. Stop the bridge WebSocket server if running.
		if b.bridgeServer != nil {
			_ = b.bridgeServer.Stop()
			b.bridgeServer = nil
		}

		// 3. Stop VPN rotator and disconnect VPN auth handler.
		b.vpnRot = nil
		if b.vpn != nil {
			b.vpn.mu.Lock()
			if b.vpn.authCancel != nil {
				_ = b.vpn.authCancel()
				b.vpn.authCancel = nil
			}
			b.vpn.mu.Unlock()
		}

		// 4. Stop fingerprint rotator.
		b.fpRot = nil

		// 5. Close CDP connection.
		if b.browser != nil {
			closeErr = b.browser.Close()
			b.browser = nil
		}

		// 6. Update session info: clear PIDs or remove entirely.
		if b.sessionID != "" {
			if b.opts.reusableSession {
				// Preserve session dir, clear PIDs and update last_used.
				if info, err := ReadSessionInfo(b.sessionID); err == nil {
					info.ScoutPID = 0
					info.BrowserPID = 0
					info.LastUsed = time.Now()
					_ = WriteSessionInfo(b.sessionID, info)
				}
			} else {
				RemoveSessionInfo(b.sessionID)
			}
		}

		// 7. Kill process tree and clean up temp user-data-dir.
		if b.launcher != nil {
			b.launcher.Kill()

			if b.opts.reusableSession && b.sessionID != "" {
				// Do NOT call Cleanup() — it would delete the data dir.
			} else {
				// Non-reusable: clean up data dir.
				go b.launcher.Cleanup()
			}

			b.launcher = nil
		}

		b.closed.Store(true)
	})

	if closeErr != nil {
		return fmt.Errorf("scout: close browser: %w", closeErr)
	}

	return nil
}

// WebMCPRegistry returns the browser's WebMCP tool registry, or nil if
// auto-discover is not enabled (see WithWebMCPAutoDiscover).
func (b *Browser) WebMCPRegistry() *WebMCPRegistry {
	if b == nil {
		return nil
	}

	return b.webmcpRegistry
}

// BridgeServer returns the WebSocket bridge server, or nil if bridge port
// was not configured (see WithBridgePort).
func (b *Browser) BridgeServer() *BridgeServer {
	if b == nil {
		return nil
	}

	return b.bridgeServer
}

// registerSession writes a scout.pid file into this browser's session directory.
func (b *Browser) registerSession() {
	if b.launcher == nil {
		return
	}

	dataDir := b.launcher.Get(flags.UserDataDir)
	if dataDir == "" {
		return
	}

	sessionID := filepath.Base(dataDir)

	browserName := string(b.opts.browserType)
	if browserName == "" {
		browserName = "chrome"
	}

	// Check if reusing an existing session.
	if existing, err := ReadSessionInfo(sessionID); err == nil {
		existing.LastUsed = time.Now()
		existing.ScoutPID = os.Getpid()

		existing.BrowserPID = b.launcher.PID()
		if existing.DomainHash == "" && b.opts.targetURL != "" {
			existing.DomainHash = DomainHash(b.opts.targetURL)
			existing.Domain = RootDomain(b.opts.targetURL)
		}

		_ = WriteSessionInfo(sessionID, existing)
		b.sessionID = sessionID

		return
	}

	info := &SessionInfo{
		ScoutPID:   os.Getpid(),
		BrowserPID: b.launcher.PID(),
		Reusable:   b.opts.reusableSession,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		Headless:   b.opts.headless,
		Browser:    browserName,
		DomainHash: DomainHash(b.opts.targetURL),
		Domain:     RootDomain(b.opts.targetURL),
	}

	_ = WriteSessionInfo(sessionID, info)
	b.sessionID = sessionID
}

// SessionID returns the UUID v7 session identifier for this browser instance.
func (b *Browser) SessionID() string {
	if b == nil {
		return ""
	}

	return b.sessionID
}

// startBridgeServer initializes and starts the bridge WebSocket server.
func (b *Browser) startBridgeServer() error {
	addr := fmt.Sprintf("127.0.0.1:%d", b.opts.bridgePort)

	s := NewBridgeServer(addr)
	if err := s.Start(); err != nil {
		return fmt.Errorf("scout: bridge server: %w", err)
	}

	b.bridgeServer = s

	return nil
}
