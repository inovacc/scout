package scout

import (
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	opts := defaults()

	tests := []struct {
		name  string
		check func() bool
	}{
		{"headless_true", func() bool { return opts.headless }},
		{"bridge_true", func() bool { return opts.bridge }},
		{"stealth_false", func() bool { return !opts.stealth }},
		{"windowW_1920", func() bool { return opts.windowW == 1920 }},
		{"windowH_1080", func() bool { return opts.windowH == 1080 }},
		{"timeout_30s", func() bool { return opts.timeout == 30*time.Second }},
		{"no_proxy", func() bool { return opts.proxy == "" }},
		{"no_user_agent", func() bool { return opts.userAgent == "" }},
		{"no_exec_path", func() bool { return opts.execPath == "" }},
		{"not_incognito", func() bool { return !opts.incognito }},
		{"not_no_sandbox", func() bool { return !opts.noSandbox }},
		{"no_devtools", func() bool { return !opts.devtools }},
		{"no_remote_cdp", func() bool { return opts.remoteCDP == "" }},
		{"no_smart_wait", func() bool { return !opts.smartWait }},
		{"no_hijack", func() bool { return !opts.hijack }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Errorf("default check failed for %s", tt.name)
			}
		})
	}
}

func TestWithBrowser(t *testing.T) {
	tests := []struct {
		name string
		bt   BrowserType
	}{
		{"chrome", BrowserChrome},
		{"brave", BrowserBrave},
		{"edge", BrowserEdge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := defaults()
			WithBrowser(tt.bt)(opts)

			if opts.browserType != tt.bt {
				t.Errorf("got %v, want %v", opts.browserType, tt.bt)
			}
		})
	}
}

func TestWithHeadless(t *testing.T) {
	tests := []struct {
		name string
		val  bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := defaults()
			WithHeadless(tt.val)(opts)

			if opts.headless != tt.val {
				t.Errorf("got %v, want %v", opts.headless, tt.val)
			}
		})
	}
}

func TestWithStealth(t *testing.T) {
	opts := defaults()
	WithStealth()(opts)

	if !opts.stealth {
		t.Error("WithStealth should enable stealth")
	}
}

func TestWithUserAgent(t *testing.T) {
	opts := defaults()
	WithUserAgent("Mozilla/5.0 Test")(opts)

	if opts.userAgent != "Mozilla/5.0 Test" {
		t.Errorf("got %q", opts.userAgent)
	}
}

func TestWithProxy(t *testing.T) {
	opts := defaults()
	WithProxy("socks5://127.0.0.1:1080")(opts)

	if opts.proxy != "socks5://127.0.0.1:1080" {
		t.Errorf("got %q", opts.proxy)
	}
}

func TestWithWindowSize(t *testing.T) {
	opts := defaults()
	WithWindowSize(800, 600)(opts)

	if opts.windowW != 800 || opts.windowH != 600 {
		t.Errorf("got %dx%d", opts.windowW, opts.windowH)
	}
}

func TestWithTimeout(t *testing.T) {
	opts := defaults()
	WithTimeout(60 * time.Second)(opts)

	if opts.timeout != 60*time.Second {
		t.Errorf("got %v", opts.timeout)
	}
}

func TestWithSlowMotion(t *testing.T) {
	opts := defaults()
	WithSlowMotion(500 * time.Millisecond)(opts)

	if opts.slowMotion != 500*time.Millisecond {
		t.Errorf("got %v", opts.slowMotion)
	}
}

func TestWithIgnoreCerts(t *testing.T) {
	opts := defaults()
	WithIgnoreCerts()(opts)

	if !opts.ignoreCerts {
		t.Error("should enable ignoreCerts")
	}
}

func TestWithExecPath(t *testing.T) {
	opts := defaults()
	WithExecPath("/usr/bin/chromium")(opts)

	if opts.execPath != "/usr/bin/chromium" {
		t.Errorf("got %q", opts.execPath)
	}
}

func TestWithUserDataDir(t *testing.T) {
	opts := defaults()
	WithUserDataDir("/tmp/profile")(opts)

	if opts.userDataDir != "/tmp/profile" {
		t.Errorf("got %q", opts.userDataDir)
	}
}

func TestWithEnv(t *testing.T) {
	opts := defaults()
	WithEnv("FOO=bar", "BAZ=qux")(opts)

	if len(opts.env) != 2 || opts.env[0] != "FOO=bar" {
		t.Errorf("got %v", opts.env)
	}
	// Append behavior
	WithEnv("EXTRA=val")(opts)

	if len(opts.env) != 3 {
		t.Errorf("expected 3 env vars, got %d", len(opts.env))
	}
}

func TestWithIncognito(t *testing.T) {
	opts := defaults()
	WithIncognito()(opts)

	if !opts.incognito {
		t.Error("should enable incognito")
	}
}

func TestWithNoSandbox(t *testing.T) {
	opts := defaults()
	WithNoSandbox()(opts)

	if !opts.noSandbox {
		t.Error("should enable noSandbox")
	}
}

func TestWithWindowState(t *testing.T) {
	opts := defaults()
	WithWindowState(WindowStateMaximized)(opts)

	if opts.windowState != WindowStateMaximized {
		t.Errorf("got %v", opts.windowState)
	}
}

func TestWithMaximized(t *testing.T) {
	opts := defaults()
	WithMaximized()(opts)

	if opts.windowState != WindowStateMaximized {
		t.Error("should set maximized state")
	}
}

func TestWithExtension(t *testing.T) {
	opts := defaults()
	WithExtension("/path/ext1", "/path/ext2")(opts)

	if len(opts.extensions) != 2 {
		t.Errorf("got %d extensions", len(opts.extensions))
	}
}

func TestWithExtensionByID(t *testing.T) {
	opts := defaults()
	WithExtensionByID("abc123", "def456")(opts)

	if len(opts.extensionIDs) != 2 {
		t.Errorf("got %d extension IDs", len(opts.extensionIDs))
	}
}

func TestWithDevTools(t *testing.T) {
	opts := defaults()
	WithDevTools()(opts)

	if !opts.devtools {
		t.Error("should enable devtools")
	}
}

func TestWithBridge(t *testing.T) {
	opts := defaults()
	opts.bridge = false
	WithBridge()(opts)

	if !opts.bridge {
		t.Error("should enable bridge")
	}
}

func TestWithoutBridge(t *testing.T) {
	opts := defaults()
	WithoutBridge()(opts)

	if opts.bridge {
		t.Error("should disable bridge")
	}
}

func TestWithBlockPatternsOption(t *testing.T) {
	opts := defaults()
	WithBlockPatterns("*.css", "*analytics*")(opts)

	if len(opts.blockPatterns) != 2 {
		t.Errorf("got %d patterns", len(opts.blockPatterns))
	}
	// Append behavior
	WithBlockPatterns("*.js")(opts)

	if len(opts.blockPatterns) != 3 {
		t.Errorf("expected 3, got %d", len(opts.blockPatterns))
	}
}

func TestWithRemoteCDPOption(t *testing.T) {
	opts := defaults()
	WithRemoteCDP("ws://127.0.0.1:9222")(opts)

	if opts.remoteCDP != "ws://127.0.0.1:9222" {
		t.Errorf("got %q", opts.remoteCDP)
	}
}

func TestWithSmartWaitOption(t *testing.T) {
	opts := defaults()
	WithSmartWait()(opts)

	if !opts.smartWait {
		t.Error("should enable smartWait")
	}
}

func TestWithAutoFree(t *testing.T) {
	opts := defaults()
	WithAutoFree(5 * time.Minute)(opts)

	if opts.autoFreeInterval != 5*time.Minute {
		t.Errorf("got %v", opts.autoFreeInterval)
	}
}

func TestWithAutoFreeCallback(t *testing.T) {
	opts := defaults()
	called := false

	WithAutoFreeCallback(func() { called = true })(opts)

	if opts.autoFreeCallback == nil {
		t.Fatal("callback should be set")
	}

	opts.autoFreeCallback()

	if !called {
		t.Error("callback should be callable")
	}
}

func TestWithTLSProfileOption(t *testing.T) {
	opts := defaults()
	WithTLSProfile("randomized")(opts)

	if opts.tlsProfile != "randomized" {
		t.Errorf("got %q", opts.tlsProfile)
	}
}

func TestWithWebMCPAutoDiscoverOption(t *testing.T) {
	opts := defaults()
	WithWebMCPAutoDiscover()(opts)

	if !opts.webmcpAutoDiscover {
		t.Error("should enable webmcpAutoDiscover")
	}
}

func TestWithBridgePort(t *testing.T) {
	opts := defaults()
	WithBridgePort(8080)(opts)

	if opts.bridgePort != 8080 {
		t.Errorf("got %d", opts.bridgePort)
	}
}

func TestWithSessionHijack(t *testing.T) {
	opts := defaults()
	WithSessionHijack()(opts)

	if !opts.hijack {
		t.Error("should enable hijack")
	}
}

func TestWithProxyAuth(t *testing.T) {
	opts := defaults()
	WithProxyAuth("user", "pass")(opts)

	if opts.proxyAuth == nil {
		t.Fatal("proxyAuth should be set")
	}

	if opts.proxyAuth.username != "user" || opts.proxyAuth.password != "pass" {
		t.Errorf("got %q/%q", opts.proxyAuth.username, opts.proxyAuth.password)
	}
}

func TestWithLaunchFlag(t *testing.T) {
	opts := defaults()
	WithLaunchFlag("disable-gpu")(opts)

	if _, ok := opts.launchFlags["disable-gpu"]; !ok {
		t.Error("should set launch flag")
	}

	WithLaunchFlag("window-size", "800,600")(opts)

	vals := opts.launchFlags["window-size"]
	if len(vals) != 1 || vals[0] != "800,600" {
		t.Errorf("got %v", vals)
	}
}

func TestBlockPresets(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		minLen   int
	}{
		{"BlockAds", BlockAds, 5},
		{"BlockTrackers", BlockTrackers, 5},
		{"BlockFonts", BlockFonts, 5},
		{"BlockImages", BlockImages, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.patterns) < tt.minLen {
				t.Errorf("expected at least %d patterns, got %d", tt.minLen, len(tt.patterns))
			}

			for _, p := range tt.patterns {
				if p == "" {
					t.Error("pattern should not be empty")
				}
			}
		})
	}
}

func TestOptionChaining(t *testing.T) {
	opts := defaults()
	chain := []Option{
		WithHeadless(false),
		WithStealth(),
		WithProxy("http://proxy:8080"),
		WithWindowSize(1280, 720),
		WithTimeout(10 * time.Second),
		WithNoSandbox(),
		WithIncognito(),
	}

	for _, o := range chain {
		o(opts)
	}

	if opts.headless {
		t.Error("headless should be false")
	}

	if !opts.stealth {
		t.Error("stealth should be true")
	}

	if opts.proxy != "http://proxy:8080" {
		t.Error("proxy mismatch")
	}

	if opts.windowW != 1280 || opts.windowH != 720 {
		t.Error("window size mismatch")
	}

	if opts.timeout != 10*time.Second {
		t.Error("timeout mismatch")
	}

	if !opts.noSandbox {
		t.Error("noSandbox should be true")
	}

	if !opts.incognito {
		t.Error("incognito should be true")
	}
}

func TestWithElectronApp(t *testing.T) {
	o := defaults()
	WithElectronApp("/path/to/app")(o)

	if o.electronApp != "/path/to/app" {
		t.Errorf("got %q", o.electronApp)
	}
}

func TestWithElectronVersion(t *testing.T) {
	o := defaults()
	WithElectronVersion("v33.2.0")(o)

	if o.electronVersion != "v33.2.0" {
		t.Errorf("got %q", o.electronVersion)
	}
}

func TestWithElectronCDP(t *testing.T) {
	o := defaults()
	WithElectronCDP("ws://127.0.0.1:9222")(o)

	if o.electronCDP != "ws://127.0.0.1:9222" {
		t.Errorf("got %q", o.electronCDP)
	}
}
