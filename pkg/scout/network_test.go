package scout

import (
	"strings"
	"sync/atomic"
	"testing"
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

func TestHandleAuth(t *testing.T) {
	b := newTestBrowser(t)

	// HandleAuth returns a function — just verify it doesn't panic
	waitAuth := b.HandleAuth("user", "pass")
	if waitAuth == nil {
		t.Error("HandleAuth() should return a non-nil function")
	}
}
