package scout

import (
	"os"
	"time"
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
	browserType   BrowserType
	headless      bool
	stealth       bool
	userAgent     string
	proxy         string
	windowW       int
	windowH       int
	timeout       time.Duration
	slowMotion    time.Duration
	ignoreCerts   bool
	execPath      string
	userDataDir   string
	env           []string
	incognito     bool
	noSandbox     bool
	windowState   WindowState
	xvfb          bool
	xvfbArgs      []string
	launchFlags   map[string][]string
	extensions    []string
	extensionIDs  []string
	devtools      bool
	bridge        bool
	blockPatterns []string
	remoteCDP     string
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

// WithLaunchFlag adds a custom Chrome CLI flag. The name should not include the "--" prefix.
func WithLaunchFlag(name string, values ...string) Option {
	return func(o *options) {
		if o.launchFlags == nil {
			o.launchFlags = make(map[string][]string)
		}

		o.launchFlags[name] = values
	}
}
