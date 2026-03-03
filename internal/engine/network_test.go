package engine

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/inovacc/scout/pkg/scout/rod/lib/launcher"
)

func TestSetHeaders(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/echo-headers")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	cleanup, err := page.SetHeaders(map[string]string{
		"X-Custom": "test-value",
	})
	if err != nil {
		t.Fatalf("SetHeaders() error: %v", err)
	}
	defer cleanup()

	if err := page.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	el, err := page.Element("#custom")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "test-value" {
		t.Errorf("custom header = %q, want %q", text, "test-value")
	}
}

func TestSetAndGetCookies(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.SetCookies(Cookie{
		Name:  "mykey",
		Value: "myval",
		URL:   srv.URL,
	}); err != nil {
		t.Fatalf("SetCookies() error: %v", err)
	}

	cookies, err := page.GetCookies()
	if err != nil {
		t.Fatalf("GetCookies() error: %v", err)
	}

	found := false

	for _, c := range cookies {
		if c.Name == "mykey" && c.Value == "myval" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("cookie 'mykey' not found in %v", cookies)
	}
}

func TestClearCookies(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/set-cookie")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.ClearCookies(); err != nil {
		t.Fatalf("ClearCookies() error: %v", err)
	}

	cookies, err := page.GetCookies()
	if err != nil {
		t.Fatalf("GetCookies() error: %v", err)
	}

	if len(cookies) != 0 {
		t.Errorf("expected 0 cookies after clear, got %d", len(cookies))
	}
}

func TestHijack(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	var intercepted atomic.Bool

	router, err := page.Hijack("*json*", func(ctx *HijackContext) {
		intercepted.Store(true)
		ctx.Response().SetBody(`{"hijacked":true}`)
	})
	if err != nil {
		t.Fatalf("Hijack() error: %v", err)
	}

	go router.Run()

	defer func() { _ = router.Stop() }()

	if err := page.Navigate(srv.URL + "/json"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	html, err := page.HTML()
	if err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	if !strings.Contains(html, "hijacked") {
		t.Errorf("expected hijacked response, got: %s", html)
	}

	if !intercepted.Load() {
		t.Error("hijack handler was not called")
	}
}

func TestHijackRequestAccessors(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	var (
		gotMethod string
		gotURL    string
		gotHeader string
		gotBody   string
	)

	router, err := page.Hijack("*echo*", func(ctx *HijackContext) {
		req := ctx.Request()
		gotMethod = req.Method()
		gotURL = req.URL().String()
		gotHeader = req.Header("Accept")
		gotBody = req.Body()

		// Test response accessors
		ctx.Response().SetHeader("X-Test", "value")
		ctx.Response().SetBody(`{"ok":true}`)
	})
	if err != nil {
		t.Fatalf("Hijack() error: %v", err)
	}

	go router.Run()

	defer func() { _ = router.Stop() }()

	if err := page.Navigate(srv.URL + "/echo-headers"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if gotMethod != "GET" {
		t.Errorf("Method() = %q, want GET", gotMethod)
	}

	if gotURL == "" {
		t.Error("URL() should not be empty")
	}
	// Accept header may or may not be set, just ensure no panic
	_ = gotHeader
	_ = gotBody
}

func TestHijackLoadResponse(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	router, err := page.Hijack("*json*", func(ctx *HijackContext) {
		if err := ctx.LoadResponse(true); err != nil {
			t.Logf("LoadResponse() error: %v", err)
		}
		// Modify response after loading
		ctx.Response().SetBody(`{"modified":true}`)
	})
	if err != nil {
		t.Fatalf("Hijack() error: %v", err)
	}

	go router.Run()

	defer func() { _ = router.Stop() }()

	if err := page.Navigate(srv.URL + "/json"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	html, _ := page.HTML()
	if !strings.Contains(html, "modified") {
		t.Errorf("response should be modified, got: %s", html)
	}
}

func TestHijackSkip(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	var skipped atomic.Bool

	router, err := page.Hijack("*", func(ctx *HijackContext) {
		skipped.Store(true)
		ctx.Skip()
		ctx.ContinueRequest()
	})
	if err != nil {
		t.Fatalf("Hijack() error: %v", err)
	}

	go router.Run()

	defer func() { _ = router.Stop() }()

	if err := page.Navigate(srv.URL); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if !skipped.Load() {
		t.Error("handler should have been called and skipped")
	}
}

func TestHijackResponseFail(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	router, err := page.Hijack("*json*", func(ctx *HijackContext) {
		ctx.Response().Fail("BlockedByClient")
	})
	if err != nil {
		t.Fatalf("Hijack() error: %v", err)
	}

	go router.Run()

	defer func() { _ = router.Stop() }()

	// Navigate to JSON — it should fail, which is expected
	_ = page.Navigate(srv.URL + "/json")
}

func TestWithBlockPatterns(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithBlockPatterns("*json*"),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}

	defer func() { _ = b.Close() }()

	// Blocked pattern should be set automatically on new pages
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	// The page should open fine (root page is not blocked)
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}
}

func TestBlockPresetVariables(t *testing.T) {
	// Verify preset slices are non-empty
	if len(BlockAds) == 0 {
		t.Error("BlockAds should not be empty")
	}

	if len(BlockTrackers) == 0 {
		t.Error("BlockTrackers should not be empty")
	}

	if len(BlockFonts) == 0 {
		t.Error("BlockFonts should not be empty")
	}

	if len(BlockImages) == 0 {
		t.Error("BlockImages should not be empty")
	}
}

func TestWithBlockPatternsMultiplePresets(t *testing.T) {
	// Verify combining presets works at option level
	o := defaults()
	WithBlockPatterns(BlockAds...)(o)
	WithBlockPatterns(BlockFonts...)(o)

	expected := len(BlockAds) + len(BlockFonts)
	if len(o.blockPatterns) != expected {
		t.Errorf("blockPatterns length = %d, want %d", len(o.blockPatterns), expected)
	}
}

func TestPageBlock(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	// Block should work like SetBlockedURLs
	if err := page.Block("*json*"); err != nil {
		t.Fatalf("Block() error: %v", err)
	}

	// Block with presets
	if err := page.Block(BlockAds...); err != nil {
		t.Fatalf("Block(BlockAds) error: %v", err)
	}
}

func TestWithRemoteCDP(t *testing.T) {
	// Launch a browser via the launcher to get a WebSocket CDP endpoint
	l := launcher.New().Headless(true).NoSandbox(true)

	u, err := l.Launch()
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}
	defer l.Kill()

	// Connect via WithRemoteCDP — should skip local launcher
	b, err := New(WithRemoteCDP(u))
	if err != nil {
		t.Fatalf("New(WithRemoteCDP) error: %v", err)
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title() error: %v", err)
	}

	// Verify we got a valid page (title is empty or "about:blank" depending on browser)
	if title != "" && title != "about:blank" {
		t.Errorf("Title() = %q, want empty or about:blank", title)
	}
}

func TestWithRemoteCDPOptionSetsField(t *testing.T) {
	o := defaults()
	WithRemoteCDP("ws://127.0.0.1:9222")(o)

	if o.remoteCDP != "ws://127.0.0.1:9222" {
		t.Errorf("remoteCDP = %q, want %q", o.remoteCDP, "ws://127.0.0.1:9222")
	}
}

func TestHandleAuth(t *testing.T) {
	b := newTestBrowser(t)

	// HandleAuth returns a function — just verify it doesn't panic
	waitAuth := b.HandleAuth("user", "pass")
	if waitAuth == nil {
		t.Error("HandleAuth() should return a non-nil function")
	}
}
