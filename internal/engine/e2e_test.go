//go:build !short

package engine

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- Test HTML helpers ---

func testPage(title, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>%s</title></head>
<body>%s</body></html>`, title, body)
	}
}

func testFormPage() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Form Test</title></head>
<body>
<form id="test-form">
    <input id="name" type="text" placeholder="Name">
    <input id="email" type="email" placeholder="Email">
    <button id="submit" type="submit">Submit</button>
</form>
<div id="result"></div>
<script>
document.getElementById('test-form').addEventListener('submit', function(e) {
    e.preventDefault();
    document.getElementById('result').textContent =
        document.getElementById('name').value + ':' + document.getElementById('email').value;
});
</script>
</body></html>`)
	}
}

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/e2e-extract", testPage("Extract Test",
			`<h1 id="title">Hello E2E</h1>
<p id="desc">End-to-end testing with Scout.</p>
<ul id="items"><li>Alpha</li><li>Beta</li><li>Gamma</li></ul>`))

		mux.HandleFunc("/e2e-form", testFormPage())

		mux.HandleFunc("/e2e-screenshot", testPage("Screenshot Test",
			`<div style="width:200px;height:200px;background:red;" id="box"></div>`))

		mux.HandleFunc("/e2e-snapshot", testPage("Snapshot Test",
			`<nav aria-label="Main"><a href="/home">Home</a></nav>
<main><h1>Dashboard</h1><button id="action">Run</button></main>`))

		mux.HandleFunc("/e2e-markdown", testPage("Markdown Test",
			`<article>
<h1>Guide Title</h1>
<p>This is the <strong>introduction</strong> paragraph.</p>
<ul><li>Step 1</li><li>Step 2</li></ul>
<a href="https://example.com">Reference</a>
</article>`))

		mux.HandleFunc("/e2e-touch", testPage("Touch Test",
			`<div id="target" style="width:100px;height:100px;background:blue;"
ontouchstart="this.textContent='touched'"></div>`))

		mux.HandleFunc("/e2e-hijack", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Hijack E2E</title></head>
<body><h1>Hijack</h1>
<script>fetch('/e2e-hijack-api').then(r => r.json());</script>
</body></html>`)
		})

		mux.HandleFunc("/e2e-hijack-api", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		})

		mux.HandleFunc("/e2e-multi-a", testPage("Page A", `<p id="content">Content A</p>`))
		mux.HandleFunc("/e2e-multi-b", testPage("Page B", `<p id="content">Content B</p>`))

		mux.HandleFunc("/e2e-eval", testPage("Eval Test",
			`<div id="data" data-value="42">Eval Target</div>`))
	})
}

// TestE2ENavigateAndExtract navigates to a test page and extracts text using CSS selectors.
func TestE2ENavigateAndExtract(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/e2e-extract")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Extract heading text.
	el, err := page.Element("#title")
	if err != nil {
		t.Fatalf("Element(#title) error: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if text != "Hello E2E" {
		t.Errorf("expected 'Hello E2E', got %q", text)
	}

	// Extract paragraph text.
	desc, err := page.Element("#desc")
	if err != nil {
		t.Fatalf("Element(#desc) error: %v", err)
	}

	descText, err := desc.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if !strings.Contains(descText, "End-to-end testing") {
		t.Errorf("expected description text, got %q", descText)
	}

	// Extract list items.
	items, err := page.Elements("#items li")
	if err != nil {
		t.Fatalf("Elements(#items li) error: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 list items, got %d", len(items))
	}

	firstText, err := items[0].Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if firstText != "Alpha" {
		t.Errorf("expected 'Alpha', got %q", firstText)
	}
}

// TestE2EClickAndType fills a form by clicking an input, typing text, and verifying the value.
func TestE2EClickAndType(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/e2e-form")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Click and type into the name field.
	nameInput, err := page.Element("#name")
	if err != nil {
		t.Fatalf("Element(#name) error: %v", err)
	}

	if err := nameInput.Click(); err != nil {
		t.Fatalf("Click() error: %v", err)
	}

	if err := nameInput.Input("John Doe"); err != nil {
		t.Fatalf("Input() error: %v", err)
	}

	// Click and type into the email field.
	emailInput, err := page.Element("#email")
	if err != nil {
		t.Fatalf("Element(#email) error: %v", err)
	}

	if err := emailInput.Click(); err != nil {
		t.Fatalf("Click() error: %v", err)
	}

	if err := emailInput.Input("john@example.com"); err != nil {
		t.Fatalf("Input() error: %v", err)
	}

	// Verify the name field value via JS eval.
	result, err := page.Eval(`document.getElementById('name').value`)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}

	if result.String() != "John Doe" {
		t.Errorf("expected name 'John Doe', got %q", result.String())
	}

	// Click submit and verify the result.
	submitBtn, err := page.Element("#submit")
	if err != nil {
		t.Fatalf("Element(#submit) error: %v", err)
	}

	if err := submitBtn.Click(); err != nil {
		t.Fatalf("Click() error: %v", err)
	}

	// Wait briefly for the JS handler to execute.
	time.Sleep(200 * time.Millisecond)

	resultDiv, err := page.Element("#result")
	if err != nil {
		t.Fatalf("Element(#result) error: %v", err)
	}

	resultText, err := resultDiv.Text()
	if err != nil {
		t.Fatalf("Text() error: %v", err)
	}

	if resultText != "John Doe:john@example.com" {
		t.Errorf("expected 'John Doe:john@example.com', got %q", resultText)
	}
}

// TestE2EScreenshot navigates to a page and takes a screenshot, verifying non-empty bytes.
func TestE2EScreenshot(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/e2e-screenshot")
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
		t.Fatal("screenshot returned empty byte slice")
	}

	// PNG files start with the magic bytes 0x89 0x50 0x4E 0x47.
	if len(data) < 4 || data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4E || data[3] != 0x47 {
		t.Error("screenshot does not appear to be a valid PNG")
	}
}

// TestE2ESnapshot takes an accessibility snapshot and verifies it contains page elements.
func TestE2ESnapshot(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/e2e-snapshot")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	snap, err := page.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error: %v", err)
	}

	if !strings.Contains(snap, "- document") {
		t.Error("snapshot should start with document root")
	}

	if !strings.Contains(snap, `navigation "Main"`) {
		t.Error("snapshot should contain navigation landmark")
	}

	if !strings.Contains(snap, `heading "Dashboard"`) {
		t.Error("snapshot should contain Dashboard heading")
	}

	if !strings.Contains(snap, `button "Run"`) {
		t.Error("snapshot should contain Run button")
	}

	if !strings.Contains(snap, `link "Home"`) {
		t.Error("snapshot should contain Home link")
	}
}

// TestE2EMarkdown converts a page to markdown and verifies heading and content.
func TestE2EMarkdown(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/e2e-markdown")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	md, err := page.Markdown()
	if err != nil {
		t.Fatalf("Markdown() error: %v", err)
	}

	if !strings.Contains(md, "# Guide Title") {
		t.Errorf("missing heading in markdown:\n%s", md)
	}

	if !strings.Contains(md, "**introduction**") {
		t.Errorf("missing bold text in markdown:\n%s", md)
	}

	if !strings.Contains(md, "- Step 1") {
		t.Errorf("missing list items in markdown:\n%s", md)
	}

	if !strings.Contains(md, "[Reference](https://example.com)") {
		t.Errorf("missing link in markdown:\n%s", md)
	}
}

// TestE2ETouchGestures creates a page with touch emulation and verifies Touch/Swipe don't error.
func TestE2ETouchGestures(t *testing.T) {
	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTouchEmulation(),
		WithTimeout(30e9),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/e2e-touch")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Touch at coordinates (50, 50) — center of the target div.
	if err := page.Touch(50, 50); err != nil {
		t.Fatalf("Touch() error: %v", err)
	}

	// Swipe gesture from (50, 50) to (150, 50) over 300ms.
	if err := page.Swipe(50, 50, 150, 50, 300*time.Millisecond); err != nil {
		t.Fatalf("Swipe() error: %v", err)
	}
}

// TestE2ESessionHijack enables session hijacking, navigates, and verifies request events are captured.
func TestE2ESessionHijack(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	hijacker, err := page.NewSessionHijacker()
	if err != nil {
		t.Fatalf("NewSessionHijacker() error: %v", err)
	}
	defer hijacker.Stop()

	if err := page.Navigate(srv.URL + "/e2e-hijack"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Collect events with timeout.
	var events []HijackEvent

	timeout := time.After(5 * time.Second)

	for {
		select {
		case ev, ok := <-hijacker.Events():
			if !ok {
				goto done
			}

			events = append(events, ev)
			// Expect at least request+response for the page and the API fetch.
			if len(events) >= 4 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}

done:

	if len(events) == 0 {
		t.Fatal("expected at least one hijack event")
	}

	var hasRequest, hasResponse bool

	for _, ev := range events {
		switch ev.Type { //nolint:exhaustive
		case HijackEventRequest:
			hasRequest = true

			if ev.Request == nil {
				t.Error("request event has nil Request")
			}
		case HijackEventResponse:
			hasResponse = true

			if ev.Response == nil {
				t.Error("response event has nil Response")
			}
		}
	}

	if !hasRequest {
		t.Error("expected at least one request event")
	}

	if !hasResponse {
		t.Error("expected at least one response event")
	}

	// Verify at least one event URL matches the API endpoint.
	var foundAPI bool

	for _, ev := range events {
		if ev.Request != nil && strings.Contains(ev.Request.URL, "/e2e-hijack-api") {
			foundAPI = true
			break
		}
	}

	if !foundAPI {
		t.Error("expected to capture /e2e-hijack-api request")
	}
}

// TestE2EWebSocketHAR records WebSocket traffic via session hijacker and exports to HAR.
func TestE2EWebSocketHAR(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	hijacker, err := page.NewSessionHijacker()
	if err != nil {
		t.Fatalf("NewSessionHijacker() error: %v", err)
	}
	defer hijacker.Stop()

	recorder := NewHijackRecorder()

	// Navigate to WS page (reuses the existing /hijack-ws-page from hijack_session_test.go).
	if err := page.Navigate(srv.URL + "/hijack-ws-page"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Collect events and record them.
	var wsEvents []HijackEvent

	timeout := time.After(5 * time.Second)

	for {
		select {
		case ev, ok := <-hijacker.Events():
			if !ok {
				goto done
			}

			recorder.Record(ev)

			if ev.Frame != nil {
				wsEvents = append(wsEvents, ev)
			}

			// We have enough data once we see WS frames.
			if len(wsEvents) >= 2 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}

done:

	// HAR export should work (WS events are ignored in HAR, but HTTP events are recorded).
	data, _, err := recorder.ExportHAR()
	if err != nil {
		t.Fatalf("ExportHAR() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("HAR export returned empty data")
	}

	if !strings.Contains(string(data), `"version":"1.2"`) {
		t.Error("HAR output missing version field")
	}
}

// TestE2EMultiPage opens multiple pages and verifies they maintain independent state.
func TestE2EMultiPage(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	pageA, err := b.NewPage(srv.URL + "/e2e-multi-a")
	if err != nil {
		t.Fatalf("NewPage(A) error: %v", err)
	}
	defer func() { _ = pageA.Close() }()

	pageB, err := b.NewPage(srv.URL + "/e2e-multi-b")
	if err != nil {
		t.Fatalf("NewPage(B) error: %v", err)
	}
	defer func() { _ = pageB.Close() }()

	if err := pageA.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad(A) error: %v", err)
	}

	if err := pageB.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad(B) error: %v", err)
	}

	// Extract content from page A.
	elA, err := pageA.Element("#content")
	if err != nil {
		t.Fatalf("Element(A) error: %v", err)
	}

	textA, err := elA.Text()
	if err != nil {
		t.Fatalf("Text(A) error: %v", err)
	}

	if textA != "Content A" {
		t.Errorf("page A: expected 'Content A', got %q", textA)
	}

	// Extract content from page B.
	elB, err := pageB.Element("#content")
	if err != nil {
		t.Fatalf("Element(B) error: %v", err)
	}

	textB, err := elB.Text()
	if err != nil {
		t.Fatalf("Text(B) error: %v", err)
	}

	if textB != "Content B" {
		t.Errorf("page B: expected 'Content B', got %q", textB)
	}

	// Verify titles are independent.
	titleA, err := pageA.Eval(`document.title`)
	if err != nil {
		t.Fatalf("Eval title(A) error: %v", err)
	}

	titleB, err := pageB.Eval(`document.title`)
	if err != nil {
		t.Fatalf("Eval title(B) error: %v", err)
	}

	if titleA.String() != "Page A" {
		t.Errorf("page A title: expected 'Page A', got %q", titleA.String())
	}

	if titleB.String() != "Page B" {
		t.Errorf("page B title: expected 'Page B', got %q", titleB.String())
	}
}

// TestE2EEval evaluates JavaScript expressions and verifies return values.
func TestE2EEval(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/e2e-eval")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Evaluate a simple arithmetic expression.
	result, err := page.Eval(`2 + 3`)
	if err != nil {
		t.Fatalf("Eval(2+3) error: %v", err)
	}

	if result.Int() != 5 {
		t.Errorf("expected 5, got %d", result.Int())
	}

	// Evaluate a string expression.
	strResult, err := page.Eval(`"hello" + " " + "world"`)
	if err != nil {
		t.Fatalf("Eval(string) error: %v", err)
	}

	if strResult.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", strResult.String())
	}

	// Evaluate DOM query.
	attrResult, err := page.Eval(`document.getElementById('data').getAttribute('data-value')`)
	if err != nil {
		t.Fatalf("Eval(getAttribute) error: %v", err)
	}

	if attrResult.String() != "42" {
		t.Errorf("expected '42', got %q", attrResult.String())
	}

	// Evaluate a boolean expression.
	boolResult, err := page.Eval(`document.title === "Eval Test"`)
	if err != nil {
		t.Fatalf("Eval(boolean) error: %v", err)
	}

	if !boolResult.Bool() {
		t.Error("expected true for title comparison")
	}

	// Evaluate null/undefined.
	nullResult, err := page.Eval(`null`)
	if err != nil {
		t.Fatalf("Eval(null) error: %v", err)
	}

	if !nullResult.IsNull() {
		t.Error("expected null result")
	}
}
