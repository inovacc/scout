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
