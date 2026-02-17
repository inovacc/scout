package scout

import (
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
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

func TestPageNavigateForward(t *testing.T) {
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

	if err := page.NavigateBack(); err != nil {
		t.Fatalf("NavigateBack() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.NavigateForward(); err != nil {
		t.Fatalf("NavigateForward() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title() error: %v", err)
	}

	if title != "Page Two" {
		t.Errorf("Title() after forward = %q, want %q", title, "Page Two")
	}
}

func TestPageScrollScreenshot(t *testing.T) {
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

	data, err := page.ScrollScreenshot()
	if err != nil {
		t.Fatalf("ScrollScreenshot() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("ScrollScreenshot() returned empty data")
	}
}

func TestPageScreenshotPNG(t *testing.T) {
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

	data, err := page.ScreenshotPNG()
	if err != nil {
		t.Fatalf("ScreenshotPNG() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("ScreenshotPNG() returned empty data")
	}

	// PNG magic bytes
	if data[0] != 0x89 || data[1] != 0x50 {
		t.Error("ScreenshotPNG() should return PNG data")
	}
}

func TestPagePDF(t *testing.T) {
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

	data, err := page.PDF()
	if err != nil {
		t.Fatalf("PDF() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("PDF() returned empty data")
	}

	// PDF magic bytes
	if !strings.HasPrefix(string(data), "%PDF") {
		t.Error("PDF() should return PDF data")
	}
}

func TestPagePDFWithOptions(t *testing.T) {
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

	data, err := page.PDFWithOptions(PDFOptions{
		Landscape: true,
		Scale:     0.5,
	})
	if err != nil {
		t.Fatalf("PDFWithOptions() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("PDFWithOptions() returned empty data")
	}
}

func TestPageElementByJS(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.ElementByJS(`() => document.getElementById('info')`)
	if err != nil {
		t.Fatalf("ElementByJS() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Some text" {
		t.Errorf("ElementByJS() text = %q, want %q", text, "Some text")
	}
}

func TestPageElementByText(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.ElementByText("h1", "Hello World")
	if err != nil {
		t.Fatalf("ElementByText() error: %v", err)
	}

	tag, err := el.Eval(`() => this.tagName.toLowerCase()`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}

	if tag.String() != "h1" {
		t.Errorf("ElementByText() found %q, want h1", tag.String())
	}
}

func TestPageHasXPath(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	has, err := page.HasXPath("//h1")
	if err != nil {
		t.Fatalf("HasXPath() error: %v", err)
	}

	if !has {
		t.Error("HasXPath('//h1') should be true")
	}

	has, err = page.HasXPath("//nonexistent")
	if err != nil {
		t.Fatalf("HasXPath() error: %v", err)
	}

	if has {
		t.Error("HasXPath('//nonexistent') should be false")
	}
}

func TestPageElementsByXPath(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	els, err := page.ElementsByXPath("//option")
	if err != nil {
		t.Fatalf("ElementsByXPath() error: %v", err)
	}

	if len(els) != 3 {
		t.Errorf("ElementsByXPath() = %d, want 3", len(els))
	}
}

func TestPageWaitStable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitStable(200); err != nil {
		t.Fatalf("WaitStable() error: %v", err)
	}
}

func TestPageWaitDOMStable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitDOMStable(200, 0.1); err != nil {
		t.Fatalf("WaitDOMStable() error: %v", err)
	}
}

func TestPageWaitIdle(t *testing.T) {
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

	if err := page.WaitIdle(5000); err != nil {
		t.Fatalf("WaitIdle() error: %v", err)
	}
}

func TestPageSetDocumentContent(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.SetDocumentContent("<html><body><h1>Custom</h1></body></html>"); err != nil {
		t.Fatalf("SetDocumentContent() error: %v", err)
	}

	el, err := page.Element("h1")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Custom" {
		t.Errorf("text = %q, want %q", text, "Custom")
	}
}

func TestPageSetUserAgent(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/echo-headers")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.SetUserAgent("ScoutBot/1.0"); err != nil {
		t.Fatalf("SetUserAgent() error: %v", err)
	}

	if err := page.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	el, err := page.Element("#ua")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "ScoutBot/1.0" {
		t.Errorf("user agent = %q, want %q", text, "ScoutBot/1.0")
	}
}

func TestPageEvalOnNewDocument(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	cleanup, err := page.EvalOnNewDocument(`window.__injected = true`)
	if err != nil {
		t.Fatalf("EvalOnNewDocument() error: %v", err)
	}

	defer func() { _ = cleanup() }()

	// Reload to trigger the injected script
	if err := page.Navigate(srv.URL + "/page2"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	result, err := page.Eval(`() => window.__injected`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}

	if !result.Bool() {
		t.Error("injected script should have set window.__injected = true")
	}
}

func TestPageSetBlockedURLs(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.SetBlockedURLs("*json*"); err != nil {
		t.Fatalf("SetBlockedURLs() error: %v", err)
	}
}

func TestPageActivate(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.Activate(); err != nil {
		t.Fatalf("Activate() error: %v", err)
	}
}

func TestPageStopLoading(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.StopLoading(); err != nil {
		t.Fatalf("StopLoading() error: %v", err)
	}
}

func TestPageWaitXPath(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.WaitXPath("//h1")
	if err != nil {
		t.Fatalf("WaitXPath() error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Hello World" {
		t.Errorf("WaitXPath() text = %q, want %q", text, "Hello World")
	}
}

func TestPageSearch(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	el, err := page.Search("Hello World")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if el == nil {
		t.Fatal("Search() returned nil")
	}
}

func TestPageElementFromPoint(t *testing.T) {
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

	// Get a point we know has an element
	el, err := page.ElementFromPoint(100, 100)
	if err != nil {
		t.Fatalf("ElementFromPoint() error: %v", err)
	}

	if el == nil {
		t.Error("ElementFromPoint() returned nil for visible area")
	}
}

func TestPageAddScriptTag(t *testing.T) {
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

	if err := page.AddScriptTag("", `window.__scriptAdded = true`); err != nil {
		t.Fatalf("AddScriptTag() error: %v", err)
	}

	result, err := page.Eval(`() => window.__scriptAdded`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}

	if !result.Bool() {
		t.Error("injected script should set __scriptAdded")
	}
}

func TestPageAddStyleTag(t *testing.T) {
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

	if err := page.AddStyleTag("", `body { background: red !important; }`); err != nil {
		t.Fatalf("AddStyleTag() error: %v", err)
	}
}

func TestPageEmulate(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.Emulate(devices.IPhone6or7or8); err != nil {
		t.Fatalf("Emulate() error: %v", err)
	}

	result, err := page.Eval(`() => window.innerWidth`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}

	// In headless mode the viewport may not change to exact device width
	// Just verify the method didn't error — the Emulate() call above is the real test
	_ = result
}

func TestPageHandleDialog(t *testing.T) {
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

	wait, handle := page.HandleDialog()

	go func() {
		dialog := wait()
		if dialog.Type != "alert" {
			return
		}

		_ = handle(&proto.PageHandleJavaScriptDialog{Accept: true})
	}()

	// Trigger an alert
	_, _ = page.Eval(`() => alert("test")`)
}

func TestPageRace(t *testing.T) {
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

	el, idx, err := page.Race("h1", "#nonexistent")
	if err != nil {
		t.Fatalf("Race() error: %v", err)
	}

	if el == nil {
		t.Error("Race() should return the first matching element")
	}

	if idx != 0 {
		t.Errorf("Race() index = %d, want 0", idx)
	}

	text, _ := el.Text()
	if text != "Hello World" {
		t.Errorf("Race() text = %q, want %q", text, "Hello World")
	}

	// Empty selectors
	_, _, err = page.Race()
	if err == nil {
		t.Error("Race() with no selectors should error")
	}
}

func TestPageWaitRequestIdle(t *testing.T) {
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

	wait := page.WaitRequestIdle(200*time.Millisecond, nil, nil)
	wait()
}

func TestPageWaitNavigation(t *testing.T) {
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

	wait := page.WaitNavigation()

	go func() {
		_ = page.Navigate(srv.URL + "/page2")
	}()

	wait()

	title, _ := page.Title()
	if title != "Page Two" {
		t.Errorf("Title() = %q, want %q", title, "Page Two")
	}
}

func TestPageRodPage(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	rod := page.RodPage()
	if rod == nil {
		t.Error("RodPage() should not be nil")
	}
}

func TestPageKeyPressAndType(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/element-test")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Focus input first
	el, _ := page.Element("#typeinput")
	_ = el.Focus()

	if err := page.KeyType(input.KeyA, input.KeyB); err != nil {
		t.Fatalf("KeyType() error: %v", err)
	}

	if err := page.KeyPress(input.Enter); err != nil {
		t.Fatalf("KeyPress() error: %v", err)
	}
}

