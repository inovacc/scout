package scout

import (
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout/rod/lib/proto"
)

// BrowserType identifies a Chromium-based browser for auto-detection.
type BrowserType string

const (
	// BrowserChrome uses rod's default Chrome/Chromium auto-detection.
	BrowserChrome BrowserType = "chrome"
	// BrowserBrave selects Brave Browser.
	BrowserBrave BrowserType = "brave"
	// BrowserEdge selects Microsoft Edge.
	BrowserEdge BrowserType = "edge"
)

// Option configures a Browser instance.
type Option func(*options)

type options struct {
	browserType        BrowserType
	headless           bool
	stealth            bool
	userAgent          string
	proxy              string
	windowW            int
	windowH            int
	timeout            time.Duration
	slowMotion         time.Duration
	ignoreCerts        bool
	execPath           string
	userDataDir        string
	env                []string
	incognito          bool
	noSandbox          bool
	windowState        WindowState
	xvfb               bool
	xvfbArgs           []string
	launchFlags        map[string][]string
	extensions         []string
	extensionIDs       []string
	devtools           bool
	bridge             bool
	blockPatterns      []string
	remoteCDP          string
	userAgentMetadata  *proto.EmulationUserAgentMetadata
	smartWait          bool
	profile            *UserProfile
	injectScripts      []string
	injectErr          error
	autoDetect         bool
	autoFreeInterval   time.Duration
	autoFreeCallback   func()
	tlsProfile         string
	webmcpAutoDiscover bool
	bridgePort         int
	autoBypass         *ChallengeSolver
	fingerprint        *Fingerprint
	fpRotation         *FingerprintRotationConfig
	vpnProvider        VPNProvider
	vpnRotation        *VPNRotationConfig
	proxyAuth          *proxyAuthConfig
	proxyChain         *ProxyChain
	hijack             bool
	hijackFilter       *HijackFilter
}

func defaults() *options {
	headless := true
	if v := os.Getenv("SCOUT_HEADLESS"); v == "false" || v == "0" {
		headless = false
	}

	bridge := true
	if v := os.Getenv("SCOUT_BRIDGE"); v == "false" || v == "0" {
		bridge = false
	}

	stealthMode := false
	if v := os.Getenv("SCOUT_STEALTH"); v == "true" || v == "1" {
		stealthMode = true
	}

	return &options{
		headless: headless,
		stealth:  stealthMode,
		bridge:   bridge,
		windowW:  1920,
		windowH:  1080,
		timeout:  30 * time.Second,
	}
}

// WithBrowser selects which Chromium-based browser to use. Default: chrome (rod auto-detect).
// This is ignored if WithExecPath is also set.
func WithBrowser(bt BrowserType) Option {
	return func(o *options) { o.browserType = bt }
}

// WithHeadless sets whether the browser runs in headless mode. Default: true.
func WithHeadless(v bool) Option {
	return func(o *options) { o.headless = v }
}

// WithStealth enables stealth mode to avoid bot detection.
func WithStealth() Option {
	return func(o *options) { o.stealth = true }
}

// WithUserAgent sets a custom User-Agent string.
func WithUserAgent(ua string) Option {
	return func(o *options) { o.userAgent = ua }
}

// WithUserAgentMetadata sets User Agent Client Hints metadata (Sec-CH-UA-*).
// This controls what navigator.userAgentData returns in JavaScript.
func WithUserAgentMetadata(meta *proto.EmulationUserAgentMetadata) Option {
	return func(o *options) { o.userAgentMetadata = meta }
}

// WithProxy sets the proxy server URL (e.g. "socks5://127.0.0.1:1080").
func WithProxy(proxy string) Option {
	return func(o *options) { o.proxy = proxy }
}

// WithWindowSize sets the browser window dimensions. Default: 1920x1080.
func WithWindowSize(w, h int) Option {
	return func(o *options) { o.windowW = w; o.windowH = h }
}

// WithTimeout sets the default timeout for all operations. Default: 30s.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithSlowMotion adds a delay between actions for debugging.
func WithSlowMotion(d time.Duration) Option {
	return func(o *options) { o.slowMotion = d }
}

// WithIgnoreCerts disables TLS certificate verification.
func WithIgnoreCerts() Option {
	return func(o *options) { o.ignoreCerts = true }
}

// WithExecPath sets the path to the browser executable.
func WithExecPath(path string) Option {
	return func(o *options) { o.execPath = path }
}

// WithUserDataDir sets the browser user data directory for persistent sessions.
func WithUserDataDir(dir string) Option {
	return func(o *options) { o.userDataDir = dir }
}

// WithEnv sets additional environment variables for the browser process.
func WithEnv(env ...string) Option {
	return func(o *options) { o.env = append(o.env, env...) }
}

// WithIncognito opens the browser in incognito mode.
func WithIncognito() Option {
	return func(o *options) { o.incognito = true }
}

// WithNoSandbox disables the browser sandbox. Use only in containers.
func WithNoSandbox() Option {
	return func(o *options) { o.noSandbox = true }
}

// WithWindowState sets the initial window state for new pages.
func WithWindowState(state WindowState) Option {
	return func(o *options) { o.windowState = state }
}

// WithMaximized is a convenience shortcut for WithWindowState(WindowStateMaximized).
func WithMaximized() Option {
	return func(o *options) { o.windowState = WindowStateMaximized }
}

// WithExtension loads one or more unpacked Chrome extensions by directory path.
// Extensions require --load-extension and --disable-extensions-except launch flags
// which are set automatically at browser startup.
func WithExtension(paths ...string) Option {
	return func(o *options) { o.extensions = append(o.extensions, paths...) }
}

// WithExtensionByID loads one or more Chrome extensions by their Chrome Web Store ID.
// The extensions must have been previously downloaded with DownloadExtension.
func WithExtensionByID(ids ...string) Option {
	return func(o *options) { o.extensionIDs = append(o.extensionIDs, ids...) }
}

// WithDevTools opens Chrome DevTools automatically for each new tab.
func WithDevTools() Option {
	return func(o *options) { o.devtools = true }
}

// WithBridge enables the built-in Scout Bridge extension for bidirectional
// Go↔browser communication via CDP bindings. The extension is embedded for
// security and written to a temp directory at startup. Enabled by default;
// disable with WithoutBridge() or SCOUT_BRIDGE=false.
func WithBridge() Option {
	return func(o *options) { o.bridge = true }
}

// WithoutBridge disables the built-in Scout Bridge extension.
func WithoutBridge() Option {
	return func(o *options) { o.bridge = false }
}

// Common URL-blocking presets for use with WithBlockPatterns.
var (
	// BlockAds blocks common advertising domains.
	BlockAds = []string{
		"*doubleclick.net*", "*googlesyndication.com*", "*googleadservices.com*",
		"*adnxs.com*", "*adsrvr.org*", "*amazon-adsystem.com*",
		"*moatads.com*", "*serving-sys.com*", "*adform.net*",
	}

	// BlockTrackers blocks common analytics and tracking domains.
	BlockTrackers = []string{
		"*google-analytics.com*", "*googletagmanager.com*",
		"*facebook.net/tr*", "*facebook.com/tr*",
		"*hotjar.com*", "*fullstory.com*", "*segment.io*",
		"*mixpanel.com*", "*amplitude.com*",
	}

	// BlockFonts blocks web font requests.
	BlockFonts = []string{
		"*.woff", "*.woff2", "*.ttf", "*.otf", "*.eot",
		"*fonts.googleapis.com*", "*fonts.gstatic.com*",
	}

	// BlockImages blocks image requests.
	BlockImages = []string{
		"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp", "*.svg", "*.ico", "*.bmp",
	}
)

// WithBlockPatterns sets URL patterns to block on every new page.
// Patterns use wildcards (*) — e.g. "*.css", "*analytics*".
// Use preset slices (BlockAds, BlockTrackers, BlockFonts, BlockImages) or custom patterns.
func WithBlockPatterns(patterns ...string) Option {
	return func(o *options) { o.blockPatterns = append(o.blockPatterns, patterns...) }
}

// WithRemoteCDP connects to an existing Chrome DevTools Protocol endpoint instead of
// launching a local browser. Use this for managed browser services (BrightData, Browserless,
// etc.) or remote Chrome instances. Most launch-related options (execPath, proxy, noSandbox,
// extensions, etc.) are ignored when a remote endpoint is set.
//
// The endpoint should be a WebSocket URL, e.g. "ws://127.0.0.1:9222".
func WithRemoteCDP(endpoint string) Option {
	return func(o *options) { o.remoteCDP = endpoint }
}

// WithSmartWait enables framework-aware waiting on NewPage. When enabled,
// NewPage will call WaitFrameworkReady after page creation, which detects
// the frontend framework and waits for it to finish hydrating/rendering.
func WithSmartWait() Option {
	return func(o *options) { o.smartWait = true }
}

// WithAutoFree enables periodic browser recycling at the given interval.
// On each tick the browser saves open page URLs and cookies, restarts,
// and restores them. This helps avoid memory leaks in long-running sessions.
func WithAutoFree(interval time.Duration) Option {
	return func(o *options) { o.autoFreeInterval = interval }
}

// WithAutoFreeCallback sets a function called before each browser recycle.
func WithAutoFreeCallback(fn func()) Option {
	return func(o *options) { o.autoFreeCallback = fn }
}

// WithTLSProfile sets the TLS/HTTP fingerprint profile for the browser.
// Supported profiles:
//   - "chrome": default Chrome TLS stack (no extra flags)
//   - "randomized": disables HTTP/2 to vary the HTTP fingerprint
//
// For fine-grained TLS/JA3 fingerprint control, use a TLS proxy such as
// utls-based MITM proxies (e.g. cycletls, got-scraping) in combination
// with WithProxy, since Chrome does not expose cipher-suite ordering via
// command-line flags.
func WithTLSProfile(profile string) Option {
	return func(o *options) { o.tlsProfile = profile }
}

// WithWebMCPAutoDiscover enables automatic scanning for WebMCP tools after each page load.
// When enabled, the Browser initializes a WebMCPRegistry that accumulates discovered tools
// across all pages. Use Browser.WebMCPRegistry() to access the collected tools.
func WithWebMCPAutoDiscover() Option {
	return func(o *options) { o.webmcpAutoDiscover = true }
}

// WithBridgePort sets the port for the Bridge WebSocket server. When set and
// bridge is enabled, a WebSocket server starts at ws://127.0.0.1:{port}/bridge
// allowing browser extensions to communicate bidirectionally with Go.
// Use port 0 for auto-assigned port.
func WithBridgePort(port int) Option {
	return func(o *options) { o.bridgePort = port }
}

// WithFingerprint applies a specific fingerprint to the browser session.
// The fingerprint JS is injected via EvalOnNewDocument on every new page.
func WithFingerprint(fp *Fingerprint) Option {
	return func(o *options) { o.fingerprint = fp }
}

// WithRandomFingerprint generates a random fingerprint with the given options
// and applies it to the browser session.
func WithRandomFingerprint(opts ...FingerprintOption) Option {
	return func(o *options) { o.fingerprint = GenerateFingerprint(opts...) }
}

// WithFingerprintRotation enables automatic fingerprint rotation.
// When set, a new fingerprint is generated or selected according to the
// strategy (per-session, per-page, per-domain, or time interval).
// This overrides WithFingerprint and WithRandomFingerprint.
func WithFingerprintRotation(cfg FingerprintRotationConfig) Option {
	return func(o *options) { o.fpRotation = &cfg }
}

// WithVPN sets the VPN provider for proxy-based connectivity.
// The provider's Connect is called during browser creation to obtain the proxy URL.
func WithVPN(provider VPNProvider) Option {
	return func(o *options) { o.vpnProvider = provider }
}

// WithVPNRotation enables automatic server rotation through the configured VPN provider.
func WithVPNRotation(cfg VPNRotationConfig) Option {
	return func(o *options) { o.vpnRotation = &cfg }
}

// WithProxyAuth sets username and password for proxy authentication.
// This configures Chrome's Fetch.AuthRequired handler to automatically
// respond to proxy auth challenges.
func WithProxyAuth(username, password string) Option {
	return func(o *options) {
		o.proxyAuth = &proxyAuthConfig{username: username, password: password}
	}
}

// WithSessionHijack enables automatic session hijacking on new pages.
// When enabled, NewPage will auto-create a SessionHijacker.
func WithSessionHijack() Option {
	return func(o *options) { o.hijack = true }
}

// WithHijackFilter sets the filter for session hijacking.
func WithHijackFilter(f HijackFilter) Option {
	return func(o *options) { o.hijackFilter = &f }
}

// WithLaunchFlag adds a custom Chrome CLI flag. The name should not include the "--" prefix.
func WithLaunchFlag(name string, values ...string) Option {
	return func(o *options) {
		if o.launchFlags == nil {
			o.launchFlags = make(map[string][]string)
		}

		o.launchFlags[name] = values
	}
}
