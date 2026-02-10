package scout

import (
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

func TestPageNavigateAndTitle(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title() error: %v", err)
	}

	if title != "Test Page" {
		t.Errorf("Title() = %q, want %q", title, "Test Page")
	}

	u, err := page.URL()
	if err != nil {
		t.Fatalf("URL() error: %v", err)
	}

	if !strings.HasPrefix(u, "http://") {
		t.Errorf("URL() = %q, expected http:// prefix", u)
	}
}

func TestPageHTML(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	html, err := page.HTML()
	if err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	if !strings.Contains(html, "Hello World") {
		t.Error("HTML() should contain 'Hello World'")
	}
}

func TestPageScreenshot(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	data, err := page.Screenshot()
	if err != nil {
		t.Fatalf("Screenshot() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("Screenshot() returned empty data")
	}

	// PNG magic bytes
	if len(data) > 4 && data[0] != 0x89 {
		t.Error("Screenshot() should return PNG data")
	}
}

func TestPageFullScreenshot(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	data, err := page.FullScreenshot()
	if err != nil {
		t.Fatalf("FullScreenshot() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("FullScreenshot() returned empty data")
	}
}

func TestPageScreenshotJPEG(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	data, err := page.ScreenshotJPEG(80)
	if err != nil {
		t.Fatalf("ScreenshotJPEG() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("ScreenshotJPEG() returned empty data")
	}
	// JPEG magic bytes
	if len(data) > 2 && (data[0] != 0xFF || data[1] != 0xD8) {
		t.Error("ScreenshotJPEG() should return JPEG data")
	}
}

func TestPageEval(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	result, err := page.Eval(`() => document.title`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}

	if result.String() != "Test Page" {
		t.Errorf("Eval() = %q, want %q", result.String(), "Test Page")
	}

	// Test with arguments
	result, err = page.Eval(`(a, b) => a + b`, 10, 20)
	if err != nil {
		t.Fatalf("Eval() with args error: %v", err)
	}

	if result.Int() != 30 {
		t.Errorf("Eval() = %d, want 30", result.Int())
	}

	// Bool test
	result, err = page.Eval(`() => true`)
	if err != nil {
		t.Fatalf("Eval() bool error: %v", err)
	}

	if !result.Bool() {
		t.Error("Eval() should return true")
	}
}

func TestPageElement(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Hello World" {
		t.Errorf("Text() = %q, want %q", text, "Hello World")
	}
}

func TestPageElements(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	els, err := page.Elements("option")
	if err != nil {
		t.Fatalf("Elements() error: %v", err)
	}

	if len(els) != 3 {
		t.Errorf("Elements() returned %d elements, want 3", len(els))
	}
}

func TestPageElementByXPath(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.ElementByXPath("//h1")
	if err != nil {
		t.Fatalf("ElementByXPath() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Hello World" {
		t.Errorf("Text() = %q, want %q", text, "Hello World")
	}
}

func TestPageHas(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	has, err := page.Has("h1")
	if err != nil {
		t.Fatalf("Has() error: %v", err)
	}

	if !has {
		t.Error("Has('h1') should be true")
	}

	has, err = page.Has("#nonexistent")
	if err != nil {
		t.Fatalf("Has() error: %v", err)
	}

	if has {
		t.Error("Has('#nonexistent') should be false")
	}
}

func TestPageNavigateAndBack(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.Navigate(srv.URL + "/page2"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title() error: %v", err)
	}

	if title != "Page Two" {
		t.Errorf("Title() = %q, want %q", title, "Page Two")
	}

	if err := page.NavigateBack(); err != nil {
		t.Fatalf("NavigateBack() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	title, err = page.Title()
	if err != nil {
		t.Fatalf("Title() error: %v", err)
	}

	if title != "Test Page" {
		t.Errorf("Title() after back = %q, want %q", title, "Test Page")
	}
}

func TestPageWaitSelector(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/slow")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.WaitSelector("#delayed")
	if err != nil {
		t.Fatalf("WaitSelector() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Loaded" {
		t.Errorf("Text() = %q, want %q", text, "Loaded")
	}
}

func TestPageSetViewport(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.SetViewport(800, 600); err != nil {
		t.Fatalf("SetViewport() error: %v", err)
	}

	result, err := page.Eval(`() => [window.innerWidth, window.innerHeight]`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}
	// The viewport should be set
	if result.IsNull() {
		t.Error("viewport eval returned null")
	}
}

func TestPageRedirect(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/redirect")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title() error: %v", err)
	}

	if title != "Page Two" {
		t.Errorf("Title() = %q, want %q (after redirect)", title, "Page Two")
	}
}

func TestPageKeyPress(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	// Focus the input first
	el, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}
	if err := el.Focus(); err != nil {
		t.Fatalf("Focus() error: %v", err)
	}

	// Press Tab key to move focus
	if err := page.KeyPress(input.Tab); err != nil {
		t.Fatalf("KeyPress() error: %v", err)
	}
}

func TestPageKeyType(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	// Focus the input
	el, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}
	if err := el.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}
	if err := el.Focus(); err != nil {
		t.Fatalf("Focus() error: %v", err)
	}

	// Type keys using page-level method
	if err := page.KeyType(input.KeyA, input.KeyB, input.KeyC); err != nil {
		t.Fatalf("KeyType() error: %v", err)
	}

	val, err := el.Property("value")
	if err != nil {
		t.Fatalf("Property() error: %v", err)
	}

	if val != "abc" {
		t.Errorf("KeyType() input value = %q, want %q", val, "abc")
	}
}
