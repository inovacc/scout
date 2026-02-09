package scout

import (
	"testing"
	"time"
)

func TestNewBrowser(t *testing.T) {
	b := newTestBrowser(t)

	version, err := b.Version()
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}
	if version == "" {
		t.Error("Version() returned empty string")
	}
	t.Logf("Browser version: %s", version)
}

func TestBrowserPages(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	p1, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = p1.Close() }()

	p2, err := b.NewPage(srv.URL + "/page2")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = p2.Close() }()

	pages, err := b.Pages()
	if err != nil {
		t.Fatalf("Pages() error: %v", err)
	}
	if len(pages) < 2 {
		t.Errorf("Pages() returned %d pages, expected >= 2", len(pages))
	}
}

func TestBrowserCloseIdempotent(t *testing.T) {
	b, err := New(WithHeadless(true), WithNoSandbox())
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}

	if err := b.Close(); err != nil {
		t.Fatalf("first Close() error: %v", err)
	}
	// second close should be nil-safe
	if err := b.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

func TestBrowserNilSafe(t *testing.T) {
	var b *Browser
	if err := b.Close(); err != nil {
		t.Errorf("nil browser Close() should not error: %v", err)
	}
}

func TestBrowserOptions(t *testing.T) {
	o := defaults()

	if !o.headless {
		t.Error("default headless should be true")
	}
	if o.windowW != 1920 || o.windowH != 1080 {
		t.Errorf("default window size should be 1920x1080, got %dx%d", o.windowW, o.windowH)
	}
	if o.timeout != 30*time.Second {
		t.Errorf("default timeout should be 30s, got %v", o.timeout)
	}

	WithStealth()(o)
	if !o.stealth {
		t.Error("WithStealth should set stealth=true")
	}

	WithUserAgent("test")(o)
	if o.userAgent != "test" {
		t.Error("WithUserAgent should set userAgent")
	}

	WithProxy("socks5://localhost:1080")(o)
	if o.proxy != "socks5://localhost:1080" {
		t.Error("WithProxy should set proxy")
	}

	WithWindowSize(800, 600)(o)
	if o.windowW != 800 || o.windowH != 600 {
		t.Error("WithWindowSize should set window dimensions")
	}

	WithTimeout(5 * time.Second)(o)
	if o.timeout != 5*time.Second {
		t.Error("WithTimeout should set timeout")
	}

	WithSlowMotion(100 * time.Millisecond)(o)
	if o.slowMotion != 100*time.Millisecond {
		t.Error("WithSlowMotion should set slowMotion")
	}

	WithIgnoreCerts()(o)
	if !o.ignoreCerts {
		t.Error("WithIgnoreCerts should set ignoreCerts=true")
	}

	WithExecPath("/usr/bin/chrome")(o)
	if o.execPath != "/usr/bin/chrome" {
		t.Error("WithExecPath should set execPath")
	}

	WithUserDataDir("/tmp/test")(o)
	if o.userDataDir != "/tmp/test" {
		t.Error("WithUserDataDir should set userDataDir")
	}

	WithEnv("TZ=UTC")(o)
	if len(o.env) != 1 || o.env[0] != "TZ=UTC" {
		t.Error("WithEnv should append env vars")
	}

	WithIncognito()(o)
	if !o.incognito {
		t.Error("WithIncognito should set incognito=true")
	}

	WithNoSandbox()(o)
	if !o.noSandbox {
		t.Error("WithNoSandbox should set noSandbox=true")
	}
}
