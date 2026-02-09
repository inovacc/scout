package scout

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

// PDFOptions configures PDF generation.
type PDFOptions struct {
	Landscape           bool
	DisplayHeaderFooter bool
	PrintBackground     bool
	Scale               float64
	PaperWidth          float64
	PaperHeight         float64
	MarginTop           float64
	MarginBottom        float64
	MarginLeft          float64
	MarginRight         float64
	PageRanges          string
	HeaderTemplate      string
	FooterTemplate      string
}

// Page wraps a rod page (browser tab) with a simplified API.
type Page struct {
	page    *rod.Page
	browser *Browser
}

// Navigate loads the given URL in the page.
func (p *Page) Navigate(url string) error {
	if err := p.page.Navigate(url); err != nil {
		return fmt.Errorf("scout: navigate to %s: %w", url, err)
	}

	return nil
}

// NavigateBack navigates to the previous page in history.
func (p *Page) NavigateBack() error {
	if err := p.page.NavigateBack(); err != nil {
		return fmt.Errorf("scout: navigate back: %w", err)
	}

	return nil
}

// NavigateForward navigates to the next page in history.
func (p *Page) NavigateForward() error {
	if err := p.page.NavigateForward(); err != nil {
		return fmt.Errorf("scout: navigate forward: %w", err)
	}

	return nil
}

// Reload refreshes the current page.
func (p *Page) Reload() error {
	if err := p.page.Reload(); err != nil {
		return fmt.Errorf("scout: reload: %w", err)
	}

	return nil
}

// Close closes the page (tab).
func (p *Page) Close() error {
	if err := p.page.Close(); err != nil {
		return fmt.Errorf("scout: close page: %w", err)
	}

	return nil
}

// URL returns the current page URL.
func (p *Page) URL() (string, error) {
	info, err := p.page.Info()
	if err != nil {
		return "", fmt.Errorf("scout: get page url: %w", err)
	}

	return info.URL, nil
}

// Title returns the current page title.
func (p *Page) Title() (string, error) {
	info, err := p.page.Info()
	if err != nil {
		return "", fmt.Errorf("scout: get page title: %w", err)
	}

	return info.Title, nil
}

// HTML returns the full HTML content of the page.
func (p *Page) HTML() (string, error) {
	html, err := p.page.HTML()
	if err != nil {
		return "", fmt.Errorf("scout: get html: %w", err)
	}

	return html, nil
}

// Screenshot captures a screenshot of the visible viewport as PNG bytes.
func (p *Page) Screenshot() ([]byte, error) {
	data, err := p.page.Screenshot(false, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: screenshot: %w", err)
	}

	return data, nil
}

// FullScreenshot captures a full-page screenshot as PNG bytes.
func (p *Page) FullScreenshot() ([]byte, error) {
	data, err := p.page.Screenshot(true, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: full screenshot: %w", err)
	}

	return data, nil
}

// ScrollScreenshot captures a scrolling screenshot of the entire page.
func (p *Page) ScrollScreenshot() ([]byte, error) {
	data, err := p.page.ScrollScreenshot(nil)
	if err != nil {
		return nil, fmt.Errorf("scout: scroll screenshot: %w", err)
	}

	return data, nil
}

// ScreenshotPNG captures a viewport screenshot in PNG format.
func (p *Page) ScreenshotPNG() ([]byte, error) {
	data, err := p.page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	})
	if err != nil {
		return nil, fmt.Errorf("scout: screenshot png: %w", err)
	}

	return data, nil
}

// ScreenshotJPEG captures a viewport screenshot in JPEG format with the given quality (0-100).
func (p *Page) ScreenshotJPEG(quality int) ([]byte, error) {
	q := quality

	data, err := p.page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format:  proto.PageCaptureScreenshotFormatJpeg,
		Quality: gson.Int(q),
	})
	if err != nil {
		return nil, fmt.Errorf("scout: screenshot jpeg: %w", err)
	}

	return data, nil
}

// PDF generates a PDF of the current page with default settings.
func (p *Page) PDF() ([]byte, error) {
	reader, err := p.page.PDF(&proto.PagePrintToPDF{})
	if err != nil {
		return nil, fmt.Errorf("scout: generate pdf: %w", err)
	}

	buf, err := readAll(reader)
	if err != nil {
		return nil, fmt.Errorf("scout: read pdf: %w", err)
	}

	return buf, nil
}

// PDFWithOptions generates a PDF with custom options.
func (p *Page) PDFWithOptions(opts PDFOptions) ([]byte, error) {
	req := &proto.PagePrintToPDF{
		Landscape:           opts.Landscape,
		DisplayHeaderFooter: opts.DisplayHeaderFooter,
		PrintBackground:     opts.PrintBackground,
		PageRanges:          opts.PageRanges,
		HeaderTemplate:      opts.HeaderTemplate,
		FooterTemplate:      opts.FooterTemplate,
	}
	if opts.Scale > 0 {
		req.Scale = gson.Num(opts.Scale)
	}

	if opts.PaperWidth > 0 {
		req.PaperWidth = gson.Num(opts.PaperWidth)
	}

	if opts.PaperHeight > 0 {
		req.PaperHeight = gson.Num(opts.PaperHeight)
	}

	if opts.MarginTop > 0 {
		req.MarginTop = gson.Num(opts.MarginTop)
	}

	if opts.MarginBottom > 0 {
		req.MarginBottom = gson.Num(opts.MarginBottom)
	}

	if opts.MarginLeft > 0 {
		req.MarginLeft = gson.Num(opts.MarginLeft)
	}

	if opts.MarginRight > 0 {
		req.MarginRight = gson.Num(opts.MarginRight)
	}

	reader, err := p.page.PDF(req)
	if err != nil {
		return nil, fmt.Errorf("scout: generate pdf: %w", err)
	}

	buf, err := readAll(reader)
	if err != nil {
		return nil, fmt.Errorf("scout: read pdf: %w", err)
	}

	return buf, nil
}

// Eval evaluates JavaScript on the page and returns the result.
func (p *Page) Eval(js string, args ...any) (*EvalResult, error) {
	obj, err := p.page.Eval(js, args...)
	if err != nil {
		return nil, fmt.Errorf("scout: eval: %w", err)
	}

	return remoteObjectToEvalResult(obj), nil
}

// EvalOnNewDocument registers JavaScript to run on every new document before any page scripts.
// Returns a function to remove the script.
func (p *Page) EvalOnNewDocument(js string) (remove func() error, err error) {
	rm, err := p.page.EvalOnNewDocument(js)
	if err != nil {
		return nil, fmt.Errorf("scout: eval on new document: %w", err)
	}

	return rm, nil
}

// Element finds the first element matching the CSS selector.
func (p *Page) Element(selector string) (*Element, error) {
	el, err := p.page.Element(selector)
	if err != nil {
		return nil, fmt.Errorf("scout: element %q: %w", selector, err)
	}

	return &Element{element: el}, nil
}

// Elements finds all elements matching the CSS selector.
func (p *Page) Elements(selector string) ([]*Element, error) {
	els, err := p.page.Elements(selector)
	if err != nil {
		return nil, fmt.Errorf("scout: elements %q: %w", selector, err)
	}

	return wrapElements(els), nil
}

// ElementByXPath finds the first element matching the XPath expression.
func (p *Page) ElementByXPath(xpath string) (*Element, error) {
	el, err := p.page.ElementX(xpath)
	if err != nil {
		return nil, fmt.Errorf("scout: element xpath %q: %w", xpath, err)
	}

	return &Element{element: el}, nil
}

// ElementsByXPath finds all elements matching the XPath expression.
func (p *Page) ElementsByXPath(xpath string) ([]*Element, error) {
	els, err := p.page.ElementsX(xpath)
	if err != nil {
		return nil, fmt.Errorf("scout: elements xpath %q: %w", xpath, err)
	}

	return wrapElements(els), nil
}

// ElementByJS finds an element using a JavaScript expression.
func (p *Page) ElementByJS(js string, args ...any) (*Element, error) {
	opts := rod.Eval(js, args...)

	el, err := p.page.ElementByJS(opts)
	if err != nil {
		return nil, fmt.Errorf("scout: element by js: %w", err)
	}

	return &Element{element: el}, nil
}

// ElementByText finds the first element matching the CSS selector whose text matches the regex.
func (p *Page) ElementByText(selector, regex string) (*Element, error) {
	el, err := p.page.ElementR(selector, regex)
	if err != nil {
		return nil, fmt.Errorf("scout: element by text %q %q: %w", selector, regex, err)
	}

	return &Element{element: el}, nil
}

// ElementFromPoint finds the element at the given page coordinates.
func (p *Page) ElementFromPoint(x, y int) (*Element, error) {
	el, err := p.page.ElementFromPoint(x, y)
	if err != nil {
		return nil, fmt.Errorf("scout: element from point (%d,%d): %w", x, y, err)
	}

	return &Element{element: el}, nil
}

// Search finds the first element matching the query using Chrome DevTools search.
// The query can be a CSS selector, XPath expression, or text.
func (p *Page) Search(query string) (*Element, error) {
	sr, err := p.page.Search(query)
	if err != nil {
		return nil, fmt.Errorf("scout: search %q: %w", query, err)
	}
	defer sr.Release()

	return &Element{element: sr.First}, nil
}

// Has returns true if an element matching the CSS selector exists.
func (p *Page) Has(selector string) (bool, error) {
	has, _, err := p.page.Has(selector)
	if err != nil {
		return false, fmt.Errorf("scout: has %q: %w", selector, err)
	}

	return has, nil
}

// HasXPath returns true if an element matching the XPath exists.
func (p *Page) HasXPath(xpath string) (bool, error) {
	has, _, err := p.page.HasX(xpath)
	if err != nil {
		return false, fmt.Errorf("scout: has xpath %q: %w", xpath, err)
	}

	return has, nil
}

// WaitLoad waits for the page load event.
func (p *Page) WaitLoad() error {
	if err := p.page.WaitLoad(); err != nil {
		return fmt.Errorf("scout: wait load: %w", err)
	}

	return nil
}

// WaitStable waits for the page to be stable (loaded, idle, and DOM stable) for the given duration.
func (p *Page) WaitStable(d time.Duration) error {
	if err := p.page.WaitStable(d); err != nil {
		return fmt.Errorf("scout: wait stable: %w", err)
	}

	return nil
}

// WaitDOMStable waits for the DOM to stop changing. The diff threshold controls sensitivity.
func (p *Page) WaitDOMStable(d time.Duration, diff float64) error {
	if err := p.page.WaitDOMStable(d, diff); err != nil {
		return fmt.Errorf("scout: wait dom stable: %w", err)
	}

	return nil
}

// WaitIdle waits for the page to reach an idle state.
func (p *Page) WaitIdle(timeout time.Duration) error {
	if err := p.page.WaitIdle(timeout); err != nil {
		return fmt.Errorf("scout: wait idle: %w", err)
	}

	return nil
}

// WaitRequestIdle waits for all network requests to settle. The duration specifies how long
// requests must be idle. Returns a function that blocks until the condition is met.
func (p *Page) WaitRequestIdle(d time.Duration, includes, excludes []string) func() {
	return p.page.WaitRequestIdle(d, includes, excludes, nil)
}

// WaitSelector waits until an element matching the CSS selector appears.
func (p *Page) WaitSelector(selector string) (*Element, error) {
	el, err := p.page.Element(selector)
	if err != nil {
		return nil, fmt.Errorf("scout: wait selector %q: %w", selector, err)
	}

	return &Element{element: el}, nil
}

// WaitXPath waits until an element matching the XPath expression appears.
func (p *Page) WaitXPath(xpath string) (*Element, error) {
	el, err := p.page.ElementX(xpath)
	if err != nil {
		return nil, fmt.Errorf("scout: wait xpath %q: %w", xpath, err)
	}

	return &Element{element: el}, nil
}

// WaitNavigation returns a function that waits for the next navigation to complete.
// Call the returned function after triggering navigation.
func (p *Page) WaitNavigation() func() {
	return p.page.WaitNavigation(proto.PageLifecycleEventNameLoad)
}

// SetViewport sets the page viewport dimensions.
func (p *Page) SetViewport(width, height int) error {
	if err := p.page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  width,
		Height: height,
	}); err != nil {
		return fmt.Errorf("scout: set viewport: %w", err)
	}

	return nil
}

// SetWindow sets the browser window bounds.
func (p *Page) SetWindow(left, top, width, height int) error {
	if err := p.page.SetWindow(&proto.BrowserBounds{
		Left:   gson.Int(left),
		Top:    gson.Int(top),
		Width:  gson.Int(width),
		Height: gson.Int(height),
	}); err != nil {
		return fmt.Errorf("scout: set window: %w", err)
	}

	return nil
}

// Emulate configures the page to emulate a specific device.
func (p *Page) Emulate(device devices.Device) error {
	if err := p.page.Emulate(device); err != nil {
		return fmt.Errorf("scout: emulate device: %w", err)
	}

	return nil
}

// SetDocumentContent replaces the page's document HTML.
func (p *Page) SetDocumentContent(html string) error {
	if err := p.page.SetDocumentContent(html); err != nil {
		return fmt.Errorf("scout: set document content: %w", err)
	}

	return nil
}

// AddScriptTag injects a <script> tag. Provide either a URL or inline content.
func (p *Page) AddScriptTag(url, content string) error {
	if err := p.page.AddScriptTag(url, content); err != nil {
		return fmt.Errorf("scout: add script tag: %w", err)
	}

	return nil
}

// AddStyleTag injects a <style> tag. Provide either a URL or inline content.
func (p *Page) AddStyleTag(url, content string) error {
	if err := p.page.AddStyleTag(url, content); err != nil {
		return fmt.Errorf("scout: add style tag: %w", err)
	}

	return nil
}

// StopLoading stops all pending navigation and resource loading.
func (p *Page) StopLoading() error {
	if err := p.page.StopLoading(); err != nil {
		return fmt.Errorf("scout: stop loading: %w", err)
	}

	return nil
}

// Activate brings this page to the foreground.
func (p *Page) Activate() error {
	if _, err := p.page.Activate(); err != nil {
		return fmt.Errorf("scout: activate page: %w", err)
	}

	return nil
}

// HandleDialog returns functions to wait for and handle JavaScript dialogs (alert, confirm, prompt).
// Call wait() to block until a dialog appears, then call handle() to accept/dismiss it.
func (p *Page) HandleDialog() (wait func() *proto.PageJavascriptDialogOpening, handle func(*proto.PageHandleJavaScriptDialog) error) {
	return p.page.HandleDialog()
}

// Race creates an element race - the first selector to match wins.
// Returns the matched element and its index in the selectors list.
func (p *Page) Race(selectors ...string) (*Element, int, error) {
	if len(selectors) == 0 {
		return nil, -1, fmt.Errorf("scout: race requires at least one selector")
	}

	race := p.page.Race()
	for _, sel := range selectors {
		race = race.Element(sel)
	}

	el, err := race.Do()
	if err != nil {
		return nil, -1, fmt.Errorf("scout: race: %w", err)
	}

	return &Element{element: el}, -1, nil
}

// RodPage returns the underlying rod.Page for advanced use cases.
func (p *Page) RodPage() *rod.Page {
	return p.page
}

// remoteObjectToEvalResult converts a rod RemoteObject to our EvalResult.
func remoteObjectToEvalResult(obj *proto.RuntimeRemoteObject) *EvalResult {
	if obj == nil {
		return &EvalResult{Type: "undefined"}
	}

	raw, _ := obj.Value.MarshalJSON()

	return &EvalResult{
		Type:    string(obj.Type),
		Subtype: string(obj.Subtype),
		Value:   obj.Value.Val(),
		rawJSON: raw,
	}
}

func wrapElements(els rod.Elements) []*Element {
	result := make([]*Element, len(els))
	for i, el := range els {
		result[i] = &Element{element: el}
	}

	return result
}

func readAll(r *rod.StreamReader) ([]byte, error) {
	var buf []byte

	for {
		chunk := make([]byte, 64*1024)

		n, err := r.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return buf, err
		}
	}

	return buf, nil
}
